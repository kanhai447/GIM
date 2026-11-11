# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（范围已收敛，见 `AGENTS.md` §5.1）
- **Current Round:** 2026-11-11 范围收敛（C1/C2 + 好友闭环 + 文档口径 + 删除 group-rpc）— COMPLETED
- **Branch:** `main`
- **Working Tree:** 本轮日志提交后复核 clean
- **Next Authorized Work:** NONE — 等待用户确认下一阶段

## 本轮变更

- **并发修复**：Admin 私聊删除的 `repairSession` 不再吞掉非 `RecordNotFound` 错误（C1）；admin delete 与发消息统一按 user_id 升序加锁，并对 MySQL 1213 做最多 3 次、10/20 ms、context-aware 的事务重建重试（C2）。
- **好友闭环**：新增 `/api/user/search`、`/api/user/friends`（GET/POST/PUT/DELETE）、`/api/user/valid`、`/api/user/valid_status`、`/api/user/friend_info`；事务内行锁 + pending 唯一键保证并发同意只成功一次；好友列表读取 Redis Presence 并在故障时降级离线。用户端新增 `/contacts` 页面、API、Pinia store 与组件测试。
- **文档口径**：`AGENTS.md` §5.1、README、`server/`/`web/`/`admin/` README、docs 05/06/07/10、面试材料统一为“V1 已实现 / V2 未实现”。
- **拓扑收敛**：删除无调用方的 `cmd/group-rpc`、`internal/group/transport/grpc`、`api/group/v1`，同步 Compose（18 services）、Dockerfile、环境模板和拓扑文档；当前只有 User RPC。

## 组件状态

- Auth / User / Gateway：FUNCTIONAL；`go test ./...` 覆盖。
- 好友闭环：FUNCTIONAL（单测 + 前端测试）；真实 MySQL 集成用例已写但本轮未执行。
- Private / Group Chat：文本消息、ACK、幂等、session/unread 状态未改动；本轮只改 admin 删除路径与锁顺序。
- File / Settings / Logs：未改动。
- User Web：19 files / 103 tests、type-check、build PASS。
- Admin Web：11 files / 25 tests、type-check PASS；本轮未改动。
- Compose：`docker compose config --quiet` PASS；daemon 未运行，未做 build/up。

## 验证矩阵（本轮真实执行）

| 项目 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | PASS / PASS / 无输出 |
| `go test ./... -count=1` | PASS（69 个包 ok） |
| C1 回归测试（修复前失败、修复后通过） | PASS |
| C2 锁顺序/重试/退避单测 | PASS |
| 好友 service / HTTP 单测 | PASS |
| Web type-check / vitest / build | PASS / 19 files 103 tests / PASS |
| Admin type-check / vitest | PASS / 11 files 25 tests |
| `docker compose config --quiet` | PASS（18 services） |
| `scripts/check-repository-safety.ps1` | PASS |

## 本轮未执行

- 20 个真实依赖集成测试（需要 `GIM_ENV_FILE` + MySQL/Redis/etcd/Kafka；Docker 未运行）。
- 好友并发同意的真实 MySQL 运行（代码与断言已写）。
- `go test -race`：Windows native 因 32-bit MinGW 不可用，Linux Docker 不可用。
- Compose build/up、migration UP/DOWN、浏览器 E2E、性能基线复跑。

## V2 / 剩余限制

- 富消息（图片/视频/文件/语音/回复/引用/撤回/@/图文）、好友上线提醒、`user_confs` 读写、`DELETE /api/chat/chat`、`POST /api/user/valid`、`open_login`、delivery/read receipt、WS Ticket、多实例路由、文件 GC、审计 outbox/DLQ。
- `.env.example` 仍保留未被读取的 `AUTH/CHAT/FILE/SETTINGS_RPC_PORT` 占位键（本轮只授权删除 `GROUP_RPC_*`）。

## Freeze

- 本轮四个任务均已完成：C1/C2、好友闭环、文档收缩、删除 group-rpc。
- 未打新 tag；未执行 `--force` push。
- **Push 状态**：`05dc348`、`729b8aa`、`a97bee6`、`0723004`、`f0d3b6d` 已推送到 `origin/main`；`83db97a`、`bcacbae`、`3032875` 及推送状态修正提交仅本地。收尾时 GitHub 不可达（`Recv failure: Connection was reset` / `Could not connect to server`），重试两次失败。恢复网络后执行 `git push origin main` 即可补齐。
