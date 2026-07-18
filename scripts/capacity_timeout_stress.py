#!/usr/bin/env python3
"""Capacity / timeout stress against docker compose stack (distance_matrix_app).

Covers:
  A) live n=50 / 100 / 200 (cold + warm)
  B) concurrent same-tenant cold storm
  C) timeout degrade (go-zero 503 vs MATRIX_DEADLINE 504) + restore

Requires:
  docker compose -f docker-compose.dev.yml up -d

Usage:
  python3 scripts/capacity_timeout_stress.py
  python3 scripts/capacity_timeout_stress.py --skip-200
  python3 scripts/capacity_timeout_stress.py --only A,B
"""

from __future__ import annotations

import argparse
import json
import os
import random
import re
import subprocess
import threading
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

BASE = os.environ.get("MATRIX_BASE_URL", "http://127.0.0.1:8888")
ROOT = Path(__file__).resolve().parents[1]
APP = "distance_matrix_app"
DOCKER_CFG = ROOT / "etc" / "matrix.docker.yaml"
COMPOSE = ["docker", "compose", "-f", str(ROOT / "docker-compose.dev.yml")]


def http_raw(method: str, path: str, body: dict | None, tenant: str, timeout: float) -> tuple[int, bytes]:
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={"Content-Type": "application/json", "X-Tenant-Id": tenant},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return 0, str(e).encode()


def wait_ready(timeout_s: float = 90) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        code, _ = http_raw("GET", "/health/ready", None, "probe", 2)
        if code == 200:
            return
        time.sleep(0.25)
    raise SystemExit(f"matrix not ready at {BASE}")


def app_running() -> bool:
    r = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Running}}", APP],
        capture_output=True,
        text=True,
    )
    return r.returncode == 0 and r.stdout.strip() == "true"


def read_app_logs(tail: int = 8000) -> str:
    r = subprocess.run(
        ["docker", "logs", "--tail", str(tail), APP],
        capture_output=True,
        text=True,
    )
    return (r.stdout or "") + (r.stderr or "")


def beijing_points(n: int, seed: int) -> list[list[float]]:
    rng = random.Random(seed)
    return [
        [round(116.30 + rng.random() * 0.12, 6), round(39.85 + rng.random() * 0.10, 6)]
        for _ in range(n)
    ]


def parse_last(tenant: str) -> dict:
    pat = re.compile(
        rf"matrix tenant={re.escape(tenant)} n=(\d+) cache_hit=([0-9.]+) hits=(\d+) misses=(\d+) "
        rf"fallback=(\d+) provider_calls=(\d+) arccover_calls=(\d+) elapsed_ms=(\d+)"
    )
    for line in reversed(read_app_logs().splitlines()):
        m = pat.search(line)
        if m:
            return {
                "n": int(m.group(1)),
                "cache_hit": float(m.group(2)),
                "hits": int(m.group(3)),
                "misses": int(m.group(4)),
                "fallback": int(m.group(5)),
                "provider_calls": int(m.group(6)),
                "arccover_calls": int(m.group(7)),
                "elapsed_ms": int(m.group(8)),
            }
    return {}


def matrix(points: list[list[float]], tenant: str, *, strict: bool = True, timeout: float = 600) -> tuple[int, dict, dict, int]:
    body = {"points": points, "coordinate": "gcj02", "strict": strict, "geo_wide_m": 200}
    t0 = time.time()
    code, raw = http_raw("POST", "/v1/matrix", body, tenant, timeout)
    wall = int((time.time() - t0) * 1000)
    try:
        payload = json.loads(raw.decode())
    except Exception:
        payload = {"raw": raw.decode(errors="replace")[:500]}
    time.sleep(0.25)
    stats = parse_last(tenant)
    stats["http"] = code
    stats["wall_ms"] = wall
    return code, payload, stats, wall


def print_stats(label: str, stats: dict) -> None:
    print(
        f"  {label}: http={stats.get('http')} hit={stats.get('cache_hit')} "
        f"hits={stats.get('hits')} misses={stats.get('misses')} "
        f"fallback={stats.get('fallback')} provider_calls={stats.get('provider_calls')} "
        f"arccover={stats.get('arccover_calls')} elapsed_ms={stats.get('elapsed_ms')} "
        f"wall_ms={stats.get('wall_ms')}"
    )


def recreate_app_with_cfg(yaml_text: str) -> None:
    DOCKER_CFG.write_text(yaml_text, encoding="utf-8")
    subprocess.run(COMPOSE + ["up", "-d", "--force-recreate", "matrix"], check=True, cwd=str(ROOT))
    wait_ready(120)


def section_live_sizes(skip_200: bool) -> None:
    print("\n=== A) Live capacity n=50 / 100 / 200 (container) ===")
    sizes = [50, 100] + ([] if skip_200 else [200])
    for n in sizes:
        tenant = f"cap-n{n}-{int(time.time()) % 100000}"
        pts = beijing_points(n, seed=n * 17)
        client_to = 320 if n < 200 else 920
        print(f"\n-- n={n} tenant={tenant} --")
        code, payload, s1, _ = matrix(pts, tenant, timeout=client_to)
        print_stats("cold", s1)
        if code not in (200, 503, 504):
            print(f"  FAIL unexpected http={code} body={str(payload)[:300]}")
            continue
        if code in (503, 504) or payload.get("code") == 504:
            print(f"  note: cold timed out http={code} body={payload}")
            code2, _, s2, _ = matrix(pts, tenant, timeout=client_to)
            print_stats("retry after timeout", s2)
            continue
        fb = int(s1.get("fallback") or 0)
        miss = int(s1.get("misses") or 1)
        print(f"  fallback_ratio={fb / miss:.2f}")
        code2, _, s2, _ = matrix(pts, tenant, timeout=client_to)
        print_stats("warm", s2)


def section_concurrent() -> None:
    print("\n=== B) Concurrent same-tenant cold storm (container) ===")
    tenant = f"cap-conc-{int(time.time()) % 100000}"
    pts = beijing_points(8, seed=99)
    workers = 8
    barrier = threading.Barrier(workers)
    results: list[tuple[int, int, dict]] = []

    def one(i: int) -> None:
        barrier.wait(timeout=30)
        code, _, stats, wall = matrix(pts, tenant, timeout=120)
        results.append((i, wall, stats))

    t0 = time.time()
    with ThreadPoolExecutor(max_workers=workers) as ex:
        futs = [ex.submit(one, i) for i in range(workers)]
        for f in as_completed(futs):
            f.result()
    wall = int((time.time() - t0) * 1000)
    results.sort(key=lambda x: x[0])
    provider_calls = [r[2].get("provider_calls") for r in results]
    fallbacks = [r[2].get("fallback") for r in results]
    https = [r[2].get("http") for r in results]
    print(f"  workers={workers} total_wall_ms={wall}")
    for i, w, s in results:
        print_stats(f"w{i}", s)
    ok = sum(1 for h in https if h == 200)
    print(f"  ok={ok}/{workers} provider_calls={provider_calls} fallbacks={fallbacks}")
    total_pc = sum(int(x or 0) for x in provider_calls)
    print(f"  sum_provider_calls={total_pc} (storm amplifies duplicate provider work)")
    assert ok >= workers // 2, "too many concurrent failures"


def section_504() -> None:
    print("\n=== C) Timeout degrade + restore (container) ===")
    original = DOCKER_CFG.read_text(encoding="utf-8")
    tight = original.replace("Timeout: 300000", "Timeout: 1500")
    tenant = f"cap-504-{int(time.time()) % 100000}"
    pts = beijing_points(40, seed=504)
    try:
        print("  recreate app with Timeout=1500ms…")
        recreate_app_with_cfg(tight)
        code, payload, s1, _ = matrix(pts, tenant, timeout=30)
        print_stats("tight cold", s1)
        print(f"  body={payload}")
        # After config.ForServer: business Timeout → 504 MATRIX_DEADLINE (not go-zero 503).
        assert code == 504 or payload.get("code") == 504, (
            f"expected 504 MATRIX_DEADLINE, got http={code} payload={payload}"
        )
        print(f"  observed timeout surface: http={code} body={payload}")

        code2, payload2, s2, _ = matrix(pts, tenant, timeout=30)
        print_stats("tight retry", s2)
        print(f"  retry body={payload2}")
    finally:
        print("  restore Timeout=300000…")
        recreate_app_with_cfg(original)

    code3, _, s3, _ = matrix(pts, tenant, timeout=320)
    print_stats("after restore", s3)
    assert code3 == 200, f"expected 200 after restore, got {code3}"
    print("  PASS")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--skip-200", action="store_true")
    ap.add_argument("--only", default="A,B,C", help="A=sizes B=concurrent C=timeout")
    args = ap.parse_args()
    want = {x.strip().upper() for x in args.only.split(",") if x.strip()}

    if not app_running():
        raise SystemExit(f"{APP} is not running — start with: docker compose -f docker-compose.dev.yml up -d")

    wait_ready()
    print(f"target={BASE} app={APP}")
    print(subprocess.check_output(
        ["docker", "ps", "--filter", "name=distance_matrix", "--format", "{{.Names}} {{.Status}} {{.Ports}}"],
        text=True,
    ).strip())

    if "A" in want:
        section_live_sizes(skip_200=args.skip_200)
    if "B" in want:
        section_concurrent()
    if "C" in want:
        section_504()
    print("\nDONE (container)")


if __name__ == "__main__":
    main()
