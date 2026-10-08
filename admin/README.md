# GIM Admin

Independent Vue 3 administration application for GIM. It uses TypeScript, Vite, Pinia, Vue Router, Axios, Arco Design and an on-demand ECharts runtime.

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` keeps browser requests same-origin. During local development, Vite proxies `/api` to the public Gateway configured by `VITE_DEV_GATEWAY_TARGET`; production should provide the same path through its edge proxy. Admin reuses `/api/auth/login` and `/api/auth/logout`; frontend role checks improve navigation but are never the authorization boundary. Dashboard metrics are intentionally empty until real server APIs exist. No Mock `/api/data/*`, unsafe `v-html`, WebSocket, upload or Kafka behavior is included in Checkpoint 8.
