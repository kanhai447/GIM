# 验证状态

本文件只记录**真实执行过**的验证结果与**明确未执行**的项目；不把 SKIP/NOT RUN 写成 PASS。最近一次更新：2026-11-12（第四轮整理，数据来自 2026-11-11 第三轮验证）。

## 最近一次真实验证（2026-11-11）

环境：Windows 主机；本机原生 MySQL 8.0、Redis；本机 etcd 3.6.10 临时实例；**Docker daemon 不可用**；**Kafka broker 不可用**。使用隔离库 `gim_verify_*`（Redis DB 15），结束后已删除。

| 项目 | 结果 | 证据 |
|---|---|---|
| `go build ./...` | PASS | 第三轮日志 `operations/day-6/002`、本轮复跑 |
| `go vet ./...` | PASS | 同上 |
| `gofmt -l .` | PASS（无输出） | 同上 |
| `go test ./... -count=1` | PASS（69 个包 ok） | 第三轮日志 |
| 空库 migration 001–008 | PASS | `migrate -command up` 输出 001–008 applied；集成测试覆盖重复 UP/DownAll/dirty/RE-UP |
| `go test ./tests -count=1 -v`（真实 MySQL/Redis/etcd） | **20 PASS / 1 SKIP / 0 FAIL** | `operations/day-6/002` |
| 其中好友并发同意 + 拒绝后再申请 | PASS | `TestFriendLifecycleMySQLAndConcurrency` |
| 好友端到端（真实 Gateway 链路） | PASS（10 步） | `operations/day-6/002` |
| Web `vue-tsc` / `vitest` / `vite build` | PASS / 19 files 103 tests / PASS | 第三轮与第四轮复跑 |
| Admin `vue-tsc` / `vitest` / `vite build` | PASS / 11 files 25 tests / PASS | 同上 |
| `docker compose config --quiet` | PASS（17 services） | 第三/四轮 |
| `scripts/check-repository-safety.ps1` | PASS | 第三/四轮 |

## 未验证项（NOT RUN / SKIP）

| 项目 | 状态 | 原因 / 阻塞项 | 计划 |
|---|---|---|---|
| `go test -race ./...` | **NOT RUN** | 32-bit MinGW 不支持 `windows/amd64` race；`C:\Program Files\mingw64` 的 64-bit GCC 缺 libc headers；Docker 不可用，无法使用 Linux race 环境 | 第五轮 |
| `docker compose up -d --build` + healthy | **NOT RUN** | Docker daemon 不可用；仅验证了 `compose config` | 第五轮 |
| Kafka 真实 produce/consume | **SKIP** | 无 Kafka broker；需 `GIM_KAFKA_INTEGRATION=1` | 第五轮 |
| 性能/soak 基线复跑 | **NOT RUN** | 依赖 Compose 全栈；历史数字来自 2026-10-15 | 第五轮 |
| 浏览器 E2E（Web/Admin 静态站点） | **NOT RUN** | 依赖 Compose；好友 HTTP E2E 已用真实服务覆盖 | 第五轮 |

## 复现命令

```powershell
# 后端单测
cd server
go build ./...; go vet ./...; go test ./... -count=1

# 集成（需要真实 MySQL/Redis/etcd；env 文件含本机凭据，不要提交）
$env:GIM_ENV_FILE = '<absolute path to env file>'
go test ./tests -count=1 -v

# Linux race（需 Docker 或 64-bit C 工具链）
go test -race ./... -count=1

# 前端
cd ..\web; pnpm type-check; pnpm test; pnpm build
cd ..\admin; pnpm type-check; pnpm test; pnpm build

# 仓库安全
./scripts/check-repository-safety.ps1
```

## 历史记录

- 2026-10-15：V1 基线（`gim-v1`）的性能、soak、restart 与 Linux Docker race 证据见 [性能基线](performance/baseline.md) 与 `agent_logs/operations/day-4/`；本轮未复跑。
- 2026-11-11：真实依赖集成与好友 E2E，见 `agent_logs/operations/day-6/`。
