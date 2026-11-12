# 002 / Task 2 — 后端图片消息实现

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = 1997f58`（协议文档）。
- **结束状态**：本任务提交 `feat(chat): support image messages`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `server/internal/chat/protocol/envelope.go` | `MessagePayload` 增加 `ImageMsg *ImageMessage`；`ImageMessage{fileID,url,width,height,size,mimeType}` |
| `server/internal/group/ws/protocol/envelope.go` | 同上（群聊独立协议） |
| `server/internal/chat/message/repository.go` | 新增 `ImageFile`、`ImageRepository.GetOwnedImage`、`ErrImageFileNotFound` |
| `server/internal/group/message/repository.go` | 同上 |
| `server/internal/chat/message/repository/mysql/repository.go` | `GetOwnedImage`：`user_files JOIN file_objects`，按 `uf.id + uf.user_id` 查询，返回服务端 MIME/size |
| `server/internal/group/message/repository/mysql/repository.go` | 同上 |
| `server/internal/chat/message/service.go` | `ImageMessageType=2`、错误码 `2211–2214`、`MaxImageBytes`、`validate` 分支、`normalizeImage`、`previewOf`、幂等比较含 fileID |
| `server/internal/group/message/service.go` | 同上（错误码 `2412–2415`） |
| `server/internal/chat/config/config.go` | `MaxImageBytes`，读取 `IMAGE_MESSAGE_MAX_BYTES`（默认 10485760，上限 1 GiB） |
| `server/internal/group/config/config.go` | 同上 |
| `server/cmd/chat-api/main.go`、`server/cmd/group-api/main.go` | 注入 `MaxImageBytes` |
| `.env.example`、`.env.compose.example` | 新增 `IMAGE_MESSAGE_MAX_BYTES=10485760` |
| `server/internal/chat/protocol/envelope_test.go` | 图片 payload 解码/严格字段测试 |
| `server/internal/chat/message/service_test.go` | 图片归一化、幂等、冲突、归属/MIME/大小/参数/查询失败测试 |
| `server/internal/group/message/service_test.go` | 群聊图片归一化、幂等、冲突、错误码测试 |
| `server/tests/image_message_integration_test.go` | 真实 MySQL + WebSocket + Redis：图片 ACK、接收、历史、重复 clientMsgId、归属/MIME/大小校验 |

## 实现要点

1. **校验顺序**：结构（type/fileID/尺寸）→ 归属与 MIME/size（DB）→ 事务落库；失败不写 `chat_messages`/`group_messages`，不发成功 ACK。
2. **规范化**：`url="/api/file/{user_files.id}"`，`mimeType/size` 取 `file_objects`；`width/height` 保留客户端值（1..20000）。
3. **归属查询**：`SELECT uf.id, fo.size, fo.mime_type FROM user_files uf JOIN file_objects fo ON fo.id=uf.file_object_id WHERE uf.id=? AND uf.user_id=?`；不存在→`ErrImageFileNotFound`。
4. **幂等**：同一 sender/clientMsgId 只生成一条；比较 type+fileID（文本仍比较 content）。
5. **preview**：图片固定 `[图片]`，会话列表不显示二进制。
6. **无 schema 变更**：复用 `msg` JSON 列与 `user_files`/`file_objects`。

## 测试真实结果

```text
go build ./...                 -> PASS
go vet ./...                   -> PASS
gofmt -l .                     -> 无输出（格式化后）
go test ./... -count=1         -> PASS（69 个包 ok）
```

真实依赖集成（`GIM_ENV_FILE=K:\it\gim\.env.local`，本机 MySQL 3306 + Redis 6379）：

```text
=== RUN   TestChatImageMessageWebSocketHistoryAndValidation
--- PASS: TestChatImageMessageWebSocketHistoryAndValidation (1.13s)
PASS
```

覆盖：WebSocket 发送图片 → ACK(messageId) → 接收方收到 `chat.message`（type=2、规范化 url/mime/size/width/height）→ 历史可查 → 重复 clientMsgId 返回同一 messageId 且 DB 仅 1 行 → 他人文件 `2211`、非图片 `2212`、超 10 MiB `2213` 且均不落库。

群聊图片：单元测试覆盖（真实 MySQL 的群聊 E2E 未单独新增，群聊历史/推送复用同一 DTO 与事务路径）。

## 问题与修复

- 集成测试首轮使用含空格的 clientMsgId（`invalid-file belongs to receiver`），触发格式校验而非预期图片错误；改为 `invalid-image-N` 后 PASS。

## 验收

- 后端图片消息：**PASS**（单测 + 真实 MySQL/Redis/WS 集成）。
- 未实现：视频、语音、普通文件消息、回复/引用/撤回/@/图文（按授权）。

## Git

- commit：`feat(chat): support image messages`（本记录所在提交）。
