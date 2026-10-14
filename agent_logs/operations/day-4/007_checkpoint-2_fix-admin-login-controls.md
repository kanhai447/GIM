# Day 4 / Checkpoint 2 — Fix Admin Login Controls

## 时间

- 2026-10-14 17:01 +08:00

## 目标

- 修复真实 Manual Browser E2E 中 Admin Login 卡片只显示标题/说明、未显示账号输入框、密码输入框和登录按钮的问题。
- 仅处理当前 blocker，不进入其他 Admin E2E，不完成 Checkpoint，不提交或推送。

## Root Cause

- `LoginView.vue` 的账号、密码和提交按钮真实存在于 template；没有 `v-if/v-show/loading` 条件隐藏，也没有 CSS `display:none`、透明度、零高度或溢出裁剪。
- Arco CSS 已由 `main.ts` 正常导入。
- 问题在全局组件注册：代码使用 Arco 组件运行时 `name`（如 `Form/Input/Button`）注册，但 Vue 模板中的 `<a-form>/<a-input>/<a-button>` 会解析为 `AForm/AInput/AButton`。
- 因此全部 `a-*` 组件未解析。卡片中的标题和说明是原生 `h2/p`，即使 Arco 子树未实例化仍可显示；真正的 Form/Input/Button 没有渲染为可交互控件。
- Vite 服务日志无编译异常；这是浏览器运行时组件解析错误，不是 Router、Auth init 或 Vite 启动失败。

## 修改

- `admin/src/ui/arco.ts`
  - 集中维护 Admin 使用的 Arco 组件。
  - 使用模板需要的 `A${component.name}` 注册名，例如 `AForm/AInput/AButton`。
- `admin/src/main.ts`
  - 改为调用统一的 `registerArcoComponents`，移除错误的无前缀注册循环。
- `admin/src/views/loginFlow.ts`
  - 提取可测试的登录提交流程。
  - 保留输入校验、管理员角色判断、安全错误提示和 `/admin` redirect。
  - loading 状态为 true 时直接拒绝重复提交。
- `admin/src/views/LoginView.vue`
  - 保持现有视觉/template；接入可测试 login flow。
  - Arco Form 的 submit-success 仍支持 Enter 和提交按钮。
- `admin/src/ui/arco.test.ts`
  - 断言实际注册包含 `ACard/AForm/AFormItem/AInput/AInputPassword/AButton`，防止再次注册成无前缀名称。
- `admin/src/views/LoginView.test.ts`
  - 使用真实 LoginView、Vue Router/Pinia、实际 Arco 组件做 SSR 渲染，断言最终 HTML 中存在账号 input、password input 和 submit button，而非仅 shallow mount 外壳。
  - 覆盖输入账号密码后调用 Auth login、成功 redirect、失败保留输入并显示安全错误、loading 防重复提交。

## 验证

- `pnpm type-check` → PASS。
- `pnpm test` → PASS：8 test files，15 tests。
- `pnpm build` → PASS：1347 modules transformed；仅保留 Vite 非阻塞 chunk-size warning。
- 安全扫描：无 `v-html`、无 password logging、无硬编码本地 E2E 账号标识。
- Admin dev server `http://127.0.0.1:4174/login?redirect=/admin` → HTTP PASS。
- Admin Vite `/api` proxy → PASS。
- 已启动的后端 E2E 服务进程保持运行。

## 安全

- 未记录或提交本地 E2E 临时密码、密码哈希、JWT、数据库凭据或其他 Secret。
- 登录失败仅展示 API 已清洗的公开错误；未知异常使用固定通用提示。
- 未新增账号/密码 console logging。

## Git

- commit：无。
- push：无。
- Checkpoint 2：仍为 IN PROGRESS。

## 下一步

- 等待用户本人按 F5 刷新 Edge 并重新确认 Login 页面控件与真实登录。
- 未授权前不继续其他 Admin E2E、不调用 Browser Computer Use、不进入下一 Checkpoint。
