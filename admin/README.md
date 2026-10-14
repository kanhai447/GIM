# GIM Admin

Independent Vue 3 administration application for GIM. It uses TypeScript, Vite, Pinia, Vue Router, Axios, Arco Design and an on-demand ECharts runtime.

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` keeps browser requests same-origin. During local development, Vite proxies `/api` to the public Gateway configured by `VITE_DEV_GATEWAY_TARGET`; production should provide the same path through its edge proxy.

The Admin app integrates the real User, private Chat, Group, File, Settings, and Logs APIs. It supports bounded cursor pagination, filters, user restrictions/disable, message tombstone removal, group dissolution, safe file download/reference deletion, public business settings, and structured log detail. Frontend role checks improve navigation but are never the authorization boundary: Gateway/Auth inject identity and every target service validates `Role == 1` again.

Dashboard metrics remain intentionally empty because `/api/data/*` exists only in the reference Admin mock. Log and message JSON is rendered through Vue text interpolation/`<pre>`; this app does not use `v-html`. Deployment secrets never enter Settings.
