# GIM Admin

GIM 管理端：独立 Vue 3 应用，TypeScript + Vite + Pinia + Vue Router + Axios + Arco Design + ECharts（按需）。

## 启动 / 构建 / 测试

```powershell
Copy-Item .env.example .env.local
pnpm install
pnpm dev          # 本地开发；Vite 代理 /api 到 VITE_DEV_GATEWAY_TARGET
pnpm type-check
pnpm test
pnpm build
```

`VITE_API_BASE_URL=/` 保持同源；生产由边缘代理提供同源 `/api`。`.env.local` 与凭据不得提交。

## 页面

- `/login`：管理员登录（普通用户 token 无法进入）。
- `/admin`：概览——真实只读聚合 `GET /api/settings/admin/overview`（用户/私聊/群/文件/审计计数），并展示真实 IM 指标；不调用 `/api/data/*` Mock。
- `/admin/users`：用户列表、搜索、在线状态、消息/群统计、四项限制、禁用/启用。
- `/admin/chats`：私聊记录筛选与 tombstone 删除。
- `/admin/groups`：群列表、解散、群消息查看与删除。
- `/admin/files`：文件列表、管理员鉴权下载、删除引用。
- `/admin/settings`：公开业务设置 allowlist（站点名/版本/公告/注册开关/开放登录开关）。
- `/admin/logs`：结构化审计列表与详情；JSON 以文本/`<pre>` 渲染，不使用 `v-html`。
- `/forbidden`：非管理员访问兜底页。

## 权限与安全

- 前端 `meta` 角色判断只用于导航，不是授权边界；Gateway/Auth 注入身份后，Users/Chat/Group/File/Settings/Logs 目标服务都会再次校验 `Role == 1`。
- Admin 不编辑好友关系；好友由用户端管理。
- 日志/消息不执行 HTML；部署 Secret 永不进入 Settings。
- 生产构建存在非阻塞 chunk-size warning（ECharts 体积），不影响构建成功。

## 测试

- 单元/组件：`pnpm test`（Vitest）。
- 当前结果与未验证项见 `docs/validation-status.md`。
