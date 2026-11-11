# 001 / Task 1 — 修复 Admin 私聊删除的并发正确性问题（C1 + C2）

- **日期**：2026-11-11（Asia/Shanghai）
- **授权范围**：仅修复审计报告 C1、C2；不改动 Hub/Client、幂等、session 写路径、文件上传、审计脱敏。
- **开始状态**：branch `main`，HEAD `09994cb`，working tree clean。
- **结束状态**：HEAD `05dc348`，working tree clean（本日志所在提交另计）。

## 目标

1. **C1**：`repairSession` 在 session 行锁返回非 `RecordNotFound` 错误时静默返回成功，导致 tombstone 已提交但 session 指针未修复。
2. **C2**：admin 删除消息的加锁顺序与发消息不一致（sender→receiver vs user_id 升序），并发下可能死锁；admin delete 无重试。

## 修改文件

| 文件 | 变更 |
|---|---|
| `server/internal/chat/message/admin/mysql/repository.go` | C1 错误处理顺序；C2 `sessionLockOrder`、`retryDeadlock`、`deadlockBackoff`、`isDeadlock`；`Delete` 拆为 retry 包装 + `deleteOnce` |
| `server/internal/chat/message/admin/mysql/repository_test.go` | 新增 6 组单元测试（C1 回归、死锁分类、锁顺序、重试上限/成功/非 1213/取消、退避） |
| `server/tests/admin_integration_test.go` | 新增 gated 断言 `assertChatSessionsRepaired`：删除最后一条消息后两条 session 指针清空、unread=0、hidden |

## 关键操作与结果

### 1. C1 失败测试先行（修复前）

```powershell
go test ./internal/chat/message/admin/mysql/ -run TestRepairSessionPropagatesSessionLockFailure -count=1 -v
```

注入一个所有语句都失败的 `gorm.ConnPool`（`failingPool`），使 `chat_sessions ... FOR UPDATE` 返回非 `RecordNotFound` 错误。

修复前真实输出（节选）：

```text
=== RUN   TestRepairSessionPropagatesSessionLockFailure
... SELECT id, last_message_id, last_read_message_id FROM `chat_sessions` ... FOR UPDATE
    repository_test.go:57: repairSession swallowed a session lock failure
--- FAIL: TestRepairSessionPropagatesSessionLockFailure
```

### 2. C1 根因与修复

根因：`Take` 失败后 `session` 为零值，`session.LastMessageID == nil` 先命中提前返回；其后的 `if err != nil` 分支不可达。

修复（最小改动，仅调整判断顺序与拆出错误分支）：

```go
if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
    return fmt.Errorf("lock chat session after admin delete: %w", err)
}
if errors.Is(err, gorm.ErrRecordNotFound) || session.LastMessageID == nil || *session.LastMessageID != deletedID {
    return nil
}
```

修复后同一测试：`--- PASS`。

### 3. C2 实现

- `sessionLockOrder(senderID, receiverID)` 固定按 user_id 升序返回两条 session key，与 `chat/message/repository/mysql` 的 `operations[0].UserID <= operations[1].UserID` 排序一致。
- `Delete` 改为 `retryDeadlock` 包装 `deleteOnce`：仅对 MySQL **1213** 重建整个事务，最多 3 次，退避 10/20 ms，退避期间监听 `ctx.Done()`；其他错误立即返回，不伪装成功。

### 4. 测试结果

```text
go test ./internal/chat/message/admin/mysql/ -count=1 -v
--- PASS: TestRepairSessionPropagatesSessionLockFailure
--- PASS: TestIsDeadlockOnlyMatchesMySQL1213
--- PASS: TestSessionLockOrderMatchesSenderOrdering
--- PASS: TestRetryDeadlockIsBoundedAndContextAware (4 subtests)
--- PASS: TestDeadlockBackoffIsBounded
ok  github.com/kanhai447/GIM/server/internal/chat/message/admin/mysql
```

```text
go build ./...   -> PASS
go vet ./...     -> PASS
gofmt -l server  -> 无输出
go test ./internal/chat/... ./tests -count=1 -> 全部 ok（server/tests 集成用例因未设 GIM_ENV_FILE 按设计 skip）
```

### 5. 未能验证的部分（如实记录）

- **真实 MySQL 死锁复现未执行**：本机 Docker daemon 未运行，`3306/6379/2379/9092` 无监听，无法制造真实 1213。改为“锁顺序一致性 + 有界重试”单元测试覆盖（用户已允许的降级路径）。
- **`go test -race` 未执行**：Windows native race 在本机因 32-bit MinGW 不可用（`cc1.exe: unimplemented: 64-bit mode`），Linux Docker 不可用；本轮不声称 race PASS。
- **`assertChatSessionsRepaired`** 已加入 `server/tests`，需 `GIM_ENV_FILE` + MySQL 才会执行；本轮跳过。

## Git

- commit：`05dc34836a324c396fb3cb39e13199d806c45389 fix(chat): repair admin chat delete error handling and lock order`
- push：**成功**。`git push origin main` → `09994cb..05dc348  main -> main`。此前 `git ls-remote` 曾返回 `Recv failure: Connection was reset`（瞬时网络故障），随后 push 正常完成；未使用 `--force`。
- tag：无（本轮不打稳定 tag）。

## 遗留

- C1 的正常路径验证依赖 `server/tests/admin_integration_test.go`，需在可用 MySQL 环境执行。
- 群消息 admin 删除的 session 修复已正确返回错误（`group/admin/mysql/repository.go:155`），本轮未改动。

## 下一步

进入 Task 2：好友闭环（后端 8 条路由 + 前端 UI）。
