# 003 / Task 2 — 文件授权专项测试（文本路径 + 前端不触发下载）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 20398fd`。
- **结果**：文本消息不授权 + 前端文本不触发下载，**PASS**。

## 新增/修改测试

| 文件 | 覆盖 |
|---|---|
| `server/tests/file_text_authorization_integration_test.go` | 文本消息内容即使包含 `fileID`，接收方仍 `2503`；所有者可下载（所有者语义不受消息类型影响） |
| `web/src/components/chat/MessageBubble.test.ts` | type=1 文本渲染时 `fileApi.download` **未被调用**；图片 type=2 才触发下载 |

## 真实结果

```text
=== RUN   TestTextMessageDoesNotGrantImageDownload
--- PASS: TestTextMessageDoesNotGrantImageDownload (1.15s)
```

```text
vitest run src/components/chat/MessageBubble.test.ts -> PASS
```

## 说明

- 文本消息不产生文件下载调用，因此不会进入文件服务授权查询；后端 SQL 的 `$.imageMsg.fileID` 在文本 JSON 上为 NULL，双重保证不授权。
- 授权专项测试矩阵（所有者/私聊接收方/群成员/非参与者/tombstone/退群/未知 fileID）已在 002 提交的 `TestImageDownloadAuthorizationSemantics` 中覆盖。

## Git

- commit：`test(file): cover download authorization semantics`（本记录所在提交）。
