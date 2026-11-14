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

## V2 Phase 1 收尾（2026-11-14）

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| **图片代码 race（全量）** | **PASS** | Linux Docker `golang:1.25-bookworm`；`go test -race ./... -count=1`（挂载 module cache，`GOPROXY=off`） | exit 0；含 `internal/chat/...`、`internal/group/...`、`internal/file/...`、`internal/user/friend/...`；无 `WARNING: DATA RACE` |
| **文件下载授权专项测试** | **PASS** | 真实 MySQL（001–008 schema）；`go test ./tests -run 'TestImageDownloadAuthorizationSemantics\|TestTextMessageDoesNotGrantImageDownload' -v` | 所有者、私聊接收方、群成员、非参与者 `2503`、私聊 tombstone、群 tombstone、退群后 `2503`、未知 fileID `2503`、文本不授权；接收方下载在私聊/群聊集成与 E2E 中验证 |
| **群聊图片 WS E2E** | **PASS** | Chrome + Compose（15 healthy + 2 exit 0） | A 建群邀请 B/C → A 发送 type=2 → B/C 收到并预览 → 非成员 D `2503` → B 退群后 `2503` → owner A 仍可下载。首轮“退群后 200”确认是浏览器缓存，改 `cache: 'no-store'` 后复跑 PASS |
| 图片吞吐 / 预览性能 | **NOT RUN** | 本轮未做 | 可选 |

## V2 Phase 1 普通文件消息（2026-11-14）

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） | type=3 解析、归属/大小/参数校验、幂等、`[文件]` preview |
| **聊天文件集成（真实 MySQL/Redis/WS）** | **PASS** | `GIM_ENV_FILE=<absolute .env.local>`；`go test ./tests -run TestChatFileMessage -v` | `TestChatFileMessageWebSocketHistoryAndValidation`：ACK、接收方 `chat.message`（fileName/size/mime 规范化）、历史、重复 clientMsgId 仅 1 行、`2215/2216` 不落库、接收方可解析文件、非参与者不可 |
| 群聊文件消息 | **PASS（单测）** | `go test ./internal/group/message -count=1` | 归一化、幂等冲突、`2416–2418`；群文件消息授权在授权测试中覆盖 |
| 文件下载授权（文件消息） | **PASS** | `TestImageDownloadAuthorizationSemantics`（含群文件消息） | 群成员可下载；tombstone 后/退群/非成员 `2503`；owner 仍可下载 |
| Web | **PASS** | `vue-tsc --noEmit`；`vitest run`（21 files / 122 tests）；`vite build` | type=3 解析、上传后发送、文件卡片、鉴权下载、失败重试 |
| **浏览器文件 E2E** | **PASS** | Chrome + Compose 17 services（15 healthy + 2 exit 0） | 8 步：A 上传 `report.pdf` 并发送 → B 收到文件卡片 → B 下载 `report.pdf` → 非参与者 D `2503` |
| 文件消息吞吐 / 下载性能 | **NOT RUN** | 本轮未做 | 可选 |

## 文件 Phase 收尾（2026-11-14）

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| **文件 Phase 全量 race** | **PASS** | Linux Docker `golang:1.25-bookworm`；`go test -race ./... -count=1`（挂载 module cache，`GOPROXY=off`） | exit 0；含 `internal/chat/...`、`internal/group/...`、`internal/file/...`、`internal/user/friend/...`；无 `WARNING: DATA RACE` |
| **群聊文件浏览器 E2E** | **PASS** | Chrome + Compose（15 healthy + 2 exit 0） | A 建群邀请 B/C → A 发 `report.pdf`（type=3）→ B/C 收到并下载 → 非成员 D `2503` → B 退群后 `2503` → owner A 仍可下载 |

## 撤回实现（2026-11-14，第十一轮）

| 项目 | 结果 | 命令 / 环境 | 证据 |
|---|---|---|---|
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） | type=4 解析、校验顺序、时间窗口、幂等、非本人/非参与者/tombstone/已撤回/群非成员、历史替换 |
| **撤回集成（真实 MySQL/Redis/WS）** | **PASS** | `GIM_ENV_FILE=<absolute .env.local>`；`go test ./tests -run TestRecallMessage -v` | WS 撤回 ACK、接收方 type=4、历史占位不含原文、重复撤回同一 messageId、非本人 `2219`、tombstone `2221`、超窗口 `2220`、撤回后接收方文件下载 `ErrNotFound`、所有者仍可下载 |
| 文件授权联动 | **PASS** | 同一集成测试 | 授权 SQL 增加 `NOT EXISTS` 排除被撤回原消息；保持 `SELECT 1 … LIMIT 1`，无 schema 变更 |
| Web | **PASS** | `vue-tsc --noEmit`；`vitest run`（21 files / 131 tests）；`vite build` | 撤回入口/窗口判断、type=4 解析、发送/ACK 替换、推送合并、失败保留原文、占位渲染 |
| **浏览器撤回 E2E** | **PASS** | Chrome + Compose（15 healthy + 2 exit 0） | A 撤回文本 → B 看到占位且原文消失；A 撤回文件 → B 卡片消失、下载 `2503`；owner 仍 `200` |
| 并发撤回测试 | **PASS** | `go test ./internal/chat/message -run TestConcurrentRecallSameTargetIsSafe -v` | 本次观察到 1 条撤回消息（调度相关，不强行断言）；历史占位唯一；前端合并去重；K-14 记录边界 |
| **撤回窗口短配置 E2E** | **PASS** | `RECALL_WINDOW_SECONDS=3`，重启 chat/group；Chrome + Compose | 3s 内撤回成功；4s 后撤回返回 `2220` 且原消息保留；B 看到占位；已恢复默认 120s |
| 撤回代码全量 race | **PASS** | Linux Docker `golang:1.25-bookworm`；`go test -race ./... -count=1` | exit 0，无 `WARNING: DATA RACE` |
| 撤回 schema | 无需变更 | 复用 `msg` JSON；无 `recalled_at` 列/表/索引 | — |

## 好友上线提醒（2026-11-14，第十三轮，单机）

| 项目 | 结果 | 命令 / 环境 | 说明 |
|---|---|---|---|
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（70 packages） | friendnotify 入队、发送、自己过滤、查询失败、channel 满、shutdown drain、并发 enqueue |
| **好友提醒集成（真实 MySQL/Redis/WS）** | **PASS** | `GIM_ENV_FILE=<absolute>`；`go test ./tests -run TestFriendPresenceNotifications -v` | A 登录→B online；多端只一次 online；关一端无 offline；关最后 offline；非好友 C 不通知；禁用好友 D 不通知 |
| Web | **PASS** | `vue-tsc --noEmit`；`vitest run`（21 files / 135 tests） | 事件解析、friend store 更新、未知 userID 忽略、HTTP 列表合并 |
| **浏览器 E2E** | **PASS** | Chrome + Compose | B 看到 A 在线；A 关闭→B 离线；A 重登→B 在线 |
| 全量 race | **PASS** | Linux `golang:1.25-bookworm` `go test -race ./...` | exit 0，无 `WARNING: DATA RACE` |
| 多实例全局 Presence | **NOT RUN** | 本轮只做单机 | V2；`GlobalPresenceCheck` 占位（V-20/K-15） |
| Chat Hub 并发模型 | 未修改 | 现有 observer + 装配层组合 | 回调只入队，不阻塞 Hub |

## V2 收口（2026-11-14）

- 范围、V2 后期/不做项、已知边界、验证摘要与 tag 语义见 **[v2-scope.md](v2-scope.md)**。
- V2 核心四 Phase：图片 `type=2`、普通文件 `type=3`、撤回 `type=4`、好友上线提醒（单机）均 PASS（明细见上表）。
- 本文既有 **NOT RUN / NOT OBSERVED** 项保持不变：完整性能阶梯、5 分钟 soak/restart、图片/文件吞吐与预览性能、远端 GitHub Actions。
- 不声称“全量验证通过”或“支持百万连接”。

## E2E 证据链说明（2026-11-14 核实）

仓库 `web/e2e/` 仅保留 4 个通用 smoke：`private-chat-smoke.mjs`、`group-chat-smoke.mjs`、`file-management-smoke.mjs`、`admin-compose-smoke.mjs`。本页标 **PASS** 的以下专项 E2E 均为**临时脚本执行、未入库**（执行后删除），证据是当日操作日志；复现需按 docs/06/07 与操作日志重写脚本：

| 场景 | 证据 |
|---|---|
| 好友 E2E | `agent_logs/operations/day-6/002_task-2_real-dependency-verification.md` |
| 图片 E2E | `agent_logs/operations/day-10/004_task-3b_image-e2e-and-access-fix.md` |
| 群聊图片 E2E | `agent_logs/operations/day-11/005_task-4_group-image-e2e.md` |
| 文件 E2E | `agent_logs/operations/day-12/005_task-3b_file-e2e.md` |
| 群聊文件 E2E | `agent_logs/operations/day-13/002_task-2_group-file-e2e.md` |
| 撤回 E2E | `agent_logs/operations/day-14/003_task-2_frontend-recall.md` |
| 撤回窗口 E2E | `agent_logs/operations/day-15/002_task-1b_window-e2e.md` |
| 好友上线 E2E | `agent_logs/operations/day-16/003_task-2_frontend-implementation.md` |

因此：这些 PASS 是真实执行记录，但**不是仓库内可一键复现的脚本**；不得在简历/README 中表述为“E2E 脚本已入库”。

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
| 图片上传/下载吞吐与预览性能 | **NOT RUN** | 未单独做图片性能档位 | 可选后续轮次 |
| 普通文件消息吞吐/下载性能 | **NOT RUN** | 未单独做文件消息性能档位 | 可选后续轮次 |
| 好友上线提醒多实例 | **NOT RUN** | 单机已 PASS；全局 Presence 为 V2 | V-20 |
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
- 2026-11-14：图片 Phase 1 收尾（图片 race / 授权专项测试 / 群聊图片 E2E），见 `agent_logs/operations/day-11/`。
- 2026-11-14：V2 Phase 1 普通文件消息（单测 / 聊天文件集成 / 浏览器文件 E2E），见 `agent_logs/operations/day-12/`。
- 2026-11-14：文件 Phase 收尾（全量 race / 群聊文件 E2E）+ 撤回协议设计，见 `agent_logs/operations/day-13/`。
- 2026-11-14：撤回实现（单测 / 集成 / 文件授权 / 浏览器 E2E），见 `agent_logs/operations/day-14/`。
- 2026-11-14：撤回收尾（并发测试 / 窗口 E2E / 全量 race）+ 好友上线提醒协议设计，见 `agent_logs/operations/day-15/`。
- 2026-11-14：好友上线提醒实现（后端/前端/集成/E2E/race，单机），见 `agent_logs/operations/day-16/`。
