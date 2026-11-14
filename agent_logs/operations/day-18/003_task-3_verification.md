# 003 / Task 3 — 架构文档核对与收尾

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：`docs/architecture-deep-dive.md` 10 章完成；路径抽查通过；未改业务代码、未打 tag。

## 章节完成情况

| 章 | 内容 | 状态 |
|---|---|---|
| 1 | 11 个 cmd 入口/启动顺序/etcd/端口/健康/关闭 | 完成 |
| 2 | Gateway→Auth→服务链路、3 条代表性调用、错误码层 | 完成 |
| 3 | Chat/Group WS Upgrade/Auth/Hub/pump/ACK/推送/Presence/friendnotify | 完成 |
| 4 | 4.1–4.16（认证、用户/Admin、好友、私聊文本/图片/文件/撤回、session、文件、好友提醒、群、Admin、审计、migration） | 完成 |
| 5 | 18 张表 + 索引 + 事务 + docs/04 差异 D1–D9 | 完成 |
| 6 | Hub 单写、锁顺序/1213、幂等、Presence、friendnotify、审计、共享状态 | 完成 |
| 7 | env 键、compose 服务、端口边界、一次性 job | 完成 |
| 8 | 109 Go 测试/21 Web/11 Admin/4 E2E + gated 说明 + validation 出入 | 完成 |
| 9 | K-14/K-15/K-16/E-8/E-9 + 文档实现差异 A1–A11 | 完成 |
| 10 | server/internal、cmd/api/tests/migrations、web、admin 索引 | 完成 |

## 路径抽查（10 个，均存在）

`server/cmd/gateway/main.go`、`server/internal/gateway/handler.go`、`server/internal/chat/hub.go`、`server/internal/chat/friendnotify/notifier.go`、`server/internal/group/ws/hub.go`、`server/internal/chat/message/admin/transport/http/handler.go`、`server/internal/group/admin/mysql/repository.go`、`server/internal/platform/audit/metadata.go`、`web/e2e/private-chat-smoke.mjs`、`admin/src/views/UsersView.vue`。

## 未找到 / 推测清单

1. `chat_message_deletions`、`group_message_deletions`：未找到生产写入方，仅测试 INSERT。
2. `DELETE /api/chat/chat`：未找到 HTTP 入口；`HideSession` 未接线。
3. `groups.is_all_muted`：只读，未找到修改入口。
4. `/api/auth/open_login`：在 Allowlist 但未找到路由（`/api/settings/open_login` 存在）。
5. Group 消息仓库：未找到 1213 死锁重试（Chat 有）。
6. `server/migrations/README.md` 称到 007，实际有 008。
7. docs/04 列名/初始化/草案字段与 migration 存在 D1–D5 差异。
8. validation-status 的好友/图片/文件/撤回/Presence E2E 无入库脚本（仅 4 个 smoke）。
9. `admin/src/components/` 为空目录。
10. 未读 `reference/`；未逐行核对前端组件内部渲染与所有测试断言。

## 文档一致性

- 第 8/9 章已标注 validation-status/E2E、错误码重叠、Group 无 1213 重试等差异；未修改 docs/04–08。
- 未写“全量验证通过/百万连接”。

## Git

- commit：`docs: record architecture document verification`（本记录所在提交）。
- push：**FAILED**（2026-11-14，`Recv failure: Connection was reset`，重试 3 次仍失败）；本地 `main` 领先 `origin/main` 3 个 docs commit，未推送。未打 tag、未改业务代码。
