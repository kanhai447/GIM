# Day 3

## 授权状态

- Day 3：IN PROGRESS。
- Day 3 / Checkpoint 1：COMPLETED — Group Domain + Membership + API Foundation。
- Day 3 / Checkpoint 2：COMPLETED — Group WebSocket Foundation。
- Day 3 / Checkpoint 3：COMPLETED — Group Message Reliability + Session/Unread/History。

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

## 下一入口

- Group Frontend Integration（需用户另行授权）。
- 严格不继续 Group Frontend、File、Admin 或 Day 4。
