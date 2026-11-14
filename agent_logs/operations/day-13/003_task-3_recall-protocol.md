# 003 / Task 3 — 撤回协议设计（不实现）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 4c4a2cc`。
- **结果**：撤回协议写入 docs/06 §5.6、docs/05/07/04；**未写实现代码**。

## 阅读与兼容性结论

| 现有结构 | 结论 |
|---|---|
| `chat_messages.msg` / `group_messages.msg`（JSON） | 可容纳 `{"type":4,"withdrawMsg":{"msgID":...}}`；无需新增列 |
| `MessagePayload`（Go） | 增加 `WithdrawMsg` 字段即可；不影响 type=1/2/3 |
| 历史/session 查询 | 实现时需读取撤回消息并把被引用原消息替换为占位；原行保留 |
| 文件授权 SQL | 实现时需排除“被未 tombstone 撤回消息引用的原消息”；仍复用现有表，无需索引设计 |
| 幂等 | 撤回消息沿用 `(send_user_id, client_msg_id)` 唯一键；同一 msgID 重复撤回返回已有 ACK |

**不需要 schema 变更**：不新增 `recalled_at` 列，不新增表/索引。

## 协议要点

- `msg.type=4`，`withdrawMsg.msgID` 指向原消息；沿用 `chat.send`/`group.send`，不新增 WS 事件。
- 校验：存在/属于会话、本人发送、未 tombstone、未撤回、`RECALL_WINDOW_SECONDS`（默认 120）、群聊仍为成员。
- 错误码：Chat `2218–2221`、Group `2419–2422`；群非成员沿用 `2404`。
- 历史：原消息替换为 type=4 占位、`[撤回]`、不返回原内容；撤回消息自身可见；前端按 msgID 替换本地消息。
- 文件授权：撤回后非所有者不能再下载原消息引用的文件；所有者仍可下载自己的 `user_files`。
- 兼容性：旧客户端显示“消息已撤回”占位；type=1/2/3 不变。
- 管理员撤回：V2 后续，本轮不设计。

## 修改文件

- `docs/06_WEBSOCKET_PROTOCOL.md`：类型表加 type=4；新增 §5.6；重写 §6 指向协议。
- `docs/05_HTTP_API.md`：撤回错误码表；下载授权补充撤回语义。
- `docs/07_CORE_FLOWS.md`：新增 §2.3 撤回流程。
- `docs/04_DATABASE_DESIGN.md`：撤回复用 msg JSON、无新列、`[撤回]` preview。
- `.env.example`：预留 `RECALL_WINDOW_SECONDS=120`（注明第十一轮实现使用）。

## 未做

- 未实现撤回后端/前端；未改 schema；未新增路由/事件；未打 tag。

## 验收

- 协议文档先于实现提交；与现有 msg JSON 无冲突，无需 schema 变更。

## Git

- commit：`docs: define recall message protocol`（本记录所在提交）。
