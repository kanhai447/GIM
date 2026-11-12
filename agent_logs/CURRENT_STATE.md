# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计；范围见 `AGENTS.md` §5.1）
- **Current Round:** 2026-11-12 Day 7 — 项目整理（文档一致性、README、代码整洁、日志结构、Git 整洁）：COMPLETED
- **Branch:** `main`
- **Working Tree:** 本日志提交前 clean
- **Next Authorized Work:** 第五轮 — 补 race / Compose / Kafka / 性能验证；是否打 `gim-v1.1` 由用户决定

## 当前事实

- 根 README 与 server/web/admin 子 README 已按当前实现重写；`docs/validation-status.md` 记录真实验证矩阵，`docs/known-issues.md` 记录待补验证与 V2 限制。
- 契约文档与实现对齐：docs/05（HTTP 路由 + 11xx 错误码）、docs/04（§7.11 表/索引/约束清单）、docs/06（V1 事件清单，friend.online/offline 标 V2）、docs/01（服务职责 + `cmd/` 入口 + 17 services）。
- 死代码已清理：`internal/project`、`FeaturePlaceholder.vue`、`ModulePlaceholder.vue`、无调用方的 `chat.New`。
- 无读取配置键已删除：`APP_ENV`、`LOG_LEVEL`、`WEB_ORIGIN`、`ADMIN_ORIGIN`。
- 过期 UI 文案已改写：HomeView 展示 V1 能力，Admin 头部改为 “GIM V1 · Admin”。
- `agent_logs/operations/README.md` 统一定义目录/编号/字段约定与修正政策；day-4 的 checkpoint 作用域编号保留并给出映射。

## 验证矩阵（最近真实执行）

| 项目 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | PASS / PASS / 无输出 |
| `go test ./... -count=1` | PASS（69 个包 ok） |
| 集成测试（真实 MySQL/Redis/etcd，2026-11-11） | 20 PASS / 1 SKIP / 0 FAIL |
| 好友端到端（真实 Gateway，2026-11-11） | PASS（10 步） |
| Web type-check / vitest / build | PASS / 19 files 103 tests / PASS |
| Admin type-check / vitest / build | PASS / 11 files 25 tests / PASS |
| `docker compose config --quiet` | PASS（17 services） |
| `scripts/check-repository-safety.ps1` | PASS |

## 未验证项（留第五轮）

- `go test -race ./...`：NOT RUN（32-bit MinGW 不兼容；64-bit MinGW 缺 libc headers；Docker 不可用）。
- Compose build/up + healthy：NOT RUN（daemon 不可用）。
- Kafka 真实 produce/consume：SKIP（无 broker）。
- 性能/soak/restart 与浏览器 E2E：NOT RUN。
- 详见 `docs/validation-status.md` 与 `docs/known-issues.md`。

## V2 / 剩余限制

- 富消息（图片/视频/文件/语音/回复/引用/撤回/@/图文）、好友上线提醒、`user_confs` 读写、`DELETE /api/chat/chat`、`POST /api/user/valid`、`open_login`、delivery/read receipt、WS Ticket、多实例路由、文件 GC、审计 outbox/DLQ。
- 好友申请只提供接收方向；`isOnline` 非实时推送。

## Tag 与 Push

- `gim-v1`：保留不动（2026-10-15 V1 基线）。
- `gim-v1.1`：未创建；待第五轮验证通过后由用户确认。
- 本轮 commit 已推送到 `origin/main`；未使用 `--force`，未移动/删除任何 tag。
