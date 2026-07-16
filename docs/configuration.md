# Configuration

Runtime config: [`etc/matrix.yaml`](../etc/matrix.yaml)

Go structs: [`internal/config/config.go`](../internal/config/config.go)

## Server (go-zero RestConf)

| Key | Default | Description |
|-----|---------|-------------|
| `Name` | — | Service name |
| `Host` | `0.0.0.0` | Bind address |
| `Port` | `8888` | HTTP port |
| `Timeout` | `30000` | Request deadline (ms) → matrix 504 |
| `Log.*` | — | go-zero logging |

## Redis

| Key | Default | Description |
|-----|---------|-------------|
| `Enabled` | `true` | Use edge cache |
| `Addr` | `127.0.0.1:6379` | Redis address |
| `Prefix` | `distance_matrix` | Global key prefix (tenant prepended) |
| `EdgeTTL` | `1209600` | Edge HASH TTL (seconds, 14 days) |

If Redis is enabled but unreachable at startup, service runs **without cache** and `/health/ready` returns 503.

## Engine

| Key | Default | Description |
|-----|---------|-------------|
| `DefaultGeoWideM` | `200` | Default fuzzy cache radius (meters) |
| `MaxPoints` | `100` | Max matrix size |
| `FallbackFactor` | `1.5` | Haversine distance multiplier (code default) |
| `TenantQPS` | `50` | Per-tenant rate limit on `/v1/matrix` |

## Providers.amap

| Key | Default | Description |
|-----|---------|-------------|
| `Enabled` | `true` | Register Amap provider |
| `Keys` | — | Comma-separated API keys |
| `BaseURL` | `http://restapi.amap.com` | API base |
| `BatchSize` | `12` | Max waypoints per driving request |
| `TimeoutSec` | `2` | Per HTTP call timeout |
| `KeyRecoverySec` | `300` | Outcome decay half-life (seconds) |
| `KeyProbeSec` | `30` | Dead-key probe interval base T₀ (seconds) |
| `KeyConfidenceTau` | `2.0` | Confidence ramp τ in α = 1 − e^(−η/τ) |
| `KeyBetaPriorA` / `KeyBetaPriorB` | `2` / `1` | Beta prior → ρ = (S+a)/(S+F+a+b) |
| `KeyFailureSoftWeight` | `0.3` | w_f in η = S + w_f·F |
| `KeyEpsilonScale` | `4` | Lone-key ε = e^(−N/k) anti-starvation |
| `KeyMinProbeRate` | `0.02` | Probe floor ρ_min |

### Key pool tuning

- Remove permanently invalid keys from `Keys` (IP restricted, recycled) — saves probe traffic
- `KeyRecoverySec`: how fast failure history fades for recovered keys
- `KeyProbeSec`: minimum interval before a dead key gets another probe chance

See [Key pool scheduler](./key-pool-algorithm.md).

## Example

```yaml
Redis:
  Enabled: true
  Addr: 127.0.0.1:6379
  Prefix: distance_matrix
  EdgeTTL: 1209600

Engine:
  DefaultGeoWideM: 200
  MaxPoints: 100
  TenantQPS: 50

Providers:
  amap:
    Enabled: true
    Keys: key1,key2,key3
    BatchSize: 12
    TimeoutSec: 2
    KeyRecoverySec: 300
    KeyProbeSec: 30
    KeyConfidenceTau: 2.0
    KeyBetaPriorA: 2
    KeyBetaPriorB: 1
    KeyFailureSoftWeight: 0.3
    KeyEpsilonScale: 4
    KeyMinProbeRate: 0.02
```

## Environment overrides

go-zero supports `-f` flag:

```bash
go run matrix.go -f etc/matrix.yaml
```

Docker Compose mounts config and sets Redis host — see [`docker-compose.yml`](../docker-compose.yml).
