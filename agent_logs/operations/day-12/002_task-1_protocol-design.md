# 002 / Task 1 — 普通文件消息协议设计（文档先行）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = c97618f` + 本地 tag `gim-v1.2`。
- **结果**：协议文档完成，未改代码。

## 参考的现有实现

| 文件 | 关键结论 |
|---|---|
| `chat/protocol/envelope.go`、`group/ws/protocol/envelope.go` | `MessagePayload` 已有 Type/TextMsg/ImageMsg；新增 `FileMsg` 字段即可 |
| `chat/message/service.go`、`group/message/service.go` | 图片走 `validate → normalizeImage → PersistWithSessions`；文件消息复用同一流程 |
| `file/domain`、`file/repository/mysql` | `user_files.original_name` + `file_objects.size/mime_type` 可提供全部元数据；无需新表 |
| `file/repository/mysql` 授权查询 | 按 `imageMsg.fileID` 匹配；文件消息需同时匹配 `fileMsg.fileID` |
| `web/src/components/chat/MessageBubble.vue` | type=2 用 Blob 下载 + 预览；type=3 用文件名/大小/下载按钮 |

## 设计决策

1. **`msg.type=3` + `fileMsg`**：`fileID` 必填，`url/fileName/size/mimeType` 由服务端回填；客户端提交值忽略。
2. **校验**：归属 → 不复用 `image/*` 限制（普通文件允许任意 MIME）→ `size <= FILE_MESSAGE_MAX_BYTES`（默认 25 MiB）→ 参数；失败不落库、不 ACK。
3. **错误码**：Chat `2215` 文件不存在/非本人、`2216` 超限、`2217` 参数非法；Group `2416–2418`；未知类型保持 `2201/2403`；幂等冲突 `2205/2407`。
4. **幂等**：同 sender/clientMsgId 只生成一条；比较 type + `fileMsg.fileID`。
5. **授权**：复用图片语义（所有者/私聊参与者/群成员；tombstone/退群/非参与者 `2503`）；授权 SQL 同时匹配 `imageMsg.fileID` 与 `fileMsg.fileID`。
6. **兼容性**：type=1/2 不变；旧客户端 type=3 显示占位；旧服务端返回 `2201/2403`；不新增表/列。
7. **配置**：`FILE_MESSAGE_MAX_BYTES` 默认 `26214400`（25 MiB），写入两个 env 模板与文档。
8. **前端**：Composer 增加“文件”入口；上传用现有 `POST /api/file/file`；消息卡片显示文件名/大小/下载按钮，下载走带 token 的 `fileApi.download`，不裸 `<a href>`。

## 修改文件

- `docs/06_WEBSOCKET_PROTOCOL.md`：类型表新增 type=3；新增 §5.5 普通文件消息（payload、校验、错误码、授权、兼容性）。
- `docs/05_HTTP_API.md`：文件上传与 type=3 引用关系；`2215–2217` / `2416–2418` 错误码；下载授权说明覆盖文件消息。

## 验收

- 现有 `msg` JSON 与 `user_files` 元数据足够，**无需 schema/表/索引变更**。
- 不限制 MIME 前缀，与图片消息的 `image/*` 校验明确区分。
- 下一步实现后端。

## Git

- commit：`docs: define file message protocol`（本记录所在提交）。
