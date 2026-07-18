# Configuration

Runtime: [`etc/matrix.yaml`](../etc/matrix.yaml) · structs: [`internal/config/config.go`](../internal/config/config.go)

## Server (go-zero RestConf)

| Key | Default | Description |
|-----|---------|-------------|
| `Name` | — | Service name |
| `Host` | `0.0.0.0` | Bind address |
| `Port` | `8888` | HTTP port |
| `Timeout` | `30000` | Business matrix deadline (ms) → **504** `MATRIX_DEADLINE`. Rest framework timeout is off (`config.ForServer`) to avoid **503** `Request Timeout`. |
| `Log.*` | — | go-zero logging |

## Redis

| Key | Default | Description |
|-----|---------|-------------|
| `Enabled` | `true` | Edge cache |
| `Addr` | `127.0.0.1:6379` | Redis address |
| `Prefix` | `distance_matrix` | Key prefix (tenant prepended) |
| `EdgeTTL` | `1209600` | HASH TTL (14 days) |

Enabled but unreachable → no cache; `/health/ready` → 503.

## Persistence (optional MySQL L2)

Non-empty `DSN` enables cold archive. Empty = Redis-only.

| Key | Default | Description |
|-----|---------|-------------|
| `DSN` | _(empty)_ | `user:pass@tcp(host:3306)/db?parseTime=true&charset=utf8mb4` |
| `Database` | — | DB name (must exist unless `AutoMigrate`) |
| `AutoMigrate` | `false` | App `CREATE` — off by default; prefer offline DDL |
| `MaxOpenConns` | `10` | Pool |
| `MaxIdleConns` | `5` | Idle |
| `AsyncQueue` | `1024` | Async upsert queue (drop when full) |

DDL / schema:

1. Edit GORM model `internal/persist/model.go`
2. Apply with privileged user: `Persistence.AutoMigrate: true` (GORM `AutoMigrate`), or
3. Dump Migrator SQL for offline apply:
   `go run ./scripts/genddl -dsn 'user:pass@tcp(host:3306)/distance_matrix?parseTime=true&charset=utf8mb4'`

Do not hand-write CREATE TABLE. Compose can still mount generated `scripts/ddl/` into MySQL initdb after genddl.

## Engine

| Key | Default | Description |
|-----|---------|-------------|
| `DefaultGeoWideM` | `200` | Fuzzy GEO radius (m) |
| `MaxPoints` | `100` | Max matrix size |
| `TenantQPS` | `50` | Per-tenant `/v1/matrix` limit |

## Providers.amap

| Key | Default | Description |
|-----|---------|-------------|
| `Enabled` | `true` | Register provider |
| `Keys` | — | Comma-separated API keys |
| `BaseURL` | `http://restapi.amap.com` | API base |
| `BatchSize` | `12` | Max waypoints / request; Dense planner uses `L = BatchSize - 1` legs |
| `TimeoutSec` | `2` | HTTP timeout |
| `KeyRecoverySec` | `300` | Outcome decay half-life |
| `KeyProbeSec` | `30` | Dead-key probe base T₀ |
| `KeyConfidenceTau` | `2.0` | Confidence ramp τ |
| `KeyBetaPriorA` / `B` | `2` / `1` | Beta prior |
| `KeyFailureSoftWeight` | `0.3` | Soft-failure weight w_f |
| `KeyEpsilonScale` | `4` | Lone-key ε scale |
| `KeyMinProbeRate` | `0.02` | Probe floor ρ_min |

Drop permanently dead keys from `Keys`. Tuning: [key pool algorithm](./key-pool-algorithm.md).

## Example

```yaml
Redis:
  Enabled: true
  Addr: 127.0.0.1:6379
  Prefix: distance_matrix
  EdgeTTL: 1209600

Persistence:
  DSN: "matrix:matrix@tcp(127.0.0.1:3306)/distance_matrix?parseTime=true&charset=utf8mb4"
  AutoMigrate: false

Engine:
  MaxPoints: 100
  TenantQPS: 50

Providers:
  amap:
    Enabled: true
    Keys: key1,key2,key3
    BatchSize: 12
```

```bash
go run matrix.go -f etc/matrix.yaml
```

Compose: [`docker-compose.yml`](../docker-compose.yml) (`redis` + optional `mysql` + `matrix`).
