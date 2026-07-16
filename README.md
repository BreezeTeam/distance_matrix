# distance_matrix

Enterprise OD distance/time matrix service — **cache-first**, **multi-tenant**, **Amap-backed**, built for VRP and dispatch at scale.

> Approximation-aware: falls back to haversine when provider fails. Monitor metrics, not response flags.

## Documentation

| Doc | Content |
|-----|---------|
| [**Docs index**](./docs/README.md) | Full documentation hub |
| [API Reference](./docs/api-reference.md) | Endpoints, errors, headers |
| [OpenAPI](./docs/openapi.yaml) | Swagger / codegen |
| [Architecture](./docs/architecture.md) | Cache model, data flow |
| [Configuration](./docs/configuration.md) | `matrix.yaml` |
| [Operations](./docs/operations.md) | Deploy, metrics, capacity |
| [Testing](./docs/testing.md) | Unit, e2e, live |
| [User Stories](./docs/user-stories.md) | E2E acceptance criteria |
| [Key pool scheduler](./docs/key-pool-algorithm.md) | Multi-key routing formula and tuning |
| [Design](./docs/design/README.md) | v2 spec and completion plan |

## Quick start

```bash
docker compose up -d redis
go run matrix.go -f etc/matrix.yaml
curl -s http://localhost:8888/health/ready
```

```bash
curl -X POST http://localhost:8888/v1/matrix \
  -H "Content-Type: application/json" \
  -H "X-Tenant-Id: default" \
  -d '{"points":[[116.40,39.90],[116.41,39.91]],"coordinate":"gcj02"}'
```

## API summary

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/matrix` | N×N distance/duration matrices |
| POST | `/v1/route` | Multi-waypoint route + steps |
| GET | `/v1/providers` | Enabled providers |
| GET | `/health/live` | Liveness |
| GET | `/health/ready` | Readiness |

Manual requests: [`test/api.http`](./test/api.http)

## Project layout

```
matrix.go                 # entrypoint
etc/matrix.yaml           # runtime config
api/service.api           # goctl HTTP contract
docs/                     # documentation hub
  design/                 # v2 spec, completion plan
  key-pool-algorithm.md   # multi-key scheduler
internal/
  handler/  engine/  cache/  planner/  provider/
  loadbalance/  geo/  config/  middleware/  metrics/
test/
  api.http                # REST Client scenarios
  e2e/                    # HTTP + user story tests
scripts/
  verify_amap_keys.sh     # key health
  simulate_key_pool.go    # key pool simulation
  load_matrix.sh          # load smoke
```

## Testing

```bash
go test ./...                              # unit + integration
go test ./test/e2e/ -v                     # HTTP e2e + user stories
go test ./test/e2e/ -tags=e2e -run TestLive -v   # live Amap (opt-in)
bash scripts/verify_amap_keys.sh
```

**18 e2e tests** including 10 user-story acceptance tests (`TestStory01`–`TestStory10`).

## Architecture

```
HTTP → MatrixEngine → EdgeCache (Redis)
                    → RoutePlanner → AmapProvider (multi-key pool)
```

## Deploy

```bash
docker compose up --build
```

See [Operations](./docs/operations.md) for health checks, Prometheus metrics, and capacity planning.

## License

See [LICENSE](./LICENSE).
