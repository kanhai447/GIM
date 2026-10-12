# Day 3 / Checkpoint 1 — Group Domain + Membership + API Foundation

- **时间:** 2026-10-12 10:33 +08:00
- **开始状态:** `main`，`HEAD == origin/main == e2f7204b12cf528577550f4952b2acd9c6f11c2e`，working tree clean。
- **授权范围:** Group Domain、Repository、Service、Membership/Policy、基础 HTTP/RPC、Redis TTL 禁言；明确不实现 Group WebSocket、群消息、ACK、group session/unread、Group 前端和 File。

## Group Domain

- 新增独立 `Group`、`GroupMember`、`JoinRequest`、`GroupStatus`、`GroupRole`、`JoinPolicy` 类型。
- 固定角色语义：`1=owner`、`2=admin`、`3=member`；申请状态：`0=pending`、`1=accepted`、`2=rejected`，业务层不使用散落魔法数字。
- 复用 Day 1 `004_group` schema；未修改 001–006 migration，未新增 migration。
- 解散群使用既有 `groups.status=2` 并在同一事务移除 membership；后续 membership/WS 查询将已解散群视为不存在。

## Membership

- `CreateGroup` 只使用 Gateway 可信 `User-ID` 作为 owner；客户端 `ownerId/operatorId/userId` 不能决定操作者。
- Repository 提供 Create/Get/Add/Remove/List/Count/UpdateRole/MyGroups 与 Join Request 能力；`UNIQUE(group_id,user_id)` 是并发重复成员的最终保障。
- 创建群同时写 owner membership；创建时邀请成员去重并批量校验用户。
- 开放群加入在一个事务中写 accepted verify + member；审批模式创建唯一 pending request。
- approve/reject 先锁定 pending request；approve 在一个事务中插入 member 并转 accepted，reject 只允许 pending -> rejected。
- 普通成员可以退出，owner 普通退出返回 `2310`；owner 可踢 admin/member，admin 只能踢 member，普通成员不能踢人。
- owner 可在 admin/member 之间调整角色；不能修改 owner。
- 邀请能力遵循 `allow_member_invite`；owner/admin 始终可邀请，普通 member 仅在群允许时可邀请。

## Transactions

- Create：`groups + owner/invited group_members` 同事务；注入 group_members 写失败后 group 行为 0。
- Approve：`pending request + member` 同事务；注入 member 写失败后 request 保持 pending 且 member 为 0。
- Open Join：accepted request + member 同事务。
- Dissolve：active -> dissolved + 删除 membership 同事务。
- 所有事务内再次锁定/检查 group active 与 member limit，避免先验校验后的并发状态变化。

## Permission Policy

- `canManageJoin/canInvite/canActOn` 集中表达 owner/admin/member 权限，不在 Handler 散落角色判断。
- 群详情与成员列表仅允许当前群成员读取。
- HTTP Handler 严格解析 Gateway 注入的 `User-ID/Role`；未知 identity payload 字段被拒绝。
- 稳定错误码覆盖 group/member/request/permission/duplicate/state/user/owner/mute/limit，MySQL/Redis/RPC 原始错误不进入 HTTP/RPC 响应。

## Mute

- 临时个人禁言只使用 Redis TTL，key 由 `rediskeys.GroupProhibition(memberId)` 生成：`gim:group:prohibition:{memberId}`。
- membership ID 对 `(group_id,user_id)` 唯一，因此不同群/成员隔离；支持 mute、unmute、IsMuted、自然过期与 Redis 有界超时。
- Redis 不可用返回安全稳定的 `2311/503`，不泄露 endpoint/raw error。
- 测试比较 `gim:presence:*` 前后集合一致；Group 代码没有 MarkOnline/MarkOffline/Presence 写入。

## Query / N+1

- `MyGroups` 由 `group_members -> groups` JOIN + group member count 聚合一次返回，基于当前 user_id，不读取全部群后过滤。
- Member List 一次查询 membership，再使用新增的 User `BatchGetUsers` RPC 一次补齐公开资料；测试断言 batch RPC 恰好调用一次。
- User RPC batch 去重最多 500 个 ID，只返回公开 `UserInfo`，不返回 password hash。
- EXPLAIN 确认：membership lookup 使用/暴露 `ux_group_members_group_user`，member list 使用 `ix_group_members_group_role_user`，my groups 使用 `ix_group_members_user_group`，pending request 使用 `ix_group_verifies_group_status_created`；无需新增索引。

## API / RPC

- HTTP：create/info/dissolve/my groups/members/invite/leave-or-kick/role/apply/review/mute-unmute。
- Internal Group RPC：仅 `GetGroup`、`IsMember`、`GetMemberRole`、`IsMuted`。
- User RPC：新增通用 `BatchGetUsers`，用于成员列表消除 N+1。
- 新增正式 `cmd/group-api`、`cmd/group-rpc`，Group API 使用 etcd 注册；两者复用 MySQL/Redis/User RPC 和 graceful shutdown 基础设施。
- 无 Group WS route、Hub、Client、readPump/writePump、message insert、ACK、session/unread/history。

## Reference / Independent Implementation

- **FIM referenced:** 群创建、成员角色、邀请/申请、审批、退出/踢人、禁言和 Group RPC/API 的行为与字段语义；Web 参考了 `group_api.ts` 的既有请求路径/字段。
- **GIM independently implemented:** 新的 domain/repository/service/policy/transport 分层、GORM transaction repository、批量 User RPC、go-zero commands 和测试均在 GIM 新目录自主编写；没有复制 FIM 文件、改包名或依赖 reference 编译。
- **GIM improvements:** FIM 创建群和批准申请存在多步部分成功风险；GIM 使用明确事务与 row lock。FIM 禁言 key 在 logic 中字符串拼接并同时保留 DB 临时字段；GIM 使用集中 key helper + Redis TTL 单一真相。FIM 权限判断分散；GIM 集中 policy。FIM member list 依赖多类查询；GIM membership + batch RPC 无逐成员 N+1。所有 DB/RPC/Redis 使用调用方 context。

## Tests

- `go test ./... -count=1`（真实 MySQL/Redis/隔离 etcd）：PASS，包含 Private Chat 全量回归。
- `go test ./tests -run '^TestGroupFoundationMySQLRedisTransactionsAndConcurrency$' -count=3`：PASS。
- `go test ./tests -run '^(TestChatMessageMySQLAndWebSocketReliability|TestChatSessionUnreadReadAndHistoryMySQL)$' -count=1`：PASS。
- Group integration：真实 MySQL create/owner/duplicate member/member list/my groups/apply/duplicate pending/approve/reject/open join/invite/leave/kick/role/dissolve、transaction injection、concurrent approve/add member、EXPLAIN 全部 PASS。
- Redis integration：mute TTL、IsMuted、unmute、expiry、same-user cross-group isolation、unavailable safe error、Presence untouched、cleanup 全部 PASS。
- Group coverpkg：67.9%；MySQL Repository：77.4%；Service：67.0%。
- `go vet ./...`：PASS；`gofmt -l .`：PASS；`git diff --check`：PASS。
- Repository safety + tracked/candidate private-key/credential pattern scan：PASS。
- 测试临时数据库、Redis mute key、etcd 进程与 `K:\it\gim\.tmp\etcd-day3-cp1` 已清理；2379/2380 无残留监听。

## Private Chat Regression

- Private Chat message reliability、session/unread/read/history focused integration：PASS。
- 全量 `go test ./... -count=1` 中 Chat Hub/heartbeat/Presence/message/session/Gateway tunnel 与前序测试全部 PASS。
- `PRIVATE CHAT V1 = COMPLETED / REGRESSION PASS`。

## Race

- `Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`。
- 原因仍为本机 MinGW 32-bit GCC 不支持 windows/amd64 race；本 Checkpoint 未新增共享内存并发结构，依靠 MySQL row lock/unique constraint 处理并发。
- `Race Verification Pending` compatible environment/CI。

## Files

- Group：`server/api/group/v1/`、`server/internal/group/`、`server/cmd/group-api/`、`server/cmd/group-rpc/`。
- User batch RPC：`server/api/user/v1/`、User repository/service/gRPC 与测试。
- Redis：`server/internal/platform/rediskeys/`。
- Integration：`server/tests/group_foundation_integration_test.go`。
- Config/docs：`.env.example`、`server/README.md`、`docs/02_BACKEND_DESIGN.md`、`docs/05_HTTP_API.md`、`docs/07_CORE_FLOWS.md`、`docs/08_REDIS_CONFIG.md`。
- Database migration：无。WebSocket 协议：无。前端：无。

## Git

- Implementation commit：`860ba8eb16dac77ad751fecb0c62d9e509f74cb9 feat(group): add group domain and membership foundation`。
- Log/status commit：本记录与 Day 3/CURRENT_STATE/AGENT_WORKLOG 更新所在提交。
- Push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送，最终 `HEAD == origin/main` 与 clean 状态由本轮收尾复核确认。

## Next

- Checkpoint 1 完成后严格停止。
- Next Planned：Group WebSocket Foundation；未获下一 Checkpoint 授权，不进入 Group WS/Message/Session/Frontend/File。
