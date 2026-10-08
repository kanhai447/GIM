# Day 1 / Checkpoint 8 — Web + Admin 基础工程

- **日期:** 2026-10-08
- **Phase:** Phase 1
- **状态:** COMPLETED
- **目标:** 自主建立 GIM Web 与 Admin 两个独立 Vue 3 SPA，完成最小 Auth/API/Router 基础、稳定页面壳、测试与构建；不进入 Checkpoint 9 或 Day 2。

## 开始状态

- Branch：`main`。
- HEAD：`0055dc1406dfdbabca6b3a2b947ef6a389f97daa`，与 `origin/main` 同步。
- Working tree：clean。
- `web/`、`admin/`：仅有 GIM 空骨架 README，无 package 或业务实现。
- `.env.local`、`reference/`：均未跟踪。

## Web

- 技术栈：Vue 3、TypeScript、Vite、Pinia、Vue Router、Axios、Element Plus、pnpm、Vitest。
- 目录：按职责建立 `api/`、`components/`、`layouts/`、`router/`、`stores/`、`types/`、`utils/`、`views/`，未制造无用途空目录。
- Router：提供 `/login`、`/register`、受保护主布局、首页、资料页及 Chat/Group/File 明确占位路由；精确区分 guest/protected 页面并保存安全 redirect。
- Auth Store：管理 token、当前公开用户、认证状态、登录、注册、注销和 session restore；恢复时通过受保护 User Info 重新验证，而非只信任本地数据。
- API Client：统一处理可配置 base URL、`token` Header、超时、`{code,msg,data}`、401、本地状态清理、网络错误与安全用户提示；组件中没有散落 axios/fetch。
- Login/Register：连接现有 `/api/auth/login` 与 `/api/auth/register`；登录后请求 `/api/user/user_info`，注销使用正确的 `/api/auth/logout`。
- UI：Element Plus 组件按需注册；主布局提供 Chat/Group/File/Profile/Logout 入口，不包含假业务数据。
- 浏览器网络：默认 `VITE_API_BASE_URL=/` 保持同源；本地 Vite 可将 `/api` 代理到 `VITE_DEV_GATEWAY_TARGET`，生产由边缘代理提供同路径。

## Admin

- 技术栈：Vue 3、TypeScript、Vite、Pinia、Vue Router、Axios、Arco Design、ECharts、pnpm、Vitest。
- 目录：独立于 Web，按职责建立 API、布局、路由、store、类型、工具和页面结构。
- Router：提供 `/login`、`/forbidden`、`/admin` 及 Users/Chats/Groups/Files/Settings/Logs 子路由。
- Auth：复用 Gateway Auth Login/User Info/Logout；session restore 重新请求服务端用户信息，不把本地 role 当成服务端授权依据。
- 权限：未登录跳转登录；role 不是管理员时跳转 forbidden；前端 guard 仅限制 UI 导航，目标服务端仍是最终安全边界。
- Layout/页面：Arco 组件按需注册；Admin Layout、Dashboard 欢迎/空状态及六类管理模块均为稳定骨架，不伪装业务已完成。
- ECharts：依赖真实安装并通过按需、动态加载运行测试；只注册折线图所需模块，没有为展示而生成假统计数据。

## Reference

- FIM Web：只读参考登录、注册、主界面及 Chat/Group/File 入口、API 请求语义和双 WebSocket 所在业务范围；本轮没有实现 WebSocket。
- FIM Admin：只读确认 User、private chat、group、file、settings、logs 功能范围，并识别 Dashboard Mock 边界。
- GIM：所有 package 配置、DTO、HTTP Client、stores、guards、layouts、views 和 tests 均按 GIM 文档自主编写；没有复制原组件、store、axios 文件、样式或整套工程。

## Known Source Problems Avoided

- Admin Logout 使用 `/api/auth/logout`，未沿用原项目错误的 `/api/logout`。
- 未调用 `/api/data/statistic`、`/api/data/weather`、`/api/data/login_statistic`，也未创建 `gim_data` 服务。
- Logs 页面只保留结构化文本安全渲染方向，不使用 `v-html`。
- 未复制 FIM 前端代码；未实现 WebSocket、Presence、ACK、文件传输或 Kafka。
- API 错误不向 UI 输出 raw exception、stack、完整 token 或内部网络详情。

## Gateway 联调

- 在临时启动的真实本地 etcd 上执行 `GIM_ENV_FILE=[local] go test ./tests -run TestGatewayAuthUserChain -count=1 -v`：PASS。
- 实际覆盖 Register → Login → JWT → Gateway → Auth authentication → User Info → Logout → 注销后受保护请求拒绝；测试注册 key 已清理，临时 etcd 已停止。
- Web/Admin contract tests 同时验证真实后端约定的精确路径与 `token` Header。
- 当前仓库仅有 Gateway 与 migration 正式命令，没有独立 Auth/User 可执行入口，因此没有伪称完成浏览器到多进程服务的手工联调；现有真实 Gateway 集成链路和前端 API 契约均已通过。

## Tests

### Web

- `pnpm install`：PASS；仅允许 pnpm 11 构建必需的 `esbuild`，lockfile 可重复安装。
- `pnpm type-check`：PASS。
- `pnpm test`：PASS，4 files / 9 tests（Auth Store、API 解析、API contract、Router Guard）。
- `pnpm build`：PASS，Vite 7.3.7 production build；Element Plus 按需注册后无超大 chunk 警告。
- lint：NOT CONFIGURED，本轮没有无依据新增 lint 工具链。

### Admin

- `pnpm install`：PASS；仅允许 pnpm 11 构建必需的 `esbuild`，lockfile 可重复安装。
- `pnpm type-check`：PASS。
- `pnpm test`：PASS，5 files / 9 tests（Auth Store、API 解析/contract、role guard、ECharts runtime）。
- `pnpm build`：PASS，Vite 7.3.7 production build；Arco 按需注册、ECharts 动态 chunk 均低于默认 500 kB 警告阈值。
- lint：NOT CONFIGURED。

## Security

- `web/.env.example`、`admin/.env.example` 仅含同源 base URL、超时和本地 Gateway 占位地址；无 token、口令、JWT signing material 或内部生产地址。
- `.env.local`、`dist/`、`node_modules/` 均由仓库规则忽略；两份 `pnpm-lock.yaml` 已跟踪。
- 前端不接收或引用 JWT signing material，不打印 token，不硬编码管理员口令或测试 JWT。
- Secret Scan、禁用路径/实现扫描与 `git diff --check`：PASS。

## Git

- Web commit：`19512be8afe74cd03cb457b0b3a96770f4ac14e0 feat(web): add user frontend foundation`。
- Admin commit：`59490fae4e4590e3a91d49a26234c30564065267 feat(admin): add management frontend foundation`。
- Web/Admin push：SUCCESS — `origin/main`。
- 状态/日志 commit：本操作日志所在提交；完成后推送 `origin/main`。

## 停止点

- Checkpoint 8 已完成；本轮立即停止，不进入 Checkpoint 9，不进入 Day 2。
