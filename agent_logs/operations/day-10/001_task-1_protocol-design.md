# 001 / Task 1 — 图片消息协议设计（文档先行）

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = 78aa9c5`，工作区干净。
- **结果**：协议文档完成，未改代码。

## 参考的现有实现（阅读路径）

| 文件 | 关键结论 |
|---|---|
| `server/internal/chat/protocol/envelope.go` | `MessagePayload{Type uint8, TextMsg *TextMessage}`；`DecodeData` 用 `DisallowUnknownFields`，新增 `imageMsg` 字段即可向后兼容 |
| `server/internal/group/ws/protocol/envelope.go` | 群聊独立 `MessagePayload`，结构同上 |
| `server/internal/chat/message/service.go` | `validate` 仅接受 `type=1`；`preview()` 取文本前 30 rune；`sameLogicalMessage` 比较 type+text content |
| `server/internal/group/message/service.go` | 同样仅接受 `type=1`；错误码 `2401–2411` |
| `server/internal/chat/message/repository.go` / `group/.../repository.go` | `Message.Payload` 已是 `protocol.MessagePayload`；`msg` 列存 JSON，无需改表 |
| `server/internal/chat/message/transport/http/handler.go` / `group/.../handler.go` | history DTO 直接复用 `protocol.PrivateMessage` / `GroupMessage`，新增字段自动进入历史 |
| `server/internal/file/domain/file.go`、`repository/repository.go` | `UserFile{ID,UserID,FileObjectID}`、`FileObject{Size,MIMEType}`；已有 `GetOwnedByID(userID,id)` 语义 |
| `server/internal/file/transport/http/handler.go` | 上传响应含 `id/uid/fileName/size/mimeType/sha256/url/src`；`POST /api/file/image` 字段 `image` |
| `web/src/types/chat.ts` / `group.ts` | `ChatMessagePayload`/`GroupTextPayload` 目前是单一 type=1 接口 |
| `web/src/service/ws/chat_socket.ts` / `group_socket.ts` | `isMessage` 校验 `msg.type === 1 && textMsg.content`，需扩展 type=2 |
| `web/src/stores/chat.ts` / `group.ts` | `sendText` 生成 pending → `sendPending`；ACK 以 `clientMsgId` 合并 |
| `web/src/stores/chat_reconcile.ts` | `messagePreview` 直接读 `textMsg.content`，需处理图片 |
| `web/src/components/chat/ChatMessageList.vue` / `group/GroupMessageList.vue` | 直接渲染 `msg.textMsg.content`，需按 type 分支 |

## 设计决策

1. **`msg.type=2`，新增 `imageMsg` 字段**；不新增表/列，复用 `chat_messages.msg` / `group_messages.msg` JSON。
2. **字段**：`fileID`（必填，`user_files.id`）、`url`、`width`、`height`、`size`、`mimeType`；服务端以 DB 的 `file_objects.size/mime_type` 为准回填 `size`/`mimeType`/`url`，`width/height` 仅作客户端显示提示（1..20000，非法则拒绝）。
3. **校验顺序**：归属（user_files.user_id）→ MIME（image/）→ 大小（`IMAGE_MESSAGE_MAX_BYTES`，默认 10 MiB）→ 参数；失败不落库、不 ACK。
4. **错误码**：Chat `2211` 文件不存在/非本人、`2212` 非图片、`2213` 超限、`2214` 参数非法；Group `2412–2415` 对应；未知类型保持 `2201/2403`；幂等冲突 `2205/2407`。
5. **幂等**：同 sender/clientMsgId 只生成一条；比较包含 type + fileID，换图视为冲突。
6. **兼容性**：type=1 完全不变；旧客户端收到 type=2 显示占位；旧服务端返回 2201/2403；上传与发送两步。
7. **配置**：`IMAGE_MESSAGE_MAX_BYTES`（默认 10485760），由 chat-api 与 group-api 共读；写入 `.env.example` 与 docs/08。
8. **前端**：私聊/群聊共用图片消息组件；下载走带 token 的 File API（`<img src>` 不会带 Authorization，因此先取 Blob 再显示），不引入 `v-html`。

## 修改文件

- `docs/06_WEBSOCKET_PROTOCOL.md`：新增 §5.1–5.4（payload、校验、错误码、兼容性），更新 §3/§4 文本与 §9 事件表。
- `docs/05_HTTP_API.md`：§6 增加“图片消息与上传的关系”；§7 增加 2211–2214 / 2412–2415 错误码表。
- `agent_logs/prompts/round-7.md`：本轮指令。

## 验收

- 现有 `msg` JSON 列是自由结构，可容纳 `imageMsg`，**无需改 schema/新增表**。
- 上传响应已包含 `id`/`mimeType`/`size`/`url`，满足图片消息所需元数据。
- 协议文档已包含 payload、DTO、错误码、兼容性；下一步实现后端。

## Git

- commit：`docs: define image message protocol`（本记录所在提交）。
