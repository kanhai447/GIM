# GIM Web

Independent Vue 3 user application for GIM. It uses TypeScript, Vite, Pinia, Vue Router, Axios and Element Plus.

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` keeps browser HTTP requests same-origin. Chat WebSocket also defaults to the current origin and converts `http/https` to `ws/wss`; `VITE_WS_BASE_URL` is only needed when the public Gateway WebSocket origin differs. During local development, Vite proxies `/api` to `VITE_DEV_GATEWAY_TARGET`, including WebSocket Upgrade. Local `.env.local`, tokens, message content, service credentials and signing material must never be committed. Private text chat is integrated; Group and File routes remain later checkpoints.
