# 003 / Task 3 — 前端图片消息

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = e2bdf36`（后端图片消息）。
- **结束状态**：本任务提交 `feat(web): send and render image messages`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `web/src/types/chat.ts` | 新增 `ImageMessagePayload`，`ChatMessagePayload = Text \| Image` |
| `web/src/types/group.ts` | 新增 `GroupImagePayload`，`GroupMessagePayload` 联合类型 |
| `web/src/utils/message.ts` | 新增 `messagePreviewText`、`isImageMessage`、`isValidImagePayload`（WS 不可信字段校验） |
| `web/src/service/ws/chat_socket.ts`、`group_socket.ts` | `isMessage` 接受 type=1/2；type=2 校验 `imageMsg.fileID/width/height/size/url/mimeType` |
| `web/src/api/file.ts` | 新增 `uploadImage`（`POST /api/file/image`，字段 `image`），复用上传进度逻辑 |
| `web/src/stores/chat.ts` | `sendImage(file)`：先上传，再以 type=2 建 pending、复用 clientMsgId/ACK/重试；`uploadingImage` |
| `web/src/stores/group.ts` | 同上（含 `currentUnavailable` 守卫） |
| `web/src/stores/chat_reconcile.ts` | `messagePreview` 改用 `messagePreviewText`（图片→`[图片]`） |
| `web/src/components/chat/MessageBubble.vue` | 新增共享消息组件：文本/图片/未知占位；图片用带 token 的 `fileApi.download` 取 Blob 后显示；点击全屏预览；失败可重试 |
| `web/src/components/chat/ChatMessageList.vue`、`group/GroupMessageList.vue` | 按 `msg.type` 渲染文本或图片（共用 `MessageBubble`） |
| `web/src/components/chat/ChatComposer.vue`、`group/GroupComposer.vue` | 新增“图片”按钮 + 隐藏 `accept="image/*"` 文件输入；上传中禁用发送 |
| `web/src/views/ChatView.vue`、`GroupView.vue` | 绑定 `uploadingImage` 与 `sendImage` |
| `web/src/components/chat/ChatSessionList.vue`、`group/GroupSidebar.vue` | 会话 preview 支持 `[图片]` |
| 测试 | `utils/message.test.ts`、`MessageBubble.test.ts`；chat/group socket 解析、store sendImage、组件测试更新 |

## 行为说明

- **先上传后发送**：上传失败不创建消息、不发送；`lastError` 提示，输入框仍可用。
- **待发送/失败**：与文本一致复用 `clientMsgId`；失败可重试，重试不会重新上传（fileID 已固定）。
- **图片加载**：`<img src>` 不会带 Authorization，因此组件用 `fileApi.download(fileID)` 获取 Blob → `URL.createObjectURL`；卸载时 revoke；不使用 `v-html`。
- **群聊/私聊共用** `MessageBubble`，避免两套渲染逻辑。
- 未知类型显示“暂不支持的消息”占位，不崩溃。

## 测试真实结果

```text
vue-tsc --noEmit  -> PASS
vitest run        -> 21 files / 114 tests PASS（原 19 / 103）
vite build        -> PASS
```

新增覆盖：图片 preview 文案、`isValidImagePayload` 边界、chat/group 图片帧解析与非法字段拒绝、两个 store 的上传后发送、MessageBubble 文本/图片/预览/失败重试/未知类型。

## 问题与修复

- `ChatMessageItem.msg` 变为联合类型后，旧测试直接访问 `textMsg.content` 触发 TS 报错；用 type guard 收窄后通过。
- 图片预览未使用 `v-html`；`<img>` 直接访问受保护下载会 401，改为 Blob 方式。

## 验收

- 前端图片消息：**PASS**（类型检查、单测、构建）。
- 浏览器 E2E（发图片 → 对方看到 → 点击预览）在任务四前单独执行，结果写入 `docs/validation-status.md` 与 day-10 日志。

## Git

- commit：`feat(web): send and render image messages`（本记录所在提交）。
