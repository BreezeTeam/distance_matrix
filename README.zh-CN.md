# distance_matrix

[English](./README.md) | [中文](./README.zh-CN.md)

面向 VRP / 调度 / 路径优化的企业级 **OD 距离时间矩阵**服务：缓存优先、多租户隔离、高德路网，同步 HTTP 一次返回 N×N。

同类方案常见痛点是「每条边打一次 Provider、配额烧得快、超时重试从头算」。本服务默认走边缓存、Dense 打包规划与写穿续跑，让矩阵请求更省配额、更可重试。

## 亮点

| 亮点 | 你能得到什么 |
|------|----------------|
| **缓存边，不是整表** | Redis 存 directed OD 边，在租户命名空间内跨请求复用 |
| **Dense Arc Cover** | miss 边规划成连续 walk（≤ L 腿），一次 Provider 调用覆盖多条边 |
| **写穿 + 可续跑 504** | 超时返回 `MATRIX_DEADLINE`，已算边已落 Redis；原请求重试接着补 |
| **模糊 / 严格双模式** | `strict=false` 支持近邻 GEO + 反向边复用；需要确定性时用 strict |
| **租户 / method / strategy 隔离** | 车货画像与客户数据永不串缓存 |
| **多 Key ADCS 池** | 高德 Key 健康度自适应调度，软失败加权与死 Key 探测退避 |
| **可选 MySQL L2** | Redis 走热路径；冷归档通过 DSN 按需开启 |
| **降级可观测** | Provider 失败时 haversine 兜底；看 `fallback` / `provider_calls` 指标，不污染业务响应字段 |

适合自建调度中台、VRP 求解器前置矩阵，以及对成本与多租户隔离有硬要求的路网距离服务。

## 文档

| 文档 | 内容 |
|------|------|
| [文档索引](./docs/README.md) | 总入口 |
| [API](./docs/api-reference.md) | 接口与错误码 |
| [OpenAPI](./docs/openapi.yaml) | 机器可读规格 |
| [架构](./docs/architecture.md) | 缓存 + L2 + 规划器 |
| [Dense 规划器](./docs/design/dense-arc-cover-algorithm.md) | miss 边打包算法 |
| [配置](./docs/configuration.md) | `matrix.yaml` |
| [运维](./docs/operations.md) | 部署与指标 |
| [Key 池](./docs/key-pool-algorithm.md) | 多 Key ADCS |
| [设计](./docs/design/README.md) | 设计文档索引 |

## 快速开始（Docker）

```bash
docker compose -f docker-compose.dev.yml up -d --build
curl -s http://127.0.0.1:8888/health/ready
```

组件：`distance_matrix_app` (:8888)、Redis (:6379)、MySQL (:3306)。  
Compose 内配置：[`etc/matrix.docker.yaml`](./etc/matrix.docker.yaml)（主机名 `redis` / `mysql`）。

```bash
curl -X POST http://127.0.0.1:8888/v1/matrix \
  -H "Content-Type: application/json" \
  -H "X-Tenant-Id: default" \
  -d '{"points":[[116.40,39.90],[116.41,39.91]],"coordinate":"gcj02"}'
```

本机跑二进制（依赖只起 Docker）：

```bash
docker compose -f docker-compose.dev.yml up -d redis mysql
go run matrix.go -f etc/matrix.dev.yaml
```

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/matrix` | N×N 距离 / 时长 |
| POST | `/v1/route` | 多途经点路径 |
| GET | `/v1/providers` | Provider 列表 |
| GET | `/health/live` | 存活检查 |
| GET | `/health/ready` | 就绪检查 |

超时 → **504** `MATRIX_DEADLINE`（部分写穿）。手工调用：[`test/api.http`](./test/api.http)。

## 目录结构

```
matrix.go
etc/matrix.yaml | matrix.dev.yaml | matrix.docker.yaml
api/service.api
docs/
scripts/
  ddl/ genddl/            # 从 GORM model 生成 MySQL schema
  scenario_cache_matrix.py
  capacity_timeout_stress.py
internal/
  handler/ engine/ cache/ persist/ arccover/
  planner/ provider/ loadbalance/ geo/
test/e2e/
```

## 测试

```bash
go test ./...
go test ./test/e2e/ -v
go test ./test/e2e/ -tags=e2e -run TestLive -v   # 真实高德

# 针对已启动的 Compose 栈：
python3 scripts/scenario_cache_matrix.py
python3 scripts/capacity_timeout_stress.py
```

## 架构

```
HTTP → MatrixEngine → Redis (L1) → MySQL L2 (optional)
                   → DensePlanner → AmapProvider
```

## 部署

```bash
docker compose -f docker-compose.dev.yml up -d --build
# or: docker compose up --build
```

DDL：改 `internal/persist/model.go`，再执行 `go run ./scripts/genddl -dsn '...'`（或 `AutoMigrate: true`）。详见 [运维文档](./docs/operations.md)。

## 许可证

见 [LICENSE](./LICENSE)。
