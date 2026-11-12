# GIM Web

GIM 用户端：Vue 3 + TypeScript + Pinia + Vue Router + Axios + Element Plus + Vite。

## 启动 / 构建 / 测试

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm dev          # 本地开发（Vite 代理 /api 到 VITE_DEV_GATEWAY_TARGET，含 WebSocket Upgrade）
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` 保持同源；Chat/Group WebSocket 默认连接当前 origin 并自动把 `http/https` 转成 `ws/wss`，需要跨域时设置 `VITE_WS_BASE_URL`。`.env.local`、token、消息正文与服务凭据不得提交。

## 页面

- `/login`、`/register`：登录、注册。
- `/`：概览。
- `/contacts`：通讯录——搜索用户、发送申请、同意/拒绝、好友列表（在线状态）、备注编辑、删除好友、跳转私聊。
- `/chat/:peerId?`：私聊文本消息；`clientMsgId` + ACK + 超时重试 + 指数退避重连 + 会话/未读重拉。
- `/group/:groupId?`：群聊文本消息（独立 Group WS）。
- `/files`：流式上传、去重文件列表、下载、删除引用。
- `/profile`：只读个人资料。

## 行为边界

- 好友 `isOnline` 来自 `GET /api/user/friends`（Redis Presence），进入页面时刷新；V1 没有 `friend.online/offline` 实时推送。
- pending 消息只在当前页面内保留，不跨刷新持久化。
- 仅文本消息；富消息为 V2。
- 所有用户输入使用文本插值渲染，不使用 `v-html`。

## 测试

- 单元/组件：`pnpm test`（Vitest + happy-dom）。
- 浏览器 smoke（需要运行中的全栈）：`pnpm test:e2e:chat`、`pnpm test:e2e:group`、`pnpm test:e2e:file`；本轮未运行，见 `docs/validation-status.md`。
