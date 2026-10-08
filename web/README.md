# GIM Web

Independent Vue 3 user application for GIM. It uses TypeScript, Vite, Pinia, Vue Router, Axios and Element Plus.

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` keeps browser requests same-origin. During local development, Vite proxies `/api` to the public Gateway configured by `VITE_DEV_GATEWAY_TARGET`; production should provide the same path through its edge proxy. Local `.env.local`, tokens, service credentials and signing material must never be committed. Chat, Group and File routes are intentional placeholders in Checkpoint 8; no WebSocket or upload behavior is implemented yet.
