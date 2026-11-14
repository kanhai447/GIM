# 002 / Task 1B — 收紧授权查询（LIMIT 1）+ 语义测试

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = c590f9c`（审计后）。
- **结果**：唯一差异修复完成；授权语义专项测试 **PASS**。

## 修改

`server/internal/file/repository/mysql/repository.go` 的 `canAccessImage`：

- 原实现：`SELECT COUNT(*)` 统计全部命中行（私聊/群聊各一次）。
- 新实现：`SELECT 1 ... LIMIT 1`，命中即停；授权条件、表、JSON 路径、tombstone/群成员过滤全部不变。
- 无 schema 变更、无新索引、无 migration。

## 授权语义测试（真实 MySQL）

新增 `server/tests/file_authorization_integration_test.go`：

| 场景 | 断言 |
|---|---|
| 所有者 | 允许 |
| 私聊接收方（消息 rev_user） | 允许 |
| 私聊发送者（消息 send_user） | 由所有者/发送者路径覆盖 |
| 非参与者 | `2503` |
| 群成员 | 允许 |
| 群外用户 | `2503` |
| 私聊 tombstone 后 | 接收方 `2503`；所有者仍允许 |
| 群消息 tombstone 后 | 群成员 `2503`；所有者仍允许 |
| 退群后 | 原成员 `2503` |
| 构造未知 fileID | `2503`（与“存在但无权”一致） |

测试使用真实 schema（001–008 migration）和真实 `user_files`/`file_objects`/`chat_messages`/`group_messages`/`group_members` 行。

## 真实结果

```text
=== RUN   TestImageDownloadAuthorizationSemantics
--- PASS: TestImageDownloadAuthorizationSemantics (1.31s)
```

`go build ./...`、`go vet ./...`、`gofmt -l .` 均 PASS。

## 文本消息路径

- 前端 `MessageBubble` 只对 `type=2` 调用 `fileApi.download`；该断言在任务二测试提交中补充。
- 后端授权 SQL 的 `JSON_EXTRACT(msg,'$.imageMsg.fileID')` 在文本消息上返回 NULL，不会命中；专项测试在任务二提交中覆盖“文本内容提及 fileID 不授权”。

## 明确不做

- 不新增 JSON 索引/migration（先报告原则）；无速率限制（记录为工程限制）。
- 不改消息/文件表结构，不改 Hub/广播/session。

## Git

- commit：`fix(file): tighten image download authorization`（本记录所在提交）。
