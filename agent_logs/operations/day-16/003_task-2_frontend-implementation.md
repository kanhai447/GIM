# 003 / Task 2 — 前端好友上线提醒

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = eace570`。
- **结果**：前端接收/更新 + 单测 + 浏览器 E2E **PASS**。

## 修改文件

| 文件 | 变更 |
|---|---|
| `web/src/types/chat.ts` | `FriendPresenceData`、`FriendOnlineEnvelope`/`FriendOfflineEnvelope`，加入 `ChatServerEnvelope` |
| `web/src/service/ws/chat_socket.ts` | 解析 `friend.online`/`friend.offline`，校验 `userID > 0`、`at` 字符串 |
| `web/src/stores/chat.ts` | `handleEnvelope` 分支调用 friend store；Group WS 不处理 |
| `web/src/stores/friend.ts` | `applyPresence(userID, online)`：只更新已存在好友；未知 ID 忽略 |
| 测试 | socket 解析、friend store applyPresence、chat store 事件更新 |

## 行为

- B 的 Chat WS 收到 `friend.online`/`friend.offline` 后按 `userID` 更新 `friends[].isOnline`；通讯录/好友列表即时刷新。
- 未知 userID（本地列表未刷新）忽略，不报错。
- HTTP `GET /api/user/friends` 仍是恢复路径；不新增轮询。
- Group WS 解析器不识别/不处理好友事件。

## 真实测试结果

```text
vue-tsc --noEmit     PASS
vitest run           21 files / 135 tests PASS
vite build           PASS（Compose 镜像构建时验证）
```

E2E（Chrome + Compose）：

```text
STEP 5 A/B are friends (A=300, B=301)
STEP 6 B sees A online while A is logged in
STEP 7 B sees A offline after A closes the browser
STEP 8 B sees A online again after A re-login
E2E RESULT=PASS
```

## 限制

- 多实例全局 Presence 未实现（V2）；本轮单机。
- 事件 best-effort；丢失时 HTTP 恢复。

## Git

- commit：`feat(web): apply friend online events`（本记录所在提交）。
