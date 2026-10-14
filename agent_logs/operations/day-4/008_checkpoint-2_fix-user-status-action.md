# Day 4 / Checkpoint 2：修复 Admin 用户启用/禁用操作

- 日期：2026-10-14
- Checkpoint 状态：IN PROGRESS
- 范围：仅修复 Admin Users 页面状态操作闭环；未进入下一 Checkpoint，未执行 Browser Computer Use、commit 或 push。

## 根因与正式语义

- User Domain 的正式状态为 `StatusActive=1`、`StatusDisabled=2`。
- 原页面状态标签能读取 `status`，但操作列固定渲染“禁用”，并在 disabled 时直接禁用按钮。
- 原 Admin User API 只有单向 Disable，缺少恢复 Active 的正式业务入口。

## 实现

- Repository 改为接受 User Domain `Status` 的通用状态更新，并拒绝无效枚举。
- 保留 `DELETE /api/user/users?userId=` 作为 Disable；新增 `PUT /api/user/users?userId=` 作为 Enable。
- Disable/Enable 成功响应都返回正式 `status`；Admin UI 仅在 API 成功后按该响应更新当前行，不依赖整页刷新。
- API 失败时不修改行状态；页面沿用公共安全错误映射，不展示数据库或内部错误。
- 增加 `admin.user.enabled` 结构化审计事件；事件不包含凭据或其他敏感字段。
- `e2e-member` 已通过正式 Admin User HTTP API 恢复并重新查询确认为 Active。

## 修改文件

- `server/internal/user/admin/service.go`
- `server/internal/user/admin/service_test.go`
- `server/internal/user/admin/mysql/repository.go`
- `server/internal/user/admin/transport/http/handler.go`
- `server/internal/user/admin/transport/http/handler_test.go`
- `server/internal/platform/audit/event.go`
- `server/tests/admin_integration_test.go`
- `admin/src/api/admin.ts`
- `admin/src/api/admin.contract.test.ts`
- `admin/src/views/UsersView.vue`
- `admin/src/views/userStatusAction.ts`
- `admin/src/views/userStatusAction.test.ts`
- `docs/05_HTTP_API.md`
- `docs/07_CORE_FLOWS.md`

## 数据库与协议变化

- 数据库：无 migration 或 schema 变化。
- HTTP：新增 `PUT /api/user/users?userId=`；Disable/Enable 响应新增正式 `status`。
- WebSocket / RPC：无变化。

## 测试与结果

- `pnpm type-check`：PASS。
- `pnpm test`：PASS，9 files / 21 tests。
- `pnpm build`：PASS；仅保留既有 chunk-size warning。
- `go test ./... -count=1`：PASS（标准无共享集成环境运行）。
- `go test ./tests -run TestAdminRepositoriesAndManagementActions -count=1 -v`：PASS（本地测试数据库）。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS，无未格式化文件。
- 额外一次带共享 `.env.local` 的全量集成运行因正在运行的 Manual E2E etcd 服务注册发生路由竞争，两个既有 Gateway 集成用例失败；未关闭 E2E 服务。针对本次状态逻辑的数据库集成测试单独运行通过。
- 安全扫描：未引入 `v-html`、密码日志、E2E 凭据或硬编码密码。

## 参考与自主实现说明

- 参考了原 Admin 的用户管理交互语义：管理员可以对用户执行限制类操作。
- GIM 未复制参考实现代码；本次基于 GIM 既有 User Domain 枚举、Admin middleware、Repository、审计事件和统一 API 错误模型独立实现双向状态闭环。
- 改进点：前端不再维护第二套数字语义，也不做与服务端脱节的乐观文案切换，而是使用后端返回的正式状态同步 UI。

## 运行状态

- 仅 User API 为加载修复执行了热重启；MySQL、Redis、Kafka、etcd、其他 API/RPC、Gateway、Logs Consumer/API 与 Admin frontend 保持运行。
- `e2e-member` 最终状态：enabled / Active。
