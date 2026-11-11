# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（范围见 `AGENTS.md` §5.1；本轮补齐真实依赖验证）
- **Current Round:** 2026-11-11 Day 6 — 补推 + 真实依赖验证 + 拒绝再申请语义 + 遗留清理：COMPLETED
- **Branch:** `main`
- **Working Tree:** 本日志提交前 clean
- **Next Authorized Work:** NONE — 等待用户确认是否进入 V2

## 本轮变更

- **补推**：round 2 的 4 个本地 commit 已推送，`14aebef` 与远端一致。
- **真实依赖验证**：本机 MySQL/Redis + 临时 etcd 上完成空库 migration（001–008）、20 个集成测试（20 PASS / 1 SKIP / 0 FAIL）、好友 10 步端到端（真实 Gateway→Auth→User API→User RPC→MySQL）。
- **好友语义**：明确“拒绝后允许再次申请、历史保留、同时只允许一条 pending”，MySQL 集成测试覆盖；无代码/schema 改动。
- **清理**：删除 `.env.example` 中 4 个无读取的 `*_RPC_PORT`；确认 `server/api/group/v1` 无残留/无 import；docs/05 新增 11xx 错误码分配表（1113–1119 无冲突）。
- **计数修正**：Compose 服务数实际为删除前 18、删除后 17（原记录 19/18 为误计）。

## 验证矩阵（Day 6 真实执行）

| 项目 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | PASS / PASS / 无输出 |
| `go test ./... -count=1` | PASS（69 个包 ok） |
| 空库 migration（隔离库） | PASS（001–008 applied；集成测试覆盖重复 UP / DownAll / dirty 保护 / RE-UP） |
| 集成测试 `go test ./tests -count=1 -v`（真实 MySQL/Redis/etcd） | **20 PASS / 1 SKIP / 0 FAIL** |
| 好友端到端（真实服务进程 + Gateway） | PASS（10 步；`isOnline` 字段存在） |
| 好友并发同意 + 拒绝后再申请 | PASS（真实 MySQL） |
| Web type-check / vitest / build | PASS / 19 files 103 tests / PASS |
| Admin type-check / vitest / build | PASS / 11 files 25 tests / PASS |
| `docker compose config --quiet` | PASS（17 services） |
| `scripts/check-repository-safety.ps1` | PASS |

## 未验证项（本轮 NOT RUN / SKIP）

- **Race**：NOT RUN。32-bit MinGW 报 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`；`C:\Program Files\mingw64` 的 64-bit GCC 8.1.0 缺少 libc headers（`stddef.h`）；Docker 不可用，无法使用 Linux race 环境。
- **Compose 全栈 build/up**：NOT RUN。Docker daemon 未运行；`docker compose config` 仅验证配置。
- **Kafka 真实消费链路**：SKIP。无 broker；需 `GIM_KAFKA_INTEGRATION=1` + Kafka。
- **性能基线复跑**：NOT RUN。历史数字仍为 2026-10-15 记录。

## 组件状态

- Auth / User / Gateway / Chat / Group / File / Settings / Logs：FUNCTIONAL；本轮未改业务实现（除测试与配置清理）。
- 好友闭环：FUNCTIONAL；后端单测、前端测试、真实 MySQL 集成、真实 Gateway E2E 均有证据。
- 私聊/群聊：文本消息、ACK、幂等、session/unread、Presence 未改动。
- Admin Web：本轮未改动，回归 PASS。

## V2 / 剩余限制

- 富消息（图片/视频/文件/语音/回复/引用/撤回/@/图文）、好友上线提醒、`user_confs` 读写、`DELETE /api/chat/chat`、`POST /api/user/valid`、`open_login`、delivery/read receipt、WS Ticket、多实例路由、文件 GC、审计 outbox/DLQ。
- 好友申请列表只提供接收方向；`isOnline` 非实时推送。

## Freeze

- Day 6 四个任务完成；未打新 tag；未使用 `--force`。
- **Push 状态**：任务 1–4 的 commit 已推送（`14aebef..df82d59`）；本日志提交随后推送，最终以 `origin/main` 为准。
