# 验证状态

本文件只记录**真实执行过**的验证结果与**明确未执行**的项目；不把 SKIP/NOT RUN 写成 PASS。最近更新：2026-11-12（第五轮补验证 + V2 Phase 1 图片消息）。

## 本轮新增验证（2026-11-12）

环境：Windows 主机 + Docker Desktop 29.4.0（Linux containers）；Compose 全栈；Kafka 4.2.2 KRaft（Compose service）。本机 MySQL/Redis 与临时 etcd 仍可用。

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| **race（全量）** | **PASS** | `docker run --rm -v K:\it\gim\server:/src -v C:\Users\hfhy0\go\pkg\mod:/gomodcache -w /src -e CGO_ENABLED=1 -e GOMODCACHE=/gomodcache -e GOPROXY=off -e GOFLAGS=-mod=mod golang:1.25-bookworm bash -c "go test -race ./... -count=1"` | exit 0；全部包 ok；无 race warning。重点包 `internal/chat/...`、`internal/user/friend/...` 均通过 |
| **Compose 全栈** | **PASS** | `docker compose up -d --build`；`docker compose ps -a` | 17 services = 15 running healthy + `migration`/`storage-init` exit 0；`docker compose up -d` 复跑 exit 0 |
| **浏览器好友 E2E** | **PASS** | Chrome + `playwright-core`，目标 `http://127.0.0.1:5173`（Nginx → Gateway） | 11 步：注册 A/B、登录、搜索、申请、同意、好友列表、备注、删除、双方列表清空 |
| **Kafka 真实消费** | **PASS** | `GIM_ENV_FILE=.tmp/env.compose-host GIM_KAFKA_INTEGRATION=1 go test ./tests -run TestKafkaAudit -count=1 -v`，broker `127.0.0.1:19092` | `TestKafkaAuditProduceConsumePersistQueryRestartAndIdempotency` PASS（1.59s）；`TestKafkaAuditUnavailableBrokerKeepsPublishBounded` PASS |
| **性能最小场景** | **PASS（仅两档）** | `go run ./tools/loadtest prepare/http/private/cleanup`，全部经 Gateway | HTTP profile c10/10s：27281/27281，2728 req/s，p95 4.834 ms；Private 10 conn/10s@10 msg/s：100/100 ACK、100/100 delivery、p95 22.859 ms、correctness PASS |

## V2 Phase 1 图片消息（2026-11-12）

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| 后端构建/单测 | **PASS** | `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1`（69 packages） | 图片解析、服务端归一化、归属/MIME/大小校验、幂等、错误码 |
| **聊天图片集成（真实 MySQL/Redis/WS）** | **PASS** | `GIM_ENV_FILE=<absolute .env.local>`；`go test ./tests -run TestChatImage -count=1 -v` | `TestChatImageMessageWebSocketHistoryAndValidation` PASS：ACK、接收方 `chat.message`、历史、重复 clientMsgId 仅 1 行、`2211/2212/2213` 不落库、接收方可解析文件、非参与者 `ErrNotFound` |
| 群聊图片 | **PASS（单测）** | `go test ./internal/group/message -count=1` | 归一化、幂等冲突、`2412–2415`；未单独跑群聊图片 WS E2E |
| Web | **PASS** | `vue-tsc --noEmit`；`vitest run`（21 files / 114 tests）；`vite build` | WS 解析、上传后发送、pending/ACK、渲染、预览、失败重试、未知占位 |
| **浏览器图片 E2E** | **PASS** | Chrome + Compose 17 services（15 healthy + 2 exit 0） | 10 步：注册 A/B、登录、好友、A 上传并发送、B 收到图片、B 打开预览、关闭。首轮曾因接收方无下载权限失败，修复（`a1ecf64`）后 PASS |
| 图片相关 race | **NOT RUN** | 上一轮 race 覆盖 V1 代码；图片代码未单独跑 | 建议下一轮补 |
| 群聊图片浏览器 E2E / 图片性能 | **NOT RUN** | 本轮未做 | 可选 |

## 既有验证（2026-11-11，真实 MySQL/Redis/etcd）

| 项目 | 结果 | 证据 |
|---|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | PASS / PASS / 无输出 | `operations/day-6/002`、本轮复跑 |
| `go test ./... -count=1` | PASS（69 个包） | 同上 |
| 空库 migration 001–008 | PASS（含重复 UP / DownAll / dirty / RE-UP） | `TestDatabaseMigrationsAndUserAuthRegression` |
| 集成测试（真实 MySQL/Redis/etcd） | **20 PASS / 1 SKIP / 0 FAIL** | `operations/day-6/002`；SKIP 为 Kafka 用例，本轮已用 Compose broker 补齐 |
| 好友端到端（HTTP，真实 Gateway） | PASS（10 步） | `operations/day-6/002` |
| Web `vue-tsc` / `vitest` / `vite build` | PASS / 19 files 103 tests / PASS | 第四/五轮复跑 |
| Admin `vue-tsc` / `vitest` / `vite build` | PASS / 11 files 25 tests / PASS | 同上 |
| `docker compose config --quiet` | PASS（17 services） | 第四/五轮 |
| `scripts/check-repository-safety.ps1` | PASS | 第四轮 |

## 仍未验证（NOT RUN）

| 项目 | 状态 | 原因 / 边界 | 计划 |
|---|---|---|---|
| 完整性能阶梯（25/50/100/200 WS 连接、50 msg/s） | **NOT RUN** | 本轮只复跑最小两档；未做阶梯与容量探索 | 可选后续轮次 |
| 5 分钟 soak / restart 稳定性 | **NOT RUN** | 历史结果仍为 2026-10-15 记录，未复跑 | 可选后续轮次 |
| 文件吞吐档位（1 MiB/10 MiB、并发） | **NOT RUN** | 本轮未跑 file 负载 | 可选后续轮次 |
| 图片代码 `-race` / 群聊图片 E2E | **NOT RUN** | 图片功能在 race 验证之后加入，未单独复跑；群聊图片仅单测 | 下一轮 |
| 远端 GitHub Actions 结果 | **NOT OBSERVED** | 本环境无 GitHub API 访问；仅 local equivalent PASS | — |

> 历史性能数字（`docs/performance/baseline.md`）是 2026-10-15 的记录；本轮最小场景结果只代表当前机器当前栈，不构成容量承诺，也不替代历史完整档位。

## 复现命令

```powershell
# 后端单测
cd server
go build ./...; go vet ./...; gofmt -l .; go test ./... -count=1

# 真实依赖集成（MySQL/Redis/etcd；Kafka 用例需 broker + 开关）
$env:GIM_ENV_FILE = '<absolute path to env file>'
go test ./tests -count=1 -v
$env:GIM_KAFKA_INTEGRATION = '1'
go test ./tests -run TestKafkaAudit -count=1 -v

# Linux race（Docker；挂载本机 module cache 可离线）
docker run --rm -v <repo>\server:/src -v <GOMODCACHE>:/gomodcache -w /src `
  -e CGO_ENABLED=1 -e GOMODCACHE=/gomodcache -e GOPROXY=off -e GOFLAGS=-mod=mod `
  golang:1.25-bookworm bash -c "go test -race ./... -count=1"

# Compose 全栈
Copy-Item .env.compose.example .env.compose.local   # 替换 CHANGE_ME
docker compose up -d --build
docker compose ps -a

# 前端
cd web; pnpm type-check; pnpm test; pnpm build
cd ..\admin; pnpm type-check; pnpm test; pnpm build

# 仓库安全
./scripts/check-repository-safety.ps1
```

## 历史记录

- 2026-10-15：V1 基线（`gim-v1`）的性能、soak、restart 与 Linux Docker race 证据见 [性能基线](performance/baseline.md) 与 `agent_logs/operations/day-4/`。
- 2026-11-11：真实依赖集成与好友 E2E，见 `agent_logs/operations/day-6/`。
- 2026-11-12：race / Compose / Kafka / 浏览器 E2E / 性能最小场景，见 `agent_logs/operations/day-8/`。
- 2026-11-12：V2 Phase 1 图片消息（单测 / 聊天集成 / 浏览器 E2E），见 `agent_logs/operations/day-10/`。
