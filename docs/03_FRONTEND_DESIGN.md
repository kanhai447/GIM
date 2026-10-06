# 03 用户端 Web 详细设计

用户主要负责后端，用户端由 Agent 以“最少重构、稳定联调”为目标处理。

## 1. 原源码事实

原 `fim_web` 使用 Vue3 + TypeScript + Pinia + Vue Router + Axios + Element Plus + Vite。登录成功后在 Store 中同时创建两条连接：

- `/api/chat/ws/chat?token=...`
- `/api/group/ws/chat?token=...`

`send_msg.ts` 根据 `user/group` 直接调用对应 `WebSocket.send()`。当前没有完整的 `onclose` 自动重连、pending ACK 或发送状态管理。

## 2. 保留

保留现有页面、消息组件、好友/群管理交互和消息类型。HTTP API 继续按服务拆分。

## 3. WebSocket 重构

建议：

```text
src/service/ws/
  base_socket.ts
  chat_socket.ts
  group_socket.ts
```

`BaseSocket` 负责连接、关闭、重连退避、事件分发、pending ACK、主动 logout 禁止重连。

状态：`idle | connecting | open | reconnecting | closed`。

## 4. pending 消息

发送前生成 `clientMsgId`：

```text
sending -> sent(ACK)
        -> failed(timeout/明确错误)
```

ACK 超时重试必须复用同一个 `clientMsgId`。

## 5. 重连与状态恢复

非主动 logout 的 onclose 指数退避（例如 1/2/4/8 秒，上限 30 秒）。重连成功后重新拉 session/unread，不能假设断线期间没有消息。

## 6. Presence

好友在线状态以 Chat WS/Redis Presence 为权威。Group WS 的断开不能把用户标记离线。
