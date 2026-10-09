# Day 1

## 今日目标

- 仅执行 Phase 0、0.5、1，并按稳定 Checkpoint 完成测试、日志、commit、push 闭环。

## 完成项

- [x] Checkpoint 1：安全配置检查与 GIM 独立工程骨架。
- [x] Checkpoint 2A：配置、统一错误与 HTTP 响应。
- [x] Checkpoint 2B：MySQL、Redis、etcd 客户端。
- [x] Checkpoint 3：User API / RPC 基础能力。
- [x] Checkpoint 4：Auth 注册、登录、JWT、Logout。
- [x] Checkpoint 5：Gateway 与 Auth 鉴权链路。
- [x] Checkpoint 6：Day 1 Core Integration Acceptance。
- [x] Checkpoint 7：Day 1 数据库底座与 migration。
- [x] Checkpoint 8：Web/Admin 基础工程与 build。
- [x] Checkpoint 9：Day 1 全量验收与冻结。

## Checkpoint 1

- 日期：2026-10-07
- Phase：0
- 主要改动：独立 Go module、三工程空骨架、仓库安全检查脚本、可恢复状态文件。
- DB/API/WS 变化：无。
- 测试：安全脚本、gofmt、Go test/vet、reference 零依赖、工程边界均 PASS。
- 问题：首次测试脚本相对路径错误；修正调用路径后完整重跑通过。
- Commit：`81340911fe58df2ccaee3e3fc540b6c6c7cbf662`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- 从 Checkpoint 2B MySQL、Redis、etcd 客户端恢复。

## Checkpoint 2A

- 日期：2026-10-07
- Phase：0.5
- 主要改动：安全配置解析、统一应用错误、V1 HTTP 响应与单元测试。
- DB/API/WS 变化：没有业务路由或字段变化；公共响应保持 `{code,msg,data}`。
- 测试：普通单测、覆盖率、vet、gofmt、安全脚本 PASS。
- Race：本机 cgo 编译器不支持 64 位模式，真实状态为 NOT AVAILABLE。
- Commit：`e6811c9df4248c3af9e84b905da6bdb498958e2a`。
- Push：SUCCESS — `origin/main`。

## Checkpoint 2B

- 日期：2026-10-07
- Phase：0.5
- 主要改动：MySQL/GORM、Redis、etcd 可注入客户端，context 健康检查与显式 Close，真实本地集成测试。
- DB/API/WS 变化：无 migration、无业务表、无业务 Redis key、无 API/WS 变化。
- 测试：全量单测、覆盖率、vet、gofmt、安全脚本、MySQL/Redis/etcd 集成测试 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN。
- Commit：`b255dd3221a7fc83f15aeb3a15cfaf70c7257b93`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 2B 已完成并推送；下一次从 Checkpoint 3 User API / RPC 基础能力恢复。

## Checkpoint 3

- 日期：2026-10-07
- Phase：0.5
- 主要改动：User Domain/Repository/Service、公开资料 HTTP Handler、Create/GetByID/GetByAccount gRPC 契约与生成客户端集成测试。
- DB/API/RPC 变化：建立 users GORM 模型契约但未 migration；实现 `GET /api/user/user_info`；新增三个内部 User RPC。
- 安全：Auth 负责 Hash，User 只接收 `password_hash`；公开 DTO 不返回密码哈希，底层数据库错误不泄露。
- 测试：全量单测、User 覆盖率、vet、gofmt、安全脚本、内存 gRPC 调用 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- Commit：`3aa0150ffbeecadbcc872344c70b67cf24a0ce0a`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 3 已完成并推送；下一次从 Checkpoint 4 Auth 注册、登录、JWT、Logout 基础链路恢复。

## Checkpoint 4

- 日期：2026-10-08
- Phase：0.5
- 主要改动：Auth 注册/登录、bcrypt、JWT Claims 与验证、精确公开路径、authentication、Logout Redis blacklist、User RPC 适配与 HTTP Handler。
- DB/API/Redis 变化：无 migration；实现四个 Auth HTTP 路由；新增统一 `gim:auth:logout:{tokenHash}`，TTL 为 JWT 剩余时间。
- 安全：签名材料仅从安全配置读取；不记录口令/完整认证材料；Redis key 使用 SHA-256 fingerprint；登录失败统一外部提示。
- 测试：全量单测、72.8% Auth 汇总 coverage、vet、gofmt、安全脚本、真实 Redis TTL 集成测试 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- Commit：`519282dbdfdbead9efeb556e6013dd14104d1c08`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 4 已完成并推送；下一次仅在用户明确指令下进入 Checkpoint 5 Gateway。

## Checkpoint 5

- 日期：2026-10-08
- Phase：0.5
- 主要改动：Gateway 显式服务路由、etcd 多端点 discovery 与 lease registry、Auth Client、身份 Header 防伪造和 HTTP Reverse Proxy。
- API/Discovery 变化：所有合法 Gateway API 先调用 Auth authentication；新增 `/gim/services/{service}/{instanceID}` 注册契约和 Gateway `1301`–`1306` 错误语义。
- 安全：删除客户端 `User-ID` / `Role` / `ValidPath`，只注入 Auth 可信身份；仅 Gateway 应公开，内部 API 不作为公网入口。
- 测试：全量单测、Gateway 相关 79.5% 汇总 coverage、vet、gofmt、安全脚本、真实 etcd Gateway/Auth/User 注册登录认证注销链路均 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- Commit：`eb9eebc6bfe4ee41c388d6eb6297cbfca43316bf`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 5 已完成并推送；下一次仅在用户明确指令下进入 Checkpoint 6。

## Checkpoint 6

- 日期：2026-10-08
- Phase：0.5
- 主要内容：既有 Platform/User/Auth/Gateway 核心主链路、异常认证、Header 防伪造、context/timeout、资源关闭、错误与配置安全验收。
- 小修复：`Registration.Close` 在 revoke 后于调用方 context 内等待 keepalive goroutine 退出。
- 回归补强：真实链路分别覆盖 malformed、错误签名、过期、注销凭证，补充 ValidPath 伪造、不可用 upstream、2 秒 lease keepalive 与 cleanup。
- 测试：非缓存全量测试、真实 MySQL/Redis/etcd/Gateway/Auth/User 集成、核心 75.7% 汇总 coverage、vet、gofmt、Secret Scan 均 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- DB/API/WS 变化：无 migration、无协议或业务功能变化；未进入 Day 2。
- Commit：`af53a76a1217918fb2f2008b068ff451908bbd95`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 6 已完成并推送；下一次仅在用户明确指令下进入 Checkpoint 7 migration。

## Checkpoint 7

- 日期：2026-10-08
- Phase：1
- 主要改动：六个版本化 UP/DOWN migration、轻量 Runner/CLI、16 张 V1 业务表、正式 unique/check/default/index、User GORM 对齐和隔离 MySQL 验收。
- 数据设计：好友双向行 + pending pair 唯一；私聊/群聊 session 分离；clientMsgId 幂等；用户维度隐藏/删除；SHA-256 FileObject/UserFile 分层；Settings 与部署 Secret 分离；统一无数据库 FK。
- 实测：EMPTY→UP、重复 UP、ALL DOWN、RE-UP、9 类重复写入、11 类 EXPLAIN、真实 User Repository/Auth 注册登录与 lookup 全部 PASS，临时库已清理。
- 测试：`go test ./... -count=1`、75.7% 核心汇总 coverage、vet、gofmt、Secret Scan PASS。
- Race：NOT RUN - environment limitation，本机 cgo C 编译器不支持 64 位模式。
- Implementation commit：`b38a893`；push：SUCCESS — `origin/main`。
- DB/API/WS：新增正式 Schema 与 migration 命令；无新 HTTP/WS 协议，无 Day 2 行为。

## 次步入口

- Checkpoint 7 完成后停止；下一次仅在用户明确指令下进入 Checkpoint 8 Web/Admin 基础工程与 build。

## Checkpoint 8

- 日期：2026-10-08
- Phase：1
- Web：Vue 3/TS/Vite/Pinia/Router/Element Plus；完成 Login/Register、typed API Client、Auth Store、session restore、protected guard、Main/Profile 与 Chat/Group/File 占位入口。
- Admin：Vue 3/TS/Vite/Pinia/Router/Arco/ECharts；完成复用 Auth、role UI guard、Admin Layout、Dashboard 空状态与 User/Chat/Group/File/Settings/Logs 路由骨架。
- 安全：同源 API + 可配置 dev proxy；`.env.example` 无 Secret；local env、dist、node_modules 忽略；无 token 输出、Mock Dashboard API、`v-html`、WebSocket/File/Kafka 业务。
- 联调：真实 etcd 下既有 Gateway/Auth/User Register→Login→User Info→Logout→revoked request 集成链路 PASS；前端 API contract 路径/Header PASS。
- 测试：Web type-check/build + 4 files / 9 tests PASS；Admin type-check/build + 5 files / 9 tests PASS；lint 未配置；Secret Scan PASS。
- Commits：`19512be` Web，`59490fa` Admin；push：SUCCESS — `origin/main`。
- DB/API/WS：无数据库变化、无新后端协议、无 WebSocket；仅按既有 HTTP 契约接入基础能力。

## 次步入口

- Checkpoint 8 完成后停止；下一次仅在用户明确指令下进入 Checkpoint 9 Day 1 全量验收。

## Checkpoint 9

- 日期：2026-10-09
- Phase：1 Final Acceptance & Freeze。
- 补齐正式入口：Auth API、User API、User RPC；与既有 Gateway/Migrate 共同形成可启动 Day 1 后端。
- 修复 Windows 下 go-zero REST signal hook 不执行 shutdown 的问题；四个进程 Ctrl+C 均 exit 0，etcd key immediate cleanup。
- 真实多进程：Register→Login→Gateway/Auth→User Info→Logout→旧 credential 拒绝；Header spoofing、service missing、upstream unavailable 均 PASS。
- Database：隔离 MySQL UP/DOWN/dirty fail-fast/RE-UP、约束/EXPLAIN/User/Auth 回归 PASS。
- Backend：全量非缓存 test、vet、gofmt PASS；核心 coverage 76.7%。
- Web：type-check/build、4 files / 9 tests PASS；Admin：type-check/build、5 files / 9 tests PASS；lint 均未配置。
- Security/reference：Secret Scan、ignore、协议、context/resource、reference independence PASS。
- Race：NOT RUN - environment limitation，Windows cgo C compiler 不支持 64 位模式。
- Runtime commit：`67d1077e20cda66169a755cae9f36c7996c8a09c`；push SUCCESS。
- DB/API/WS：无新业务协议、无 schema 变化；只新增正式 runtime wiring 与 migration 验收测试，未进入 Day 2。

# Day 1 Final Summary

## Phase 0

- 完成三份 FIM reference 只读审计、安全边界确认、Git/GitHub 基线、`.gitignore`、安全脚本和独立 `server/ web/ admin/` 空骨架。
- Reference 未进入 Git，也不是编译/运行依赖；GIM 从空工程自主实现。

## Phase 0.5

- 完成安全 dotenv 配置、统一应用错误/HTTP envelope、MySQL/GORM、Redis、etcd 客户端与显式 Close。
- 完成 User Domain/Repository/HTTP/gRPC、Auth 注册登录/JWT/Logout blacklist、Gateway/Auth/discovery/proxy 主链路。
- 完成真实基础设施集成、Header 防伪造、context/timeout、错误安全与 lease 生命周期验收。

## Phase 1

- 完成六个版本化 migration、16 张 V1 业务表、约束/索引/dirty protection/UP-DOWN-RE-UP。
- 完成独立 Web 与 Admin 基础工程、typed HTTP/Auth/Router/store/layout/page skeleton。
- 补齐 Gateway、Auth API、User API、User RPC 正式 runtime，完成真实多进程最终验收与 Day 1 freeze。

## Backend

- 可启动命令：`cmd/gateway`、`cmd/auth-api`、`cmd/user-api`、`cmd/user-rpc`、`cmd/migrate`。
- Gateway 保持 Client → Gateway → Auth → Gateway → Business Service；不在 Gateway 本地验签。
- Auth API 使用 Redis logout fingerprint TTL，User API/RPC 使用真实 MySQL；API 服务使用 etcd lease 注册。
- startup/shutdown timeout 配置化；HTTP/gRPC、MySQL、Redis、etcd、lease/keepalive 和连接均有关闭路径。

## Database

- Migration 001–006 覆盖 Identity、Friend、Chat、Group、File、Settings；正式 schema 不依赖 AutoMigrate。
- clientMsgId 幂等、chat/group sessions、unread/read/top、SHA-256 FileObject/UserFile 字段和关键索引已准备。
- 真实隔离 MySQL 的 UP、重复 UP、DOWN、dirty fail-fast、RE-UP、9 类 duplicate constraint、11 类 EXPLAIN 全部 PASS。

## Web

- Vue 3 + TypeScript + Vite + Pinia + Router + Axios + Element Plus。
- Login/Register、Auth Store、session restore、Router Guard、User Info、Logout、Main/Profile，以及 Chat/Group/File 占位入口完成。
- 同源 API + 可配置 Vite Gateway proxy；未提前实现实时业务。

## Admin

- 独立 Vue 3 + TypeScript + Vite + Pinia + Router + Axios + Arco Design + ECharts SPA。
- Login/Forbidden、role UI guard、Admin Layout、Dashboard 空状态和 User/Chat/Group/File/Settings/Logs 路由骨架完成。
- 不信任前端 role 作为服务端安全边界；未接入 FIM Mock Dashboard 或危险 HTML 渲染。

## Security

- Auth 错误签名/过期/注销、公开/保护路径、Header spoofing 与安全 Gateway/discovery/proxy 错误均通过。
- `.env.local`、reference、private key、真实凭据、完整 credential、node_modules/dist/local data 均未跟踪。
- `server/web/admin` 无 reference import、runtime read、symlink、module/package/build dependency。

## Tests

- Backend：`go test ./... -count=1`、`go vet ./...`、gofmt PASS；核心 coverage 76.7%。
- Integration：真实 MySQL/Redis/etcd、正式四进程 Gateway/Auth/User、lease keepalive/revoke、migration 全部 PASS。
- Web：type-check/build PASS，4 files / 9 tests；Admin：type-check/build PASS，5 files / 9 tests；lint NOT CONFIGURED。
- Race：NOT RUN - environment limitation，未虚报 PASS。

## Known Issues

- Windows cgo C compiler 不支持 64 位 race；Day 2 WebSocket 并发开发前应优先准备兼容环境。
- 本机 `protoc 3.9.0` 较旧，新增 RPC 契约时需验证插件兼容性。
- Gateway V1 只选择排序后的首个 lease endpoint；负载均衡和主动健康探测留待后续多实例阶段。

## Explicitly Not Implemented

- Day 2：Chat WebSocket、Hub/Client、readPump/writePump、Ping/Pong、Presence、ACK、clientMsgId 实时逻辑、session/unread 业务。
- Day 3：Group WebSocket、群消息可靠性、文件上传/下载/预览、Web 自动重连与实时状态。
- Day 4：真实 Admin 管理 API、Kafka/Logs 完整链路、最终部署与完整 V1 release。
- Docker Compose 最终部署、WebRTC、WS ticket、多实例 WS 路由均未在 Day 1 实现。

## Day 1 Freeze

- **DAY 1 = COMPLETED**。
- Checkpoint 9 完成后停止；Day 2 状态为 **NOT STARTED**，当前 **Next Authorized Work: NONE**。
