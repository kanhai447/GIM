# 003 / Task 3 — “拒绝后再申请”语义

- **日期**：2026-11-11（Asia/Shanghai）
- **开始状态**：`HEAD = 14aebef`（已推送），working tree clean。
- **结束状态**：本任务提交 `test(user): cover reject-then-reapply ...`。

## 目标

明确“好友申请被拒绝后能否再次申请”的产品语义：审查现有事务/唯一键，用真实服务链路取证，补测试与文档；若需改 schema 或状态机则先停下询问。

## 当前实现审查

- `server/internal/user/friend/repository/mysql/repository.go`
  - `CreateRequest`：事务内先查双向 `friends`（存在 → `ErrAlreadyFriend`），再 `INSERT friend_verifies(status=0)`；1062 → `ErrDuplicateRequest`。
  - `Respond`：行锁申请 → 校验接收者 → 校验 `status=0` → 同意时写双向 `friends`，最后更新 `status`（1 或 2）。
  - 没有“rejected 后禁止申请”的额外判断。
- `server/migrations/002_friend.up.sql`
  - `pending_low_id/pending_high_id` 为生成列：`CASE WHEN status = 0 THEN ... ELSE NULL END`。
  - 唯一键 `ux_friend_verifies_pending_pair(pending_low_id, pending_high_id)`；MySQL 唯一索引允许多个 NULL。
  - 因此 processed（accepted/rejected）历史不占用 pending 唯一键。

## 真实行为（本轮用真实服务链路取证）

启动 `user-rpc + user-api + auth-api + gateway`（本机 MySQL/Redis/etcd），经 Gateway 实测：

```text
E1 apply#1 code=0 id=3 status=0
E2 apply#2-while-pending code=1116 msg=已有待处理的好友申请
E3 reject code=0 status=2
E4 apply-after-reject code=0 newRequestID=5 status=0
E5 accept#2 code=0 status=1
E6 apply-when-friends code=1114 msg=对方已经是好友
```

**结论：当前实现已经符合推荐语义** —— 拒绝后允许再次申请，历史 rejected 记录保留，同一时间只允许一条 pending；无需改代码或 schema。

## 变更

| 文件 | 变更 |
|---|---|
| `server/tests/friend_integration_test.go` | 新增断言：rejected 历史状态保持 2；拒绝后重新申请成功且 ID 不同；re-apply 后重复 pending → `1116`；接受后好友行恰好 2；已好友再申请 → `1114`；新增 `friendVerifyStatus` 辅助 |
| `docs/05_HTTP_API.md` | `POST /api/user/friends` 明确“拒绝后允许再次申请、历史保留、同时仅一条 pending” |
| `docs/07_CORE_FLOWS.md` | 好友流程补充 `Re-apply after rejection` 分支与唯一键解释 |

未改 schema、未改 API 结构、未改错误码。

## 测试（真实 MySQL）

```powershell
$env:GIM_ENV_FILE='K:\it\gim\.tmp\env.integration'
go test ./tests -run TestFriendLifecycleMySQLAndConcurrency -count=1 -v
go vet ./tests/...
```

真实结果：

```text
=== RUN   TestFriendLifecycleMySQLAndConcurrency
--- PASS: TestFriendLifecycleMySQLAndConcurrency (1.18s)
PASS
ok  github.com/kanhai447/GIM/server/tests 1.747s
VET=0
```

该用例同时覆盖本轮要求的三种场景：拒绝后再次申请（成功）、重复 pending（`1116`）、已好友申请（`1114`）。

## 问题与修复

- 无代码缺陷：现有实现已符合“拒绝后允许再次申请、同时只允许一条 pending”的推荐语义，未改 schema/状态机。
- 文档原先未写明该语义；本轮补入 docs/05 与 docs/07。

## Git

- commit：本记录所在提交（`test(user): cover reject-then-reapply ...`）。
- push：随本轮后续提交统一推送（见任务四日志与最终汇总）。

## 下一步

Task 4：env 清理、group api 残留核查、错误码 1113–1119 文档确认。
