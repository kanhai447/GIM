# 001 / Task 1 — 后端撤回消息实现

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 0e0ff81`（撤回协议已定义）。
- **结束状态**：本任务提交 `feat(chat): support recall messages`（文件授权联动单独提交）。

## 修改文件

| 文件 | 变更 |
|---|---|
| `chat/protocol/envelope.go`、`group/ws/protocol/envelope.go` | `WithdrawMessage{msgID}` + `MessagePayload.WithdrawMsg` |
| `chat/message/repository.go`、`group/message/repository.go` | `WithdrawMessageType=4`；`RecallRepository`（GetByIDForRecall / IsRecalled / GetWithdrawBySenderAndTarget / RecalledTargets） |
| 两个 `repository/mysql/repository.go` | 撤回查询实现：目标原始行（含 tombstone）、已撤回判断、同发送者重复撤回、批量被撤回目标；`record` 增加 `admin_deleted_at` 映射 |
| 两个 `message/service.go` | type=4 解析/校验、`sendWithdraw`、时间窗口、幂等、历史/session 替换、preview `[撤回]` |
| 两个 `config/config.go` | `RecallWindow` 读取 `RECALL_WINDOW_SECONDS`（默认 120，1–86400） |
| `cmd/chat-api/main.go`、`cmd/group-api/main.go` | 注入 `RecallWindow` |
| `.env.example`、`.env.compose.example` | `RECALL_WINDOW_SECONDS=120` 正式接线 |
| 测试 | chat/group 单测（成功/历史替换/幂等/非本人/非参与者/tombstone/已撤回/窗口/参数）、protocol 解码、`server/tests/recall_integration_test.go`（WS ACK/推送/历史/重复/窗口） |

## 实现要点

- 私聊：撤回消息 `ReceiverID` 取原消息的接收者；不依赖好友关系（可撤回历史消息）。
- 群聊：校验 `GroupID` 匹配、发送者本人、tombstone/已撤回、窗口，最后 `IsMember`（非成员 `2404`）。
- 错误码：Chat `2218–2222`、Group `2419–2423`（新增 `2222/2423` 用于参数非法）。
- 幂等：先按 sender+target 查已有撤回消息，直接返回同一 ACK；同 clientMsgId 走通用幂等。
- 历史/session：`RecalledTargets` 批量查询后把原消息替换为 `type=4` 占位，preview `[撤回]`，不返回原 payload；分页/游标 ID 不变。
- 无 schema 变更。

## 测试真实结果

```text
go build ./...                 PASS
go vet ./...                   PASS
go test ./internal/chat/message ./internal/group/message ./internal/chat/protocol  PASS
```

真实 MySQL/Redis/WS：

```text
=== RUN   TestRecallMessageWebSocketHistoryAndAuthorization
--- PASS: TestRecallMessageWebSocketHistoryAndAuthorization (1.29s)
```

覆盖：WS 撤回 ACK、接收方收到 type=4、历史占位（原文本不可见）、重复撤回同一 messageId/仅 1 行、非本人 `2219`、tombstone `2221`、超窗口 `2220`。

## 已知边界

- 并发不同 clientMsgId 撤回同一消息在无唯一索引下可能产生两条撤回消息；当前按“顺序幂等”实现，已登记到 known-issues。

## Git

- commit：`feat(chat): support recall messages`（本记录所在提交）。
