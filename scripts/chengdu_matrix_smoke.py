#!/usr/bin/env python3
"""Chengdu matrix smoke: 50-point live, 500-point dry-run (Dense plan only)."""

from __future__ import annotations

import json
import math
import random
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

BASE = "http://127.0.0.1:8888"
TENANT = "chengdu-smoke"

# Chengdu core bbox (approx GCJ-02)
LON0, LON1 = 104.00, 104.16
LAT0, LAT1 = 30.57, 30.72


def chengdu_points(n: int, seed: int = 42) -> list[list[float]]:
    rng = random.Random(seed)
    pts = []
    for _ in range(n):
        lon = LON0 + rng.random() * (LON1 - LON0)
        lat = LAT0 + rng.random() * (LAT1 - LAT0)
        pts.append([round(lon, 6), round(lat, 6)])
    return pts


def http_raw(method: str, path: str, body: dict | None = None, timeout: float = 600) -> tuple[int, bytes]:
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={
            "Content-Type": "application/json",
            "X-Tenant-Id": TENANT,
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def http_json(method: str, path: str, body: dict | None = None, timeout: float = 600) -> tuple[int, dict]:
    code, raw = http_raw(method, path, body, timeout)
    try:
        return code, json.loads(raw.decode())
    except Exception:
        return code, {"raw": raw.decode(errors="replace")}


def wait_ready(timeout_s: float = 60) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            code, _ = http_raw("GET", "/health/ready", timeout=2)
            if code == 200:
                return
        except Exception:
            pass
        time.sleep(0.5)
    raise SystemExit("matrix service not ready")


def summarize_matrix(payload: dict, n: int) -> None:
    data = payload.get("data") or {}
    dist = data.get("distances") or []
    dur = data.get("durations") or []
    assert len(dist) == n and len(dur) == n
    vals = [dist[i][j] for i in range(n) for j in range(n) if i != j]
    print(f"  matrix {n}x{n} ok; sample d[0][1]={dist[0][1]:.1f}m t[0][1]={dur[0][1]:.1f}s")
    print(f"  off-diag: min={min(vals):.1f} max={max(vals):.1f} mean={sum(vals)/len(vals):.1f}")


def dry_run_500() -> None:
    """Build request JSON + Dense plan stats without calling Amap."""
    # Invoke a tiny Go helper via `go run` would be heavier; print request and
    # estimated full-miss Dense LB instead, and write request body to disk.
    pts = chengdu_points(500, seed=7)
    body = {
        "points": pts,
        "coordinate": "gcj02",
        "strict": True,
        "geo_wide_m": 200,
    }
    out = Path("/tmp/chengdu_500_request.json")
    out.write_text(json.dumps(body), encoding="utf-8")
    m = 500 * 499
    L = 11  # BatchSize 12 → maxLegs 11
    lb0 = math.ceil(m / L)
    print("=== 500-point dry-run (no provider call) ===")
    print(f"  request written: {out} ({out.stat().st_size} bytes)")
    print(f"  n=500 m={m} maxLegs={L} LB0=ceil(m/L)={lb0}")
    print(f"  first 3 points: {pts[:3]}")
    print("  note: full live 500×500 would be ~249k OD edges; skipped per request.")


def main() -> None:
    wait_ready()
    print("=== health ===")
    print("  ready OK")

    pts50 = chengdu_points(50, seed=42)
    body50 = {
        "points": pts50,
        "coordinate": "gcj02",
        "strict": True,
        "geo_wide_m": 200,
    }

    print("\n=== 50-point live #1 (cold cache) ===")
    t0 = time.time()
    code, payload = http_json("POST", "/v1/matrix", body50, timeout=600)
    ms = (time.time() - t0) * 1000
    print(f"  http={code} elapsed={ms:.0f}ms msg={payload.get('msg')}")
    if code != 200:
        print(json.dumps(payload, ensure_ascii=False)[:2000])
        sys.exit(1)
    summarize_matrix(payload, 50)
    Path("/tmp/chengdu_50_result.json").write_text(json.dumps(payload), encoding="utf-8")
    print("  saved /tmp/chengdu_50_result.json")

    print("\n=== 50-point live #2 (expect cache hits) ===")
    t0 = time.time()
    code2, payload2 = http_json("POST", "/v1/matrix", body50, timeout=600)
    ms2 = (time.time() - t0) * 1000
    print(f"  http={code2} elapsed={ms2:.0f}ms msg={payload2.get('msg')}")
    if code2 != 200:
        print(json.dumps(payload2, ensure_ascii=False)[:2000])
        sys.exit(1)
    summarize_matrix(payload2, 50)
    print("  check service logs for cache_hit / hits / misses / provider_calls")

    dry_run_500()
    print("\nDONE")


if __name__ == "__main__":
    main()
