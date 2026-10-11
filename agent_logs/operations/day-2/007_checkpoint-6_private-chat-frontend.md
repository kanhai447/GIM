# Day 2 / Checkpoint 6 — Private Chat Frontend Integration

- **时间:** 2026-10-11 18:12 +08:00
- **开始状态:** `main`，HEAD/origin/main 均为 `6caeaa586f2098e2c6a0967de7c31de8550ffb6a`，working tree clean。
- **范围:** 仅实现用户端私聊 text 消息的 HTTP/WS 集成、可靠发送状态、session/unread/read/history UI 与测试；未进入 Group、File、Presence UI、Admin 或 Day 3。

## Architecture

- `web/src/api/chat.ts` 集中封装 session、message-ID cursor history、mark read 与 top HTTP API。
- `web/src/service/ws/chat_socket.ts` 是无 DOM 的单例连接对象，负责 URL、连接/关闭、typed frame 解析、发送、认证失败和有界指数退避。
- Pinia `Chat Store` 管理 session、current peer、per-peer messages/history、pending/ACK/error、unread/read 与 session reconcile；协议合并/去重拆入 `chat_reconcile.ts`，页面不直接操作 WebSocket。
- `/chat/:peerId?` 为私聊入口；左侧展示后端排序的 session/preview/time/unread/top，右侧展示 cursor history、加载更早、composer 和 pending/sent/failed 状态。

## Connection / Auth Lifecycle

- 登录成功或 session restore 后由 `App.vue` 启动一条 Chat Socket；同用户重复 start 不创建第二条连接，无 token 不连接。
- WS 路径固定为 `/api/chat/ws/chat?token=JWT`；默认从当前 HTTP origin 转换 `http/https -> ws/wss`，可由 `VITE_WS_BASE_URL` 或 `VITE_API_BASE_URL` 配置，Vite `/api` proxy 显式开启 Upgrade。
- 状态为 `disconnected / connecting / connected / reconnecting`；重连采用 1/2/4/8/16/30 秒上限和最多 20% jitter，任何时刻只有一个 reconnect timer。
- logout 先 `chat.stop()`，关闭 socket、取消 reconnect/ACK timer、清空内存状态，再调用 Auth logout 并清理身份。
- WebSocket policy/auth close code，或 Chat HTTP 的 401 / `1203..1206`，都会停止重连、清空 Chat/Auth 状态并回登录页；错误提示不暴露 URL token、内部异常或敏感 Header。

## Reliable Send / ACK

- 每次逻辑发送生成一个 UUID `clientMsgId`，先插入本地 `pending`；断线时保留内存队列，重连后复用相同 ID 重发。
- ACK 将对应本地消息更新为 `sent`、写入 `messageId/createdAt`；重复 ACK 不新增 UI 消息。ACK 只解释为服务端 message/session transaction committed，不表示 delivered/read。
- ACK 10 秒超时或带 `clientMsgId` 的 typed error 只把对应消息置为 `failed`；手动重试复用同一 ID，socket 和其他 pending 不受影响。
- pending 不持久化到 local/session storage；页面刷新后由 MySQL history 恢复已持久化消息，未获 ACK 的纯内存 pending 可能丢失，作为 V1 明确限制。

## Receive / Session / Read / History

- `chat.message` 以 server `messageId` 去重；sender 自己的 HTTP/history copy 还可按 `clientMsgId` 与本地 pending 合并，避免 HTTP/WS/pending 三路重复。
- 当前 peer 消息实时追加；其他 peer 先本地更新 preview/unread，再通过单一 250ms debounce refresh 与服务器 session 排序/计数对账，避免每帧无限 full refresh。
- 打开会话先取 latest history，再用最新 server message ID mark read；只有当前路由且页面 `visible + focused` 时，实时消息自动 mark read。read 状态是 MySQL 用户级状态，不产生 peer receipt。
- history 只使用 `beforeMessageID`，oldest→newest 合并，load older 不使用 offset，也不会重复插入跨页边界消息。
- 多设备遵循服务端语义：receiver 所有在线 Client 实时收到；read 为用户级数据库状态，其他设备通过 session/history refresh 对账；sender ACK 只属于 origin Client。

## UI / Security

- Element Plus + Vue safe interpolation；没有 `v-html`，本阶段只渲染 text message。
- 没有把 Chat history、正文、pending 或 token 写入新增 storage；没有业务 `console.log` 或 token/error URL 输出。
- top 状态展示并支持显式 set；Presence UI 未实现，避免把 session 存在误判为在线。

## Reference / Independent Implementation

- 只读参考 FIM 的 session/sidebar、history、top、文本 composer、登录后连接 `/api/chat/ws/chat?token=...` 等用户行为。
- 未复制 FIM 文件或实现。GIM 自主增加 typed envelope、独立 Socket/Store/API 边界、single socket、reconnect backoff、pending/ACK/failed、messageId/clientMsgId 双重 reconcile、unread/read cursor 和安全文本渲染。
- 没有保留 FIM 的直接 `WebSocket.send` 业务调用、缺少 reconnect/ACK 的全局 Store socket 或 HTML 图文输入模式。

## Tests

- `pnpm type-check`: PASS。
- `pnpm test`: PASS，8 files / 35 tests。覆盖 Socket connect/backoff/logout/auth failure/typed parsing、stable clientMsgId、ACK mapping/duplicate ACK、error/timeout/retry、reconnect retry、message dedupe、session unread、mark read、history cursor merge，以及 pending/sent/failed/unread/load-more 组件状态。
- `pnpm build`: PASS，Vite production build 1719 modules。
- lint: NOT CONFIGURED（`web/package.json` 无 lint script）。
- `go test ./... -count=1`: PASS；无后端业务代码改动。
- `go vet ./...`: PASS。
- `git diff --check`: PASS。
- Repository Secret Scan: PASS。
- Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；本轮未重复执行已知必然失败的 Windows cgo race，兼容环境/CI verification 仍 PENDING。

## Real Browser / Integration

- 本地 MySQL、Redis、临时 etcd、User RPC、Auth/User/Chat API、Gateway、Vite 与本机 Chrome headless 两个隔离 context 真实联调。
- 两用户 login、A→B text：A DOM 状态序列观察到 `pending -> sent`，B realtime 收到且未打开时 unread=1；B 打开后 history 可见、read 归零，刷新后消息与 read 状态恢复：PASS。
- Receiver 同账号第二浏览器页同时收到 realtime message：PASS。
- 发送超过 20 条后刷新只取 latest 20，点击加载更早后合并为 >20 且无重复：PASS。
- 短暂停止并重启本轮 Chat API，浏览器 MutationObserver 记录 `reconnecting`，随后回到 `connected`，再次发送/接收：PASS。
- Windows Computer Use helper 因本机 `failed to write kernel assets: path not found` 两次初始化失败；改用 Playwright Core 调用预装 Chrome，不下载浏览器。该工具问题不影响真实浏览器结果。
- 本轮两个临时用户（精确 ID/account 校验后）、好友关系、session/messages、临时 etcd/bin/log 和全部临时服务 PID 已清理；相关端口最终监听数 0。

## Problems / Fixes

1. 首次 reconnect smoke 使用 Chromium offline 模式，既有 WebSocket 未立即 close；改为重启本轮临时 Chat API，验证真实 onclose/backoff/reconnect。
2. 可见状态轮询可能错过短暂 reconnect transition；改为重启前安装 DOM MutationObserver，断言完整状态序列。
3. 重复执行 E2E 时固定 pagination 文本造成 strict locator 多匹配；为每次 run 增加唯一 ID。

## Database / Protocol Changes

- Database migration：无；只在 smoke 临时写入并已清理测试数据。
- HTTP/WS 服务端协议：无变化；前端按既有 CP4/CP5 typed contract 接入。
- Frontend route：`/chat/:peerId?`；新增可配置 `VITE_WS_BASE_URL`，缺省同源。
- Redis：无 key/helper 变化；Presence UI 未实现。

## Main Files

- `web/src/types/chat.ts`
- `web/src/api/chat.ts`
- `web/src/service/ws/chat_socket.ts`
- `web/src/stores/chat.ts`、`chat_reconcile.ts`
- `web/src/components/chat/*`
- `web/src/views/ChatView.vue`
- `web/e2e/private-chat-smoke.mjs`
- `web/src/**/*chat*.test.ts`、`web/src/components/chat/chat_components.test.ts`
- `web/src/App.vue`、`web/src/layouts/MainLayout.vue`、`web/src/router/index.ts`
- `web/{package.json,pnpm-lock.yaml,vite.config.ts,.env.example,README.md}`

## Git / Next

- implementation commit: `80456b3e27e2ebc0dc56d03f17c1ee58cf0c6f6d` — `feat(web): integrate reliable private chat`。
- log/status commit: 本 operation/CURRENT_STATE/day-2/worklog 所在提交 — `docs(day2): record private chat frontend integration`。
- Private Chat V1: FUNCTIONAL。
- Day 2 blocker: NONE；Race 兼容环境/CI 补验仍是验收缺口，但不阻塞当前功能状态。
- 下一计划：Day 2 Final Private Chat Acceptance；本轮不进入 Group、File、Presence UI、Admin 或 Day 3。
