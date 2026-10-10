# GIM Current State

- **Current Day:** DAY 2
- **Current Phase:** PHASE 3 PRIVATE MESSAGE RELIABILITY IN PROGRESS
- **Current Checkpoint:** PRIVATE MESSAGE PERSISTENCE + CLIENTMSGID + SERVER ACK COMPLETED
- **Last Completed Checkpoint:** Private Message Persistence + clientMsgId + Server ACK
- **Working Tree:** CLEAN（本状态随 Checkpoint 4 日志提交并推送后）
- **Last Stable Business Baseline:** `6da24e4e161d5fc66c7d843eb676ff367e47f431` — `feat(chat): add idempotent private messaging and server ack`
- **Last Push:** Checkpoint 4 implementation + 日志提交完成后推送 `origin/main`，实际结果见本轮最终 Git 复核
- **Next Authorized Work:** Chat Session + Unread + Read State（等待下一 Checkpoint 明确指令）
- **Next Planned Work:** DAY 2 - PHASE 3 SESSION / UNREAD / READ STATE
- **Database Migration:** PASS — EMPTY → UP → DOWN → RE-UP on isolated local MySQL

## Completed

- Pre-Day1 Git/GitHub、安全配置与本地基础设施验证。
- `reference/`、`.env.local`、密钥、本地数据和构建产物忽略规则验证。
- MySQL、Redis、etcd、Go、Node/pnpm 环境验证。
- GIM 独立 Go module 与 `server/`、`web/`、`admin/` 空骨架。
- 可重复执行且不回显 Secret 的仓库安全检查脚本。
- 安全 dotenv 配置读取、统一应用错误与 V1 HTTP 响应基础模块。
- 可注入的 MySQL/GORM、Redis、etcd 客户端及真实本地连接测试。
- User Domain、GORM Repository、共享 Service、公开资料 HTTP Handler 与内部 gRPC 基础契约。
- Auth 注册/登录、bcrypt、JWT Claims/验证、精确公开路径、authentication、Logout Redis blacklist 与 User RPC 适配。
- Gateway 显式路由、etcd lease 注册/多端点发现、Auth Client、可信身份 Header 注入与 HTTP Reverse Proxy。
- 真实 etcd 下 Gateway -> Auth -> User 注册、登录、认证、资料访问、注销与身份防伪造集成链路。
- Day 1 核心主链路、Authentication 异常、Header 防伪造、discovery lease/keepalive/cleanup、context、资源与错误安全验收。
- 六个正式 MySQL migration、轻量 Runner/CLI、16 张 V1 业务表、约束/索引、dirty failure state 与 rollback 文档。
- 真实隔离 MySQL 的字段/default/constraint、9 类重复写入、11 类 EXPLAIN、User/Auth Repository 与 UP/DOWN/UP 回归。
- 独立 Web Vue 3 SPA：Element Plus、typed API Client、Auth Store、登录/注册、session restore、Router Guard 与主界面/业务占位路由。
- 独立 Admin Vue 3 SPA：Arco Design、ECharts 按需加载、复用 Auth、role UI guard、Admin Layout 与六类管理页面骨架。
- Web/Admin 同源 API + 可配置开发 Gateway proxy、安全 env 模板、独立 pnpm lockfiles、类型检查/单测/production build。
- Gateway、Auth API、User API、User RPC 正式启动入口及跨平台 graceful shutdown/资源关闭。
- 真实多进程 Register/Login/Gateway/Auth/User/Logout、Header 防伪造、lease keepalive 与 immediate cleanup 最终验收。
- Chat Hub event loop、Client、readPump/唯一 writePump、同用户多连接、bounded Send/backpressure 与 shutdown。
- 配置化精确 Origin policy、Chat API 正式入口、etcd 注册清理及 Gateway/Auth/Chat 真实 WebSocket tunnel。
- 配置化 Ping/Pong、read/write deadline、dead connection detection、原因分类与无 zombie cleanup。
- 同用户多设备独立 heartbeat 生命周期、total/user connection count 和可等待的 graceful shutdown。
- Redis-backed Chat Presence、集中 key helper、本实例 `0->1` / `1->0` transition 与多设备安全语义。
- 多 Chat API instance ZSET contribution、TTL/独立 refresh crash recovery、context timeout/retry 和 shutdown cleanup。
- typed `chat.send` / `chat.ack` / `chat.message` / `error`，可信 connection sender identity 与正文/payload 限制。
- Chat Message Service、MySQL Repository、User RPC receiver validation、curtail_chat 与好友 policy validation。
- MySQL `(send_user_id, client_msg_id)` 最终幂等、并发 duplicate 收敛、lost ACK retry 返回同一 messageId。
- Server persist ACK 只回 originating Client；offline receiver 不影响 ACK。
- Redis Pub/Sub 跨实例 best-effort realtime fanout；same-instance 统一走 bus，createdNew 防 duplicate delivery，多设备全投。

## In Progress

- 无；Checkpoint 4 已完成并停止，等待下一 Checkpoint 指令。

## Authorization

Day 2:
AUTHORIZED

Next Checkpoint:
Chat Session + Unread + Read State

## Not Started

- Day 2 后续：chat session、unread、mark read、history API 和前端 socket manager 尚未开始。
- Day 3+：Group WebSocket、File、完整 Web 协议适配、Kafka/Logs 与完整 Admin 业务。

## Test Status

- Pre-Day1 Verification：PASS。
- Checkpoint 1：安全脚本、`gofmt`、`go test ./...`、`go vet ./...`、reference 零依赖与目录边界检查均 PASS。
- Checkpoint 2A：普通单测、覆盖率、vet、gofmt 和安全脚本 PASS；race 因本机 cgo 64 位编译器不可用而未能执行。
- Checkpoint 2B：全量单测、覆盖率、vet、gofmt、安全脚本和 MySQL/Redis/etcd 真实集成测试 PASS；race 仍为 NOT RUN。
- Checkpoint 3：全量单测、User 覆盖率、vet、gofmt、安全脚本、HTTP Handler 与内存 gRPC 集成测试 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 4：全量单测、Auth 72.8% 汇总覆盖率、vet、gofmt、安全脚本、真实 Redis blacklist TTL 集成测试 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 5：全量单测、Gateway 相关 79.5% 汇总覆盖率、vet、gofmt、安全脚本、真实 etcd Gateway/Auth/User 全链路与身份 Header 防伪造测试 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 6：非缓存全量测试、Day 1 核心 75.7% 汇总覆盖率、vet、gofmt、Secret Scan、真实 MySQL/Redis/etcd/Gateway/Auth/User 与 lease 生命周期回归 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 7：非缓存全量测试、核心 75.7% 汇总覆盖率、vet、gofmt、Secret Scan、真实 MySQL UP/DOWN/UP、constraint/EXPLAIN 与 User/Auth 回归 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 8：Web/Admin `pnpm install`、type-check、production build、Secret Scan PASS；Web 4 files / 9 tests、Admin 5 files / 9 tests PASS；真实 etcd Gateway/Auth/User 注册登录资料注销链路 PASS；lint 未配置。
- Checkpoint 9：正式四进程 smoke、Auth/Gateway 异常与 Header 防伪造、etcd lease/keepalive/revoke、migration dirty/UP/DOWN/RE-UP、非缓存全量测试、vet、gofmt、76.7% 核心 coverage、Web/Admin test/type-check/build、Secret Scan 与 reference independence 全部 PASS；race 为 NOT RUN。
- Pre-Day2：`go test ./... -count=1` 及 Auth/Gateway/Redis focused tests PASS；`go test -race ./...` 在 runtime/cgo 构建阶段失败，准确错误为 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Day 2 后续必须在兼容环境/CI 执行 race 验证。
- Day 2 / Checkpoint 1：非缓存全量测试、vet、gofmt、diff check、Secret Scan、Hub/Client/slow-client/Origin 集成和真实 Gateway/Auth/Chat WebSocket tunnel 全部 PASS；Chat foundation coverage 78.6%。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。
- Day 2 / Checkpoint 2：全量测试、vet、gofmt、diff check、Secret Scan、heartbeat/lifecycle 10 轮稳定性和真实 Gateway Ping/Pong tunnel 全部 PASS；Chat heartbeat/lifecycle coverage 81.5%。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。
- Day 2 / Checkpoint 3：全量测试、vet、gofmt、diff check、Secret Scan、Presence 75.8% coverage、关键 transition/heartbeat/slow/shutdown/Redis failure 10 轮稳定性、真实 Redis 多实例/TTL 与 Gateway/Auth/Chat Presence tunnel 全部 PASS。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。
- Day 2 / Checkpoint 4：全量测试、vet、gofmt、diff check、Secret Scan、Message 69.5% / User RPC adapter 71.4% coverage、真实 MySQL 50-way 并发幂等、lost ACK retry、offline/online/multi-device/multi-instance Redis Pub/Sub、duplicate no-redelivery、Gateway 与 Presence 回归全部 PASS。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。

## Known Issues

- `protoc 3.9.0` 较旧，后续首次生成 RPC 代码时需要验证与当前 Go 插件兼容性。
- 本机只有 MinGW.org GCC 6.3.0 `mingw32/i586` C 编译器，不支持 windows/amd64 race 构建；未发现可仅切换 `CC` 使用的本机 64 位编译器。该问题不阻塞 Day 2 开发，但 Day 2 完成前必须在兼容环境/CI 做 race 验证。
- Gateway 当前选择 Resolver 返回的首个有序健康注册端点；负载均衡/主动健康探测留待后续多实例阶段。私聊 receiver realtime fanout 已通过 Redis Pub/Sub 跨 Chat API 实例，但不等同于通用分布式 WS router。
- Gateway 的 Upgrade 检测、query token 认证和真实 Chat WebSocket tunnel 已验证；Chat 内部 endpoint 信任 Gateway Header，正式部署不得直接暴露公网。
- Presence key helper 已集中建立；未来若确需 connection/device tracking key，仍必须加入 `rediskeys`，不得硬编码。

## Next Action

- Chat WebSocket Foundation、Heartbeat/Lifecycle、Presence 与 Private Message Persistence/clientMsgId/Server ACK 已完成，无 Day 2 blocker。
- 下一入口为 **Chat Session + Unread + Read State**；Race 必须在兼容环境/CI 补验。本轮不进入该 Checkpoint。
