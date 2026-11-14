# 001 / Task 1A — 并发撤回单元测试（K-14）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = e3e37fd`。
- **结果**：并发撤回测试 **PASS**；实际观察 1 条撤回消息（调度相关），K-14 已更新为已知边界。

## 测试

`server/internal/chat/message/service_test.go` 新增 `TestConcurrentRecallSameTargetIsSafe`：

- 两个 goroutine 同时用不同 clientMsgId 撤回同一 msgID；
- 断言无 panic、无系统错误；
- 统计实际落库的撤回消息数（不强行断言只有一条）；
- 历史查询：原消息只出现一次且已替换为 `[撤回]` 占位，撤回行数量与落库一致；
- 前端 `applyWithdrawals` 对重复撤回行合并为一条占位（`web/src/stores/chat_reconcile.test.ts`）。

真实结果：

```text
=== RUN   TestConcurrentRecallSameTargetIsSafe
    service_test.go:633: concurrent recall produced 1 withdraw rows (known boundary K-14)
--- PASS: TestConcurrentRecallSameTargetIsSafe (0.00s)
```

```text
vitest run src/stores/chat_reconcile.test.ts -> 4 tests PASS
```

## 结论

- 本次调度下两个并发请求只产生 1 条撤回消息；无唯一目标索引，其他调度/数据库时序下仍可能产生两条（K-14）。
- 历史占位唯一、前端合并去重、授权语义不受影响；不需要加列/唯一索引即可保证不重复渲染。
- 若未来要求“严格一条”，需新增唯一索引/列，属于 schema 变更，按红线不在本轮做。

## K-14 更新

`docs/known-issues.md` K-14 改为：当前为已知边界；并发可能产生两条；前端合并去重、历史占位唯一；修复需评估 schema 变更。

## 验收

- 并发撤回：**PASS（记录实际行为）**。

## Git

- commit：`test(chat): cover concurrent recall boundary`（本记录所在提交）。
