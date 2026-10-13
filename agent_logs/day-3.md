# Day 3

## 授权状态

- Day 3：COMPLETED。
- Day 3 / Checkpoint 1：COMPLETED — Group Domain + Membership + API Foundation。
- Day 3 / Checkpoint 2：COMPLETED — Group WebSocket Foundation。
- Day 3 / Checkpoint 3：COMPLETED — Group Message Reliability + Session/Unread/History。
- Day 3 / Checkpoint 4：COMPLETED — Group Frontend Integration。
- Day 3 / Checkpoint 5：COMPLETED — File Service Backend Foundation。
- Day 3 / Checkpoint 6：COMPLETED — File Frontend Integration + Day 3 Final Acceptance。

## Checkpoint 1 — Group Domain + Membership + API Foundation

- 时间：2026-10-12 10:33 +08:00。
- 新增 Group Domain/Repository/Service/Policy，角色为 owner/admin/member，复用既有 `004_group` schema，无 migration 变化。
- 创建群保证 group + owner/invited members 原子；owner 一定存在于 membership。
- 加群支持 closed/open/approval foundation；open join 与 approve 都保证 request + member 原子；duplicate pending/concurrent approve 由唯一约束与行锁收敛。
- 完成 info/my groups/member list/invite/leave/kick/role/dissolve；成员列表通过 User BatchGetUsers 消除逐成员 RPC N+1。
- 完成 Redis TTL mute/unmute/IsMuted 与集中 membership key；Group 不触碰 Presence。
- 完成基础 HTTP 与最小 Group RPC；无 Group WS/message/ACK/session/unread/history/frontend。
- 真实 MySQL/Redis group integration 连续 3 次 PASS；事务 failure injection、并发 Add/Approve 与四类 EXPLAIN PASS。
- Group coverpkg 67.9%；Repository 77.4%；Service 67.0%。
- Private Chat focused regression 与全量 `go test ./... -count=1` PASS；vet/gofmt/diff/Secret Scan PASS。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI PENDING。

## Operation Log

- `agent_logs/operations/day-3/001_checkpoint-1_group-domain-membership.md`
- `agent_logs/operations/day-3/002_checkpoint-2_group-ws-foundation.md`
- `agent_logs/operations/day-3/003_checkpoint-3_group-message-session.md`
- `agent_logs/operations/day-3/004_checkpoint-4_group-frontend.md`
- `agent_logs/operations/day-3/005_checkpoint-5_file-backend-foundation.md`
- `agent_logs/operations/day-3/006_checkpoint-6_file-frontend-day3-final.md`

## Checkpoint 6 — File Frontend + Day 3 Final Acceptance

- 时间：2026-10-13 13:52 +08:00。
- 新增 typed File API/Store/models/formatters/components 与真实 Chrome smoke；上传、独立 progress task、50 MiB 预检、cursor list、refresh、authenticated Blob download/revoke、UserFile delete 与安全 error/auth/ownership UX 完成。
- File UI 只展示公开 metadata，HTML-like filename 纯文本；无 preview、`v-html`、`innerHTML`、public window.open 或 storage path。
- 真实 Chrome A/B：small/12 MiB upload-download hash、oversize preflight、refresh、20+ cursor、2503 owner boundary、shared dedup delete safety、malicious filename 和 confirmation全部 PASS。
- Private Chat A/B 与 Group A/B/C 真实浏览器最终回归 PASS；Group 从 FUNCTIONAL 升级 COMPLETED。
- etcd 五类 API 注册与 shutdown key cleanup、001–006 migration repeated up、Redis `gim:*` 清零、Gateway 五路、真实 MySQL/Redis/etcd integration全部 PASS。
- Web 16 files / 90 tests、type-check/build PASS；后端全量、File/Chat/Group 3 轮、WS lifecycle 10 轮、vet/gofmt/diff/Secret Scan/reference audit PASS；lint NOT CONFIGURED。
- implementation：`a9e1182 feat(web): integrate file management`；final log/status commit 随后推送并复核 equality/clean。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible CI PENDING。
- `PRIVATE CHAT V1 = COMPLETED`；`GROUP CHAT V1 = COMPLETED`；`FILE V1 = COMPLETED`；`DAY 3 = COMPLETED`。
- `DAY 4 = NOT STARTED`；Next Authorized Work：NONE。

## Checkpoint 5 — File Service Backend Foundation

- 时间：2026-10-13 13:13 +08:00。
- 新增独立 File Domain/Repository/Service/Local Storage/HTTP/runtime，复用既有 005_file，无 migration 变化。
- multipart 正文以双层 cap 流式写同盘 temp，同时 SHA-256 与 MIME sniff；最终按 hash 内容寻址并原子发布，production 无整文件 `io.ReadAll`。
- MySQL Attach 事务、SHA UNIQUE、per-hash lock 和补偿完成；50 并发同内容收敛，跨用户/改名保持独立 UserFile。
- owner-only list/detail/download/delete、安全 Content-Disposition、路径/CRLF/Unicode/size/MIME 防护、共享引用安全删除完成。
- 真实 MySQL/filesystem 与真实 etcd/Auth/Gateway 测试 PASS；DB/temp/move/reference failure injection、restart/missing physical/EXPLAIN PASS。
- `go test ./... -count=1`、Private Chat/Group focused 3 轮、vet/gofmt/diff/Secret Scan PASS；Web 77 tests/type-check/build PASS。
- File coverpkg 62.2%；Race 保持 NOT AVAILABLE，compatible environment/CI PENDING。
- Implementation commit/push：`5142da9 feat(file): add secure streaming file service`，SUCCESS。
- `FILE BACKEND V1 = FUNCTIONAL`；未进入 File Frontend、Chat/Group 文件消息、Admin File、S3/OSS 或 Day 4。

## Checkpoint 4 — Group Frontend Integration

- 时间：2026-10-13 07:50 +08:00。
- 新增独立 Group types/API/socket/store/reconcile/components 与 `/group/:groupId?`，Chat/Group 保持两条 singleton socket 和两套 store。
- Group socket 具备 typed protocol、单 reconnect timer、30 秒上限退避、认证终止与 logout cleanup；两条 socket 统一 auth invalidation。
- stable clientMsgId、pending/sent/failed、ACK/message 双事件顺序、duplicate、lost ACK retry 与 history overlap 均正确合并。
- Group Sessions 与 My Groups 分离；realtime preview/order/unread、本地 active/background read、服务端 session reconciliation 与多设备 persisted read 完成。
- 群 info/member cache/minimal create/mute UX 完成；text-only，无伪造文件能力；HTML-like text 只作文本渲染。
- Web `type-check`、12 files / 77 tests、production build PASS；lint NOT CONFIGURED。
- 真实 Chrome A/B/C：pending→sent、三成员 realtime/unread/read、刷新、mute/unmute、被禁言仍接收、HTML 安全、B1/B2 exact-once/shared read、20+ cursor history、Group restart reconnect、Chat unaffected、logout/auth revoked 全部 PASS。
- Group + Private Chat 真实 MySQL/Redis focused regression、`go test ./... -count=1`、vet/gofmt/diff/Secret Scan PASS；Race 保持 NOT AVAILABLE（32-bit MinGW）。
- 临时浏览器/服务/etcd/测试用户与群数据已清理；数据库测试用户与群计数均为 0。
- `GROUP CHAT V1 = FUNCTIONAL`（text scope）；未进入 File、Admin 或 Day 4。

## Git — Checkpoint 4

- Implementation commit：`0a17af6 feat(web): integrate reliable group chat`，已推送 `origin/main`。
- Log/status commit：本记录所在提交，完成后推送并复核 equality/clean。

## Checkpoint 3 — Group Message Reliability + Session / Unread / History

- 时间：2026-10-12 22:32 +08:00。
- 启用 typed text `group.send/group.ack/group.message/error`；可信 Client sender、active/member/all-mute/personal-mute validation。
- message + transaction-time members sessions/unread 同一 MySQL transaction；50 duplicate 收敛一次，50 distinct 无丢增量。
- ACK 为 commit 后 originating-client persist ACK；独立 `gim:group:delivery` 完成跨实例/多设备 fanout，Redis failure 不撤销 ACK。
- Group session JOIN、member-only message-ID cursor history、per-user deletion、monotonic partial/latest/stale Mark Read 完成。
- message/session failure injection rollback、Redis post-commit degraded、Presence negative、EXPLAIN PASS。
- 真实 MySQL/Redis actual WS integration 连续 3 轮、全量真实依赖 tests、Private Chat regression、vet/gofmt/diff/Secret Scan PASS；focused coverage 69.1%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI PENDING。

## Git — Checkpoint 3

- Implementation commit：`547afe835f84760adf9733f1661050706ef8b32f feat(group): add reliable group messaging and sessions`。
- Log/status commit：本记录所在提交。
- Push：implementation 与本 log/status commit 已推送 `origin/main`；最终 `HEAD == origin/main`、working tree clean。

## Checkpoint 2 — Group WebSocket Foundation

- 时间：2026-10-12 16:26 +08:00。
- 独立 Group Hub/Client event loop，用户级多设备、readPump/唯一 writePump、bounded queue/slow removal 与 graceful shutdown。
- 第一版即具备 1 MiB read limit、60s Pong wait、50s Ping、10s write deadline；peer Ping 的 Pong 也经 writePump。
- endpoint `/api/group/ws/chat?token=JWT` 经 Gateway/Auth；Origin allowlist、可信 Header 覆盖、missing/revoked JWT 均真实验证。
- typed malformed/unsupported error foundation；`group.send` 明确未实现，无消息/ACK/clientMsgId/session/unread/history。
- Group WS 无 Presence dependency；真实 Redis connect/disconnect/timeout/slow/shutdown negative test PASS。
- Group API foundation 与 Private Chat focused/full regression PASS；Group WS 10 轮 stress PASS，汇总 coverage 77.3%。
- 正式 Group API runtime 的 etcd lease、401 direct boundary、Ctrl+C Hub/registration/listener cleanup PASS。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Chat + Group compatible CI verification PENDING。

## Git — Checkpoint 1

- Implementation commit：`860ba8eb16dac77ad751fecb0c62d9e509f74cb9 feat(group): add group domain and membership foundation`。
- Log/status commit：本记录所在提交。
- Push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并在本轮最终复核。

## Git — Checkpoint 2

- Implementation commit：`c3bb003aeb12ed7bdc06a88fea118ee3790316dd feat(group): add websocket connection foundation`。
- Log/status commit：本记录所在提交。
- Push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并执行最终等值/clean 复核。

## Day 3 最终冻结

- 六个 Checkpoint 全部完成，Day 3 总验收通过。
- 下一计划为 Day 4 final engineering / deployment / Admin / logging / performance，但尚未授权。
- 严格不继续 Chat/Group 文件消息、Admin File/Chat、Kafka、Docker Compose、S3/OSS 或 Day 4。
