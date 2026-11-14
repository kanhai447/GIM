# 004 / Task 3 — 前端普通文件消息

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 2f13134`（后端文件消息）。
- **结束状态**：本任务提交 `feat(web): send and render file messages`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `web/src/types/chat.ts`、`types/group.ts` | 新增 `FileMessagePayload`/`GroupFilePayload`，payload 联合类型加入 type=3 |
| `web/src/utils/message.ts` | `messagePreviewText` 支持 `[文件]`；新增 `isFileMessage`、`isValidFilePayload`、`formatFileSize` |
| `web/src/service/ws/chat_socket.ts`、`group_socket.ts` | type=3 解析与 `fileMsg` 严格字段校验 |
| `web/src/stores/chat.ts`、`group.ts` | 新增 `sendFile(file)`：先上传（`POST /api/file/file`），再以 type=3 建 pending、复用 clientMsgId/ACK/重试；`uploadingFile` |
| `web/src/components/chat/MessageBubble.vue` | 文件卡片：文件名、格式化大小、下载按钮；下载走带 token 的 `fileApi.download` → Blob → `<a download>`；下载中/失败重试 |
| `web/src/components/chat/ChatComposer.vue`、`group/GroupComposer.vue` | 新增“文件”按钮与隐藏文件输入；上传中禁用/提示 |
| `web/src/views/ChatView.vue`、`GroupView.vue` | 绑定 `uploadingFile` 与 `sendFile` |
| 测试 | utils、两个 socket、两个 store、`MessageBubble` 文件卡片/下载/失败重试 |

## 行为

- 先上传后发送；上传失败不创建消息、不发送，`lastError` 提示。
- 文件卡片显示 `fileName` 与 `formatFileSize`；未返回文件名时显示 `文件 {id}`。
- 下载不使用裸 `<a href>`，而是带 token 获取 Blob 后触发下载；失败显示“重试下载”。
- 私聊/群聊共用 `MessageBubble`。
- 会话 preview 显示 `[文件]`。

## 测试真实结果

```text
vue-tsc --noEmit  -> PASS
vitest run        -> 21 files / 122 tests PASS（原 21 / 114）
vite build        -> PASS
```

新增覆盖：`[文件]` preview、`isValidFilePayload` 边界、`formatFileSize`、chat/group type=3 帧解析与非法字段拒绝、两个 store 的上传后发送、文件卡片渲染与下载/失败重试。

## 问题与修复

- 无阻塞问题；type=3 复用现有 pending/ACK 流程，未改 reconcile 逻辑。

## 验收

- 前端普通文件消息：**PASS**（类型检查、单测、构建）。
- 浏览器 E2E（发文件 → 接收方下载）在任务四前单独执行。

## Git

- commit：`feat(web): send and render file messages`（本记录所在提交）。
