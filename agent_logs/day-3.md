# Day 3

## 授权状态

- Day 3：IN PROGRESS。
- Day 3 / Checkpoint 1：COMPLETED — Group Domain + Membership + API Foundation。
- 本轮明确未进入 Checkpoint 2。

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

## Git

- Implementation commit：`860ba8eb16dac77ad751fecb0c62d9e509f74cb9 feat(group): add group domain and membership foundation`。
- Log/status commit：本记录所在提交。
- Push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并在本轮最终复核。

## 下一入口

- Group WebSocket Foundation（需用户另行授权）。
- 严格不继续 Group Hub/Client、Group Message/ACK、Group Session/Unread、Group Frontend 或 File。
