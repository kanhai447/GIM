# 003 / Task 2 — 前端撤回消息

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 16dc592`（后端 + 授权）。
- **结束状态**：本任务提交 `feat(web): recall messages`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `web/src/types/chat.ts`、`types/group.ts` | 新增 `WithdrawMessagePayload`/`GroupWithdrawPayload`，payload 联合加入 type=4 |
| `web/src/utils/message.ts` | `[撤回]` preview、`isWithdrawMessage`、`isValidWithdrawPayload`、`RECALL_WINDOW_MS`、`canRecallMessage` |
| `web/src/service/ws/chat_socket.ts`、`group_socket.ts` | type=4 帧解析与 `withdrawMsg.msgID` 校验 |
| `web/src/stores/chat_reconcile.ts`、`group_reconcile.ts` | `applyWithdrawals`/`applyGroupWithdrawals`：原消息替换为占位，合并时隐藏单独撤回行 |
| `web/src/stores/chat.ts`、`group.ts` | `recallMessage(messageId)`、`recalling`；ACK/错误/推送处理；历史合并后归一化 |
| `web/src/components/chat/MessageBubble.vue` | type=4 渲染“消息已撤回”占位 |
| `web/src/components/chat/ChatMessageList.vue`、`group/GroupMessageList.vue` | 自己发送、已发送且 120s 内的文本/图片/文件显示“撤回”；撤回中禁用 |
| `web/src/views/ChatView.vue`、`GroupView.vue` | 绑定 `recalling` 与 `recallMessage` |
| 测试 | utils（preview/eligibility/校验）、socket type=4、两个 store、MessageBubble 占位、列表入口显示/点击 |

## 行为

- 撤回入口仅前端 UX：服务端仍校验窗口/本人/已撤回等。
- 点击撤回发送 type=4，复用 clientMsgId；ACK 或推送到达后按 `msgID` 替换本地原消息为占位，不保留原文/图片/文件卡片。
- 服务端推送的撤回消息若目标在本页，合并后只显示一条占位（隐藏单独撤回行）；目标不在本页时显示为占位消息。
- 失败显示 `lastError`，原消息保持不变；超时提示可重试。
- 历史加载时服务端已替换占位，前端直接渲染。

## 测试真实结果

```text
vue-tsc --noEmit  -> PASS
vitest run        -> 21 files / 131 tests PASS（原 21 / 122）
vite build        -> PASS
```

新增覆盖：撤回入口窗口/本人判断、type=4 帧解析、chat/group 撤回发送与 ACK 替换、失败保留原文、推送合并、占位渲染。

## 问题与修复

- E2E 首轮定位器使用了被替换掉的原文，导致假失败；截图确认已撤回后，改用稳定定位器（`.withdrawn` 计数）重跑 PASS。非产品缺陷。

## 验收

- 前端撤回：**PASS**（类型检查、单测、构建、浏览器 E2E）。

## Git

- commit：`feat(web): recall messages`（本记录所在提交）。
