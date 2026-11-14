# 002 / Task 1B — 撤回后撤销文件下载授权

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 945b197`（撤回后端）。
- **结束状态**：本任务提交 `fix(file): revoke download after recall`。

## 修改

`server/internal/file/repository/mysql/repository.go` 的 `canAccessImage`：

- 私聊授权查询增加：
  `NOT EXISTS (SELECT 1 FROM chat_messages AS withdraw WHERE withdraw.msg_type=4 AND withdraw.admin_deleted_at IS NULL AND JSON_UNQUOTE(JSON_EXTRACT(withdraw.msg,'$.withdrawMsg.msgID')) = CAST(chat_messages.id AS CHAR))`
- 群聊授权查询增加同样条件（外层 `gm.id`），并保留“当前仍是 group_members 成员”。
- 保持 `SELECT 1 ... LIMIT 1`，未改 COUNT；所有者路径不查撤回，始终可下载。

## 安全测试

`server/tests/recall_integration_test.go` 新增：

- 发送文件消息（`recall.pdf`）→ 撤回 → `FindAccessibleByID(receiver)` 返回 `ErrNotFound`（对应 `2503`）；
- 所有者 `FindAccessibleByID(sender)` 仍成功；
- 同时验证撤回 ACK/推送/历史占位/重复撤回/窗口/非本人（同一测试）。

真实结果：

```text
=== RUN   TestRecallMessageWebSocketHistoryAndAuthorization
--- PASS: TestRecallMessageWebSocketHistoryAndAuthorization (1.29s)
```

## 性能与边界

- 撤回授权条件为 `NOT EXISTS` + JSON 比较，无索引；仅影响非所有者下载路径，已登记 known-issues（E-8 更新）。
- 原消息行保留，撤回消息非 tombstone 时持续生效。

## Git

- commit：`fix(file): revoke download after recall`（本记录所在提交）。
