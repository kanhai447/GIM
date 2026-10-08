# GIM Agent Worklog

> 由 Agent 追加维护，禁止覆盖历史。

## 2026-10-07 Day 1 / Phase 0 — Checkpoint 1 独立工程骨架

### 目标

- 建立 GIM 自有 `server/`、`web/`、`admin/` 边界和可重复仓库安全检查。

### 开始状态

- branch: `main`
- HEAD: `c1dc8d47a0e294b66bd7097227b2b0e5fd09540f`
- working tree: 存在此前已生成但未提交的 `agent_logs/operations/pre-day1/`。

### 修改

- 新建独立 Go module、最小可编译 package、Server/Web/Admin 空骨架。
- 新建不回显 Secret 的仓库安全检查脚本。
- 新建 `agent_logs/CURRENT_STATE.md`。

### 数据/协议变化

- DB: 无。
- HTTP: 无。
- WebSocket: 无。
- Redis: 无。

### 测试

- repository safety script -> PASS。
- `gofmt -l .` -> PASS。
- `go test ./...` -> PASS。
- `go vet ./...` -> PASS。
- reference dependency scan -> PASS。

### Git

- commit: `81340911fe58df2ccaee3e3fc540b6c6c7cbf662 chore(day1): establish independent project skeleton`。
- push: `origin/main` -> SUCCESS。
- tag: 无。

### 遗留与下一步

- 完成 Checkpoint 1 commit/push 后实施 Checkpoint 2 公共基础模块。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 2A 公共 HTTP 基础

### 目标与修改

- 自主实现 dotenv 配置读取、统一应用错误和 V1 HTTP 响应。
- 未新增业务路由、数据库访问或 WebSocket 实现。

### 参考与调整

- 保留 FIM `{code,msg,data}` 与业务 HTTP 200 兼容行为。
- 不沿用直接返回 `err.Error()`；未知错误统一隐藏内部细节。

### 测试

- `go test ./...` -> PASS。
- `go test -cover ./internal/platform/...` -> PASS。
- `go vet ./...`、`gofmt -l .`、安全脚本 -> PASS。
- `go test -race ...` -> NOT AVAILABLE，本机 cgo C 编译器不支持 64 位模式。

### Git

- commit: `e6811c9df4248c3af9e84b905da6bdb498958e2a feat(server): add safe config and HTTP foundations`。
- push: `origin/main` -> SUCCESS。

### 下一步

- 完成 Checkpoint 2A commit/push，然后实施 Checkpoint 2B 基础设施客户端。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 2B 基础设施客户端

### 目标与修改

- 新增可注入的 MySQL/GORM、Redis、etcd 客户端与 context 健康检查、Close 生命周期。
- 新增真实本地基础设施集成测试；未执行 migration 或业务数据写入。

### 参考与调整

- 参考 FIM 基础连接需求和 ServiceContext 依赖关系。
- 不沿用 panic、无效 timeout context、反复创建且不关闭 etcd client 的方式。

### 测试

- `go test ./...`、cover、vet、gofmt、安全脚本 -> PASS。
- MySQL connect/Ping/错误凭据脱敏/Close -> PASS。
- Redis Ping/Close -> PASS。
- etcd Status/Close -> PASS。
- race -> NOT RUN，本机 cgo 64 位工具链不可用。

### Git

- commit: `b255dd3221a7fc83f15aeb3a15cfaf70c7257b93 feat(platform): add mysql redis and etcd clients`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 2B 已完成并推送；停止，下一次进入 Checkpoint 3。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 3 User API / RPC 基础能力

### 目标与修改

- 新增 User Domain、GORM Repository、共享 Service、go-zero HTTP Handler 与 gRPC Adapter。
- RPC 提供 CreateUser、GetUserByID、GetUserByAccount；公开 UserInfo 不含 password hash。
- 同步 users account/status 模型约束和 HTTP 错误码文档；未执行 migration。

### 参考与调整

- 参考 FIM User 字段语义、`/api/user/user_info` 路径及 User RPC 服务调用关系。
- GIM 不在 User 内 Hash 密码，不返回整模型 JSON bytes，不暴露数据库原始错误；HTTP/RPC 共用 Service。

### 测试

- `go test ./...`、User cover、vet、gofmt、安全脚本 -> PASS。
- Handler 公开字段与内存 gRPC 生成客户端调用 -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `3aa0150ffbeecadbcc872344c70b67cf24a0ce0a feat(user): add user api rpc and domain foundation`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 3 已完成并推送；停止，下一次进入 Checkpoint 4 Auth。

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 4 Auth 服务

### 目标与修改

- 自主实现 Auth 注册、登录、bcrypt、JWT、authentication、Logout 与 Redis blacklist。
- 新增 User RPC 适配、统一 Redis key helper、四个 HTTP Handler 和真实 Redis TTL 集成测试。
- 未实现 Gateway、第三方登录、migration 或任何 Day 2 内容。

### 参考与调整

- 参考 FIM Auth 路由、`Token`/`ValidPath`、User/Settings RPC 边界和 Logout TTL 语义。
- GIM 改为 Auth 侧强哈希、User RPC 登录查询、精确公开路径、`jti/iat/exp` Claims、SHA-256 fingerprint key、调用方 context 传播和安全错误映射。

### 测试

- `go test ./...`、Auth cover、vet、gofmt、安全脚本、diff check -> PASS。
- Auth + Redis key helper 汇总 coverage：72.8% statements。
- 真实本地 Redis blacklist 写入、查询与 TTL -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。
- 额外基础设施复核：MySQL/Redis PASS；本机 etcd 未运行，因此既有 etcd health test 连接被拒绝。

### Git

- commit: `519282dbdfdbead9efeb556e6013dd14104d1c08 feat(auth): add registration jwt authentication and logout`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 4 已完成并推送；下一 Checkpoint 为 Gateway，不在本轮实施。

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 5 Gateway 鉴权代理

### 目标与修改

- 自主实现 Gateway 显式路由、etcd 服务发现/lease 注册、独立 Auth Client、可信身份 Header 注入和 HTTP Reverse Proxy。
- 增加 Gateway 可执行入口与真实 etcd 下 Gateway/Auth/User 注册、登录、认证、资料访问、注销集成测试。
- 未实现 Chat/Group WebSocket、Hub、Presence、ACK、File、Kafka、Docker 或 Day 2 内容。

### 参考与调整

- 参考 FIM `/api/{service}/...`、etcd 查询、Auth authentication、身份 Header 和 ReverseProxy 业务语义。
- GIM 改为显式服务表、可扩展多端点 Resolver、lease/keepalive 注册、调用方 context/timeout、身份 Header 先删除后可信注入、安全错误映射和单一 Auth allowlist。

### 测试

- `go test ./...`、Gateway cover、vet、gofmt、安全脚本、diff check -> PASS。
- Gateway 相关汇总 coverage（含集成测试）：79.5% statements。
- 真实 etcd Gateway -> Auth -> User 注册/登录/认证/User API/Logout，以及身份 Header 防伪造 -> PASS。
- etcd health/status 与测试注册清理 -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `eb9eebc6bfe4ee41c388d6eb6297cbfca43316bf feat(gateway): add auth-aware service proxy`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 5 已完成并推送；下一次进入 Checkpoint 6，本轮不实施。

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 6 Core Integration Acceptance

### 目标与修改

- 对既有 Platform/User/Auth/Gateway/MySQL/Redis/etcd 做完整回归、安全、context 与资源生命周期验收。
- 补充真实链路错误签名/过期凭证、ValidPath 伪造、不可用 upstream 与 lease keepalive/cleanup 测试。
- 修复 discovery registration Close 未等待 keepalive goroutine 退出的问题；无业务协议、migration 或 Day 2 功能变化。

### 验收结果

- Register/Login/JWT/Gateway/Auth/User 主链路与 public/protected path -> PASS。
- 缺失、malformed、错误签名、过期、Logout 后凭证 -> 均返回预期 Auth 公开错误。
- `User-ID` / `Role` / `ValidPath` 伪造 -> 删除后注入普通用户可信身份，PASS。
- etcd 注册/发现/lease/keepalive/cleanup、服务不存在、lookup timeout、upstream 不可用 -> PASS，服务 key 无残留。
- 请求 context/timeout 正确传播；HTTP/MySQL/Redis/etcd/RPC/lease 资源关闭与公开错误安全审计 -> PASS。

### 测试

- `go test ./... -count=1`、core cover、`go vet ./...`、gofmt、Secret Scan、diff check -> PASS。
- 真实 MySQL/Redis/etcd/Gateway/Auth/User/lease 生命周期测试 -> PASS。
- Day 1 核心汇总 coverage：75.7% statements。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `af53a76a1217918fb2f2008b068ff451908bbd95 test(day1): verify core auth gateway user integration`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 6 已完成并推送；下一次进入 Checkpoint 7 migration，本轮不实施。

## 2026-10-08 Day 1 / Phase 1 — Checkpoint 7 正式数据库底座

### 目标与修改

- 新增 001–006 成对 UP/DOWN SQL、嵌入式轻量 migration Runner/CLI 和 dirty failure state。
- 正式建立 Identity/Friend/Private Chat/Group/File/Settings 共 16 张 V1 业务表，并将 User Domain/GORM record 与 Schema 对齐。
- 新增隔离 MySQL 集成验收：字段/default/collation/index、真实 duplicate constraint、11 类 EXPLAIN、User/Auth 注册登录、UP/DOWN/RE-UP。

### Reference 与自主改进

- 只读参考 FIM 相关 Model/查询的业务语义，没有复制 SQL、GORM Model 或配置。
- GIM 使用清晰 pending 状态、clientMsgId 幂等、独立 chat/group session、用户维度隐藏/删除、SHA-256 FileObject/UserFile 分层和公开 Settings 边界。
- V1 统一无数据库 FK，通过服务事务、唯一/非空/CHECK/索引保持一致性；明确好友接受、消息+session、建群+owner member、文件补偿边界。

### 验收

- EMPTY→UP、重复 UP、ALL DOWN、RE-UP：PASS；隔离测试库自动清理。
- 9 类 duplicate insert：全部被预期唯一约束拒绝。
- 11 类 EXPLAIN：索引与谓词/排序匹配；小数据量下两类历史查询由 optimizer 选择 PRIMARY 反向扫描，设计组合索引仍为 possible key。
- User/Auth 真实 Repository + gRPC 注册、登录、lookup：PASS。
- `go test ./... -count=1`、75.7% core coverage、vet、gofmt、Secret Scan：PASS。
- Race：NOT RUN - environment limitation（Windows cgo 64 位编译器限制）。

### Git / 下一步

- implementation commit：`b38a893 feat(db): add v1 schema and versioned migrations`，push SUCCESS。
- Checkpoint 7 完成即停止；Checkpoint 8 Web/Admin 仅在用户明确指令后进入。

## 2026-10-08 Day 1 / Phase 1 — Checkpoint 8 Web + Admin 基础工程

### 目标与修改

- 自主建立相互独立的 Web 与 Admin Vue 3 SPA；统一使用 pnpm、TypeScript、Vite、Pinia、Router 和 typed Axios API 层。
- Web 使用 Element Plus，完成登录/注册、Auth Store/session restore、protected guard、主布局与 Profile/Chat/Group/File 基础入口。
- Admin 使用 Arco Design + ECharts，复用现有 Auth，完成 role UI guard、管理布局、Dashboard 空状态及 User/Chat/Group/File/Settings/Logs 骨架。
- API 默认同源，并由 Vite 开发代理连接可配置 Gateway；无硬编码生产地址或 Secret。

### Reference 与自主实现

- 仅参考 FIM Web 的页面/API 语义和 FIM Admin 的真实模块范围；全部配置、组件、store、HTTP Client、路由和测试均自主编写。
- 避免原 Admin 错误 Logout URL、三个 Mock Dashboard endpoint 和 `v-html`；没有复制原前端，也没有进入 WebSocket/File/Kafka 业务。

### 验收

- Web：`pnpm install`、type-check、production build、4 files / 9 tests PASS。
- Admin：`pnpm install`、type-check、production build、5 files / 9 tests PASS；ECharts 按需运行验证 PASS。
- lint：两端均未配置；没有为本 Checkpoint 无依据新增工具链。
- 真实 etcd Gateway/Auth/User Register/Login/User Info/Logout/注销后拒绝链路 PASS；前端 API contract 路径和 `token` Header PASS。
- env/ignore/Secret Scan/diff check PASS；无 Token、JWT signing material、数据库/Redis凭据或 FIM Secret。

### Git / 下一步

- Web commit：`19512be8afe74cd03cb457b0b3a96770f4ac14e0`。
- Admin commit：`59490fae4e4590e3a91d49a26234c30564065267`。
- push：SUCCESS — `origin/main`。
- Checkpoint 8 完成后停止；下一次仅在用户明确指令下进入 Checkpoint 9。
