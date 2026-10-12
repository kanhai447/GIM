# GIM Current State

- **Current Day:** DAY 3
- **Current Phase:** GROUP FRONTEND INTEGRATION
- **Current Checkpoint:** DAY 3 / CHECKPOINT 4 COMPLETED
- **Last Completed Checkpoint:** Day 3 / Checkpoint 4 — Group Frontend Integration
- **Working Tree:** CLEAN（本状态随 Checkpoint 4 实现/日志双提交并 push 后）
- **Last Stable Business Baseline:** `0a17af6 feat(web): integrate reliable group chat`
- **Last Push:** Checkpoint 4 implementation 已推送 `origin/main`；本 log/status commit 随后推送并复核 HEAD equality
- **Next Authorized Work:** NONE
- **Next Planned Work:** FILE SERVICE CHECKPOINT（需用户明确授权）
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
- 私聊 message + sender session + receiver session/unread 原子 transaction；ACK 只在 commit 后返回。
- receiver 在线/离线统一 unread 原子 +1；50 duplicate 只 +1，50 distinct 无 lost update，last message 不倒退。
- 用户级 Mark Read、read cursor 单调推进、partial/stale/latest/concurrent read 正确；不发送 peer read receipt。
- 私聊 Session List、数据库 top/time 排序、peer/last-message JOIN、用户维度 hidden 与新消息恢复可见。
- Message-ID cursor History、双向会话隔离、limit 上限和当前用户维度 `chat_message_deletions` 过滤。
- 用户端 typed Chat API、单例 Chat Socket、`disconnected/connecting/connected/reconnecting` 状态和 30 秒上限指数退避。
- 私聊 text pending/sent/failed、stable clientMsgId、persist ACK mapping、ACK timeout/error retry 与 HTTP/WS/pending 去重。
- `/chat/:peerId?` session/unread/top、current/other peer realtime、focus-aware mark read 与 message-ID cursor history/load older UI。
- 真实双用户 Chrome + Gateway/Auth/Chat/MySQL/Redis：pending→sent、realtime/unread/read、刷新恢复、receiver 双设备、多页 history 与 Chat API 重启重连 PASS。
- Checkpoint 7 真实双向/双设备 Chrome、ACK lost 联合证据、logout/revoked auth、共享 read、HTML 安全、并发/稳定性/生命周期/安全最终验收；`PRIVATE CHAT V1 = COMPLETED`。
- Group Domain Foundation：独立 Domain/Repository/Service/Policy、owner/admin/member 与 active/dissolved 状态。
- Group Membership：创建群 owner 原子 membership、邀请、申请/open join、approve/reject、leave/kick/role/dissolve。
- Group Permission：集中 policy，可信 Gateway identity，稳定安全错误；owner/admin/member 权限矩阵通过。
- Group Mute Foundation：集中 Redis membership key、TTL mute/unmute/expiry/cross-group isolation；Group 不修改 Presence。
- Group API/RPC：基础 HTTP 路由、最小 membership RPC、User BatchGetUsers，无 Group WebSocket/message。
- Group WS Foundation：独立用户级 Hub/Client、同用户多设备、readPump/唯一 writePump、typed inbound/error boundary。
- Group heartbeat/lifecycle：Ping/Pong、deadline、normal/abnormal/timeout/slow/shutdown 清理与 bounded backpressure。
- Group Gateway tunnel：Auth query token、可信身份覆盖、Origin allowlist、etcd registration 与 graceful cleanup。
- Group Presence negative boundary：connect/disconnect/timeout/slow/shutdown 均不创建、修改或删除 `gim:presence:*`。
- Group Message Reliability：typed text send/ack/message、可信 sender、active/member/mute validation、MySQL UNIQUE 幂等与 commit 后 ACK。
- Group Transaction：message + transaction-time current-member sessions/unread 原子提交；duplicate 不重复 session/publish，50 distinct 无 lost update。
- Group Session/Read/History：JOIN session list、monotonic partial/latest/stale read、message-ID cursor、membership authorization 与 per-user deletion。
- Group Realtime：独立 `gim:group:delivery`、actual multi-instance/multi-device WS fanout、sender sibling、Redis degraded recovery；Presence remains Chat-only。
- Group Frontend：独立 typed API/socket/store/components，`/group/:groupId?`、My Groups/Group Sessions、info/member cache、text composer。
- Group Client Reliability：stable clientMsgId、pending/sent/failed、ACK/message 双顺序合并、duplicate/lost ACK retry、30 秒上限退避与 auth/logout terminal lifecycle。
- Group Web State：realtime preview/order/unread、focus-aware Mark Read、message-ID cursor history、reconnect reconciliation 与多设备 shared read。
- Group Browser Acceptance：真实 Chrome A/B/C、mute/unmute、HTML literal、20+ history、Group restart/Chat unaffected、logout/revoked auth PASS；`GROUP CHAT V1 = FUNCTIONAL`。

## In Progress

- 无；Day 3 / Checkpoint 3 已完成并冻结，等待用户授权下一阶段。

## Authorization

Day 2:
COMPLETED

Day 3 / Checkpoint 1:
COMPLETED

Day 3 / Checkpoint 2:
COMPLETED

Day 3 / Checkpoint 3:
COMPLETED

Day 3 / Checkpoint 4:
COMPLETED

Private Chat V1:
COMPLETED / REGRESSION PASS

Next Authorized Work:
NONE

## Not Started

- File、Presence UI、Kafka/Logs 与完整 Admin 业务；均未获下一 Checkpoint 授权。

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
- Day 2 / Checkpoint 5：非缓存全量测试、vet、gofmt、diff check、Secret Scan、Message/Session/MySQL 71.1% 与 HTTP transport 56.3% coverage、真实 MySQL 50 duplicate/50 distinct、partial/stale/latest/concurrent read、session/top/hidden、history cursor/per-user deletion、三类 transaction rollback、Redis post-commit degraded delivery、Pub/Sub/Presence/Gateway regressions 全部 PASS。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。
- Day 2 / Checkpoint 6：Web type-check、8 files / 35 tests、production build、diff check、Secret Scan、Go 全量非缓存测试/vet PASS；真实 Chrome 双用户 pending→sent、realtime/unread/read、刷新恢复、receiver 双设备、20+ cursor history 与 Chat API 重启 reconnect 全部 PASS。lint NOT CONFIGURED。Race 未重复执行，状态保持 NOT AVAILABLE；兼容环境验证 PENDING。
- Day 2 / Checkpoint 7：真实 Chrome 双向/双设备/shared read/cursor/reconnect/logout/revoked token/HTML text PASS；Web type-check、8 files / 40 tests、build PASS；真实 Go full integration、message/session 3 轮、Presence 3 轮、Hub/heartbeat/single-writer/shutdown 10 轮、vet/gofmt/diff/Secret Scan 全部 PASS。lint NOT CONFIGURED。Race 保持 NOT AVAILABLE；兼容环境/CI PENDING。
- Day 3 / Checkpoint 1：真实 MySQL/Redis Group integration 连续 3 次 PASS；create/approve rollback injection、duplicate/concurrent add/approve、join/reject/invite/leave/kick/role/dissolve、TTL/expiry/cross-group mute、Presence isolation、四类 EXPLAIN PASS。Group coverpkg 67.9%、Repository 77.4%、Service 67.0%。Private Chat focused regression、`go test ./... -count=1`、vet/gofmt/diff/Secret Scan PASS。Race 保持 NOT AVAILABLE；兼容环境/CI PENDING。
- Day 3 / Checkpoint 2：Group Hub/Client/pumps/heartbeat/multi-device/slow/shutdown/Origin/typed error PASS；真实 etcd Gateway/Auth/Group tunnel 与 Redis Presence negative PASS；正式 Group API lease/cleanup PASS；10 轮 stress PASS，Group WS 汇总 coverage 77.3%。Group API 与 Private Chat regression、真实依赖全量测试、vet/gofmt/diff/Secret Scan PASS。Race 保持 NOT AVAILABLE；Chat + Group compatible CI PENDING。
- Day 3 / Checkpoint 3：真实 MySQL/Redis Group message integration 连续 3 轮 PASS；50 duplicate/50 distinct、transaction failure injection、partial/latest/stale read、session/history/deletion、actual two-instance/multi-device Group WS、Redis degraded、Presence negative 与 EXPLAIN PASS。全量真实依赖测试、Private Chat regression、vet/gofmt/diff/Secret Scan PASS；focused coverpkg 69.1%。Race 保持 NOT AVAILABLE；compatible CI PENDING。
- Day 3 / Checkpoint 4：Web 12 files / 77 tests、type-check、production build PASS；真实 Chrome A/B/C pending/ACK/realtime/unread/read/refresh/mute/multi-device/cursor/reconnect/logout/revoked auth/HTML 安全 PASS。Group + Private Chat 真实 MySQL/Redis focused integration、后端全量、vet/gofmt/diff/Secret Scan PASS。Race 保持 NOT AVAILABLE；compatible CI PENDING。

## Known Issues

- `protoc 3.9.0` 较旧，后续首次生成 RPC 代码时需要验证与当前 Go 插件兼容性。
- 本机只有 MinGW.org GCC 6.3.0 `mingw32/i586` C 编译器，不支持 windows/amd64 race 构建；未发现可仅切换 `CC` 使用的本机 64 位编译器。Private Chat V1 已按本地可执行范围验收，但 `go test -race ./...` 仍必须在兼容环境/CI 补验。
- Gateway 当前选择 Resolver 返回的首个有序健康注册端点；负载均衡/主动健康探测留待后续多实例阶段。私聊 receiver realtime fanout 已通过 Redis Pub/Sub 跨 Chat API 实例，但不等同于通用分布式 WS router。
- Gateway 的 Upgrade 检测、query token 认证和真实 Chat WebSocket tunnel 已验证；Chat 内部 endpoint 信任 Gateway Header，正式部署不得直接暴露公网。
- Presence key helper 已集中建立；未来若确需 connection/device tracking key，仍必须加入 `rediskeys`，不得硬编码。
- V1 pending queue 仅保存在内存；刷新页面前尚未获得服务端 ACK 的消息不会跨刷新保留，已持久化消息由 history 恢复。

## Next Action

- Group Domain/Membership、Group WebSocket、Group Message Reliability/Session/Unread/History 与 Group Frontend 已完成；Presence 仍为 Chat WS ONLY；`GROUP CHAT V1 = FUNCTIONAL`，`PRIVATE CHAT V1 = COMPLETED / REGRESSION PASS`。
- Next Authorized Work：NONE。Next Planned 是 **File Service Checkpoint**；必须等待用户明确授权。本轮严格停止，不进入 File、Admin 或 Day 4。
