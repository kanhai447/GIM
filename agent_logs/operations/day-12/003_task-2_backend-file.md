# 003 / Task 2 — 后端普通文件消息实现

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 70f50cb`（协议文档）。
- **结束状态**：本任务提交 `feat(chat): support file messages`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `chat/protocol/envelope.go`、`group/ws/protocol/envelope.go` | `MessagePayload` 增加 `FileMsg *FileMessage`；`FileMessage{fileID,url,fileName,size,mimeType}` |
| `chat/message/repository.go`、`group/message/repository.go` | 新增 `FileInfo`、`FileRepository.GetOwnedFile`、`ErrFileNotFound` |
| 两个 `repository/mysql/repository.go` | `GetOwnedFile`：`user_files JOIN file_objects`，返回 `original_name/size/mime_type` |
| 两个 `message/service.go` | `FileMessageType=3`；`2215–2217` / `2416–2418`；`MaxFileBytes`；`validateFilePayload`、`normalizeFile`、`previewOf("[文件]")`；幂等比较含 `fileMsg.fileID` |
| `chat/config/config.go`、`group/config/config.go` | 读取 `FILE_MESSAGE_MAX_BYTES`（默认 26214400，上限 1 GiB） |
| `cmd/chat-api/main.go`、`cmd/group-api/main.go` | 注入 `MaxFileBytes` |
| `.env.example`、`.env.compose.example` | `FILE_MESSAGE_MAX_BYTES=26214400` |
| `file/repository/mysql/repository.go` | 授权查询同时匹配 `$.imageMsg.fileID` 与 `$.fileMsg.fileID`（`LIMIT 1` 不变） |
| 测试 | chat/group service、chat protocol、`server/tests/file_message_integration_test.go`、授权测试增加群文件消息 |

## 行为

1. `type=3` 必须携带 `fileMsg.fileID`；`size<0` 拒绝。
2. 归属：`user_files.user_id = senderID`；不存在/非本人 `2215`/`2416`，不区分。
3. **不限制 MIME 前缀**；`file_objects.size <= FILE_MESSAGE_MAX_BYTES`，否则 `2216`/`2417`。
4. 服务端回填 `url/fileName/size/mimeType`；`preview="[文件]"`。
5. 与文本/图片同一事务、ACK、推送、历史；幂等比较 `type+fileID`。
6. 下载授权复用图片语义：授权 SQL 现在同时匹配 `imageMsg.fileID` 与 `fileMsg.fileID`。

## 测试真实结果

```text
go build ./...                 PASS
go vet ./...                   PASS
gofmt -l .                     无输出
go test ./... -count=1         PASS（69 packages）
```

真实 MySQL/Redis/WS：

```text
=== RUN   TestImageDownloadAuthorizationSemantics
--- PASS: TestImageDownloadAuthorizationSemantics (1.16s)
=== RUN   TestChatFileMessageWebSocketHistoryAndValidation
--- PASS: TestChatFileMessageWebSocketHistoryAndValidation (1.03s)
```

覆盖：WS `type=3` → ACK → 接收方 `chat.message`（fileName=report.pdf、size、mime 规范化）→ 历史 → 重复 clientMsgId 仅 1 行 → 他人文件 `2215`、26 MiB `2216` 且不落库 → 接收方可解析文件、非参与者不可；群文件消息的授权（成员/非成员/tombstone）在授权测试中覆盖。

## 问题与修复

- 无阻塞问题。授权查询必须同时匹配 `fileMsg.fileID`，否则接收方无法下载文件消息附件；已在同一提交修复并测试。

## 验收

- 后端普通文件消息：**PASS**（单测 + 真实 MySQL/Redis/WS 集成）。
- 无 schema/索引/migration 变更。

## Git

- commit：`feat(chat): support file messages`（本记录所在提交）。
