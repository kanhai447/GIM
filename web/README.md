# GIM Web

Independent Vue 3 user application for GIM. It uses TypeScript, Vite, Pinia, Vue Router, Axios and Element Plus.

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` keeps browser HTTP requests same-origin. Chat WebSocket also defaults to the current origin and converts `http/https` to `ws/wss`; `VITE_WS_BASE_URL` is only needed when the public Gateway WebSocket origin differs. During local development, Vite proxies `/api` to `VITE_DEV_GATEWAY_TARGET`, including WebSocket Upgrade. Local `.env.local`, tokens, message content, service credentials and signing material must never be committed.

## Pages

- `/login`, `/register`: account login and registration.
- `/` — overview.
- `/contacts` — friend loop: search users, send requests, accept/reject received requests, edit a directional remark, delete a friend, open a private chat. Online state comes from `GET /api/user/friends` (`isOnline`, Redis Presence) and is refreshed when the page loads; no friend Presence WebSocket event exists in V1.
- `/chat/:peerId?` — private text chat over the Chat WebSocket with `clientMsgId` ACK, retry and reconnect.
- `/group/:groupId?` — group text chat over the Group WebSocket.
- `/files` — streaming upload, deduplicated user file list, download and reference deletion.
- `/profile` — read-only profile.

All friend UI renders user input through Vue text interpolation and `<pre>`; it never uses `v-html`. Recipient-side delivery/read receipts and rich message types remain V2.
