#!/usr/bin/env python3
"""Cache / matrix scenarios against the docker compose stack (distance_matrix_app).

Requires:
  docker compose -f docker-compose.dev.yml up -d

Usage:
  python3 scripts/scenario_cache_matrix.py
  python3 scripts/scenario_cache_matrix.py --only 8,10
"""

from __future__ import annotations

import argparse
import json
import math
import os
import random
import re
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path

BASE = os.environ.get("MATRIX_BASE_URL", "http://127.0.0.1:8888")
ROOT = Path(__file__).resolve().parents[1]
APP = "distance_matrix_app"
DOCKER_CFG = ROOT / "etc" / "matrix.docker.yaml"
COMPOSE = ["docker", "compose", "-f", str(ROOT / "docker-compose.dev.yml")]
MYSQL = [
    "docker",
    "exec",
    "distance_matrix_mysql",
    "mysql",
    "-umatrix",
    "-pmatrix",
    "-N",
    "-e",
]


def http_raw(method: str, path: str, body: dict | None, tenant: str, timeout: float = 180) -> tuple[int, bytes]:
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


def http_json(method: str, path: str, body: dict | None, tenant: str, timeout: float = 180) -> tuple[int, dict]:
    code, raw = http_raw(method, path, body, tenant, timeout=timeout)
    try:
        return code, json.loads(raw.decode())
    except Exception:
        return code, {"raw": raw.decode(errors="replace")}


def wait_ready(timeout_s: float = 90) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            code, _ = http_raw("GET", "/health/ready", None, "probe", timeout=2)
            if code == 200:
                return
        except Exception:
            pass
        time.sleep(0.3)
    raise SystemExit(f"matrix not ready at {BASE} (is {APP} up?)")


def app_running() -> bool:
    r = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Running}}", APP],
        capture_output=True,
        text=True,
    )
    return r.returncode == 0 and r.stdout.strip() == "true"


def read_app_logs(tail: int = 4000) -> str:
    r = subprocess.run(
        ["docker", "logs", "--tail", str(tail), APP],
        capture_output=True,
        text=True,
    )
    return (r.stdout or "") + (r.stderr or "")


def beijing_points(n: int, seed: int) -> list[list[float]]:
    rng = random.Random(seed)
    return [
        [round(116.35 + rng.random() * 0.08, 6), round(39.88 + rng.random() * 0.06, 6)]
        for _ in range(n)
    ]


def offset_m(pt: list[float], east_m: float, north_m: float) -> list[float]:
    lon, lat = pt
    dlat = north_m / 111_320.0
    dlon = east_m / (111_320.0 * max(0.2, math.cos(math.radians(lat))))
    return [round(lon + dlon, 6), round(lat + dlat, 6)]


def parse_last_matrix_log(tenant: str) -> dict:
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


def mysql_count(tenant: str | None = None) -> int:
    where = f" WHERE tenant='{tenant}'" if tenant else ""
    sql = f"SELECT COUNT(*) FROM distance_matrix.distance_matrix_edge{where};"
    try:
        out = subprocess.check_output(MYSQL + [sql], stderr=subprocess.DEVNULL, text=True).strip()
        return int(out.splitlines()[-1])
    except Exception as e:
        print(f"  mysql count failed: {e}")
        return -1


def redis_flush_tenant(tenant: str) -> None:
    prefix = f"distance_matrix:{tenant}"
    script = f"""
    local n=0
    local cursor='0'
    repeat
      local r=redis.call('SCAN', cursor, 'MATCH', '{prefix}*', 'COUNT', 200)
      cursor=r[1]
      for _,k in ipairs(r[2]) do redis.call('DEL', k); n=n+1 end
    until cursor=='0'
    return n
    """
    subprocess.run(
        ["docker", "exec", "distance_matrix_redis", "redis-cli", "EVAL", script, "0"],
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def matrix(
    points: list[list[float]],
    tenant: str,
    *,
    strict: bool = False,
    geo_wide_m: int = 200,
    method: int = 0,
    coordinate: str = "gcj02",
    timeout: float = 180,
) -> tuple[int, dict, dict]:
    body = {
        "points": points,
        "coordinate": coordinate,
        "strict": strict,
        "geo_wide_m": geo_wide_m,
        "method": method,
    }
    t0 = time.time()
    code, payload = http_json("POST", "/v1/matrix", body, tenant, timeout=timeout)
    wall_ms = int((time.time() - t0) * 1000)
    time.sleep(0.25)
    stats = parse_last_matrix_log(tenant)
    stats["wall_ms"] = wall_ms
    stats["http"] = code
    return code, payload, stats


def print_stats(label: str, stats: dict) -> None:
    print(
        f"  {label}: http={stats.get('http')} hit={stats.get('cache_hit')} "
        f"hits={stats.get('hits')} misses={stats.get('misses')} "
        f"fallback={stats.get('fallback')} provider_calls={stats.get('provider_calls')} "
        f"elapsed_ms={stats.get('elapsed_ms')} wall_ms={stats.get('wall_ms')}"
    )


def _write_docker_cfg_and_restart(yaml_text: str) -> None:
    DOCKER_CFG.write_text(yaml_text, encoding="utf-8")
    subprocess.run(COMPOSE + ["up", "-d", "--force-recreate", "matrix"], check=True, cwd=str(ROOT))
    wait_ready(120)


def scenario_1_to_6_quick() -> None:
    print("\n=== Quick recheck scenarios 1–6 (container) ===")
    tenant = f"sc16-{int(time.time()) % 100000}"
    redis_flush_tenant(tenant)
    pts = beijing_points(4, seed=16)
    _, _, s1 = matrix(pts, tenant, strict=False)
    print_stats("1 cold", s1)
    _, _, s2 = matrix(pts, tenant, strict=False)
    print_stats("2 warm", s2)
    assert float(s2.get("cache_hit") or 0) > 0.9

    near = [offset_m(p, 80, 40) for p in pts]
    _, _, s3 = matrix(near, tenant, strict=False, geo_wide_m=200)
    print_stats("3 fuzzy ~100m", s3)
    _, _, s4 = matrix(near, tenant, strict=True, geo_wide_m=200)
    print_stats("4 strict near", s4)

    other = tenant + "-b"
    redis_flush_tenant(other)
    _, _, s5 = matrix(pts, other, strict=False)
    print_stats("5 other tenant", s5)
    assert int(s5.get("hits") or 0) == 0

    _, _, s6 = matrix(pts, tenant, strict=False, method=1)
    print_stats("6 method=1", s6)
    assert int(s6.get("hits") or 0) == 0
    print("  PASS")


def scenario_8() -> None:
    print("\n=== Scenario 8: partial overlap (container) ===")
    tenant = f"sc8-{int(time.time()) % 100000}"
    redis_flush_tenant(tenant)
    base = beijing_points(12, seed=8)
    code, _, s1 = matrix(base, tenant, strict=True, timeout=300)
    print_stats("cold 12", s1)
    assert code == 200, s1
    assert int(s1.get("hits") or 0) == 0
    cold_miss = int(s1.get("misses") or 0)
    cold_fb = int(s1.get("fallback") or 0)
    persisted = max(0, cold_miss - cold_fb)
    print(f"  cold persisted≈{persisted} (misses={cold_miss} fallback={cold_fb})")

    keep_n = 9
    mixed = base[:keep_n] + beijing_points(3, seed=801)
    code, _, s2 = matrix(mixed, tenant, strict=True, timeout=300)
    print_stats(f"warm {keep_n}+3", s2)
    assert code == 200
    hits = int(s2.get("hits") or 0)
    misses = int(s2.get("misses") or 0)
    n0 = len(base)
    retained_share = (keep_n * (keep_n - 1)) / max(1, n0 * (n0 - 1))
    expect = int(persisted * retained_share * 0.5)
    print(f"  retained_share={retained_share:.2f}; expect hits>~{expect}; got hits={hits} misses={misses}")
    assert hits >= max(8, expect), f"expected reuse hits>={expect}, got {hits}"
    assert misses > 0
    assert float(s2.get("cache_hit") or 0) > 0.05
    print("  PASS")


def scenario_10() -> None:
    print("\n=== Scenario 10: fallback does NOT write back (container) ===")
    original = DOCKER_CFG.read_text(encoding="utf-8")
    broken = original.replace(
        "BaseURL: http://restapi.amap.com",
        "BaseURL: http://127.0.0.1:9",
    )
    tenant = f"sc10-{int(time.time()) % 100000}"
    pts = beijing_points(3, seed=10)
    redis_flush_tenant(tenant)

    try:
        print("  recreate app with unreachable Amap BaseURL…")
        _write_docker_cfg_and_restart(broken)
        before = mysql_count(tenant)
        code, _, s1 = matrix(pts, tenant, strict=True, timeout=60)
        print_stats("cold fallback", s1)
        time.sleep(1.2)
        after_cold = mysql_count(tenant)
        print(f"  mysql rows: before={before} after_cold={after_cold}")
        assert code == 200, s1
        assert int(s1.get("fallback") or 0) > 0, "expected haversine fallback"
        assert after_cold == before, "fallback must not upsert L2"

        code, _, s2 = matrix(pts, tenant, strict=True, timeout=60)
        print_stats("warm after fallback", s2)
        time.sleep(0.8)
        after_warm = mysql_count(tenant)
        print(f"  mysql rows after warm={after_warm}")
        assert after_warm == after_cold, "warm after fallback must not grow L2"
        assert float(s2.get("cache_hit") or 0) < 0.5 or int(s2.get("fallback") or 0) > 0
        print("  PASS")
    finally:
        print("  restore matrix.docker.yaml + recreate app…")
        _write_docker_cfg_and_restart(original)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", default="1-6,8,10")
    args = ap.parse_args()
    want = {x.strip() for x in args.only.split(",") if x.strip()}

    if not app_running():
        raise SystemExit(f"{APP} is not running — start with: docker compose -f docker-compose.dev.yml up -d")

    wait_ready()
    print(f"target={BASE} app={APP} mysql_rows_total={mysql_count()}")
    print(subprocess.check_output(
        ["docker", "ps", "--filter", "name=distance_matrix", "--format", "{{.Names}} {{.Status}} {{.Ports}}"],
        text=True,
    ).strip())

    if "1-6" in want:
        scenario_1_to_6_quick()
    if "8" in want:
        scenario_8()
    if "10" in want:
        scenario_10()
    print("\nAll requested scenarios done (container).")


if __name__ == "__main__":
    main()
