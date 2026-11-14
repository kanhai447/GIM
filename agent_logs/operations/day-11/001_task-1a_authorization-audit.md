# 001 / Task 1A — 文件下载授权审计（只读）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = c590f9c`，工作区干净。
- **结论**：tombstone / 退群 / 所有者语义已满足推荐要求；唯一差异是授权查询使用 `COUNT(*)`、未带 `LIMIT 1`，本轮做最小修改（见 002）。

## 审计对象

- `server/internal/file/repository/mysql/repository.go`：`FindAccessibleByID` / `canAccessImage`（a1ecf64 引入）。
- `server/internal/file/service/service.go`：`GetAccessible` / `mapOwned`。
- `server/internal/file/transport/http/handler.go`：`Download`。
- `server/internal/group/repository/mysql/repository.go`：`RemoveMember`（退群/踢人）。
- `server/internal/chat/message/admin/mysql/repository.go`、`server/internal/group/admin/mysql/repository.go`：tombstone 写入。

## 审计结论（逐条）

| 问题 | 结论 |
|---|---|
| 所有者 | 先按 `user_files.id` 读取记录；`uf.user_id == 当前用户` 直接放行，不查消息表 |
| 私聊接收方 | 条件为 `admin_deleted_at IS NULL AND (send_user_id = 当前用户 OR rev_user_id = 当前用户)`，并用 JSON 路径匹配 `msg.imageMsg.fileID` |
| 私聊 tombstone | 已排除：`admin_deleted_at IS NULL` |
| 群成员 | `group_messages gm JOIN group_members member ON member.group_id = gm.group_id AND member.user_id = 当前用户`，且 `gm.admin_deleted_at IS NULL` |
| 退群/被踢 | `LeaveGroup`/`KickMember` 调用 `RemoveMember`，物理删除 `group_members` 行；授权 JOIN 不再命中 → 拒绝 |
| 群 tombstone | 已排除：`gm.admin_deleted_at IS NULL` |
| 非参与者 | 两个查询都不命中 → `ErrNotFound` → service 统一 `2503`；不存在文件同样 `2503` |
| 文本消息 | 前端只对 `type=2` 调用下载；SQL 的 `JSON_EXTRACT(msg,'$.imageMsg.fileID')` 在文本消息上返回 NULL，不会命中授权 |
| fileID 探测 | 任何人可构造数字 fileID 请求，但响应统一 `2503`，不区分“存在但无权”与“不存在”；无速率限制 |
| LIMIT 1 | **不满足**：使用 `COUNT(*)`，会统计全部命中行；计划改为 `SELECT 1 ... LIMIT 1` |
| JSON 索引 | `JSON_EXTRACT` 无索引；仅非所有者下载时执行，当前可接受；后续可加生成列/索引（本轮不加 migration） |

## 与推荐语义的差异

1. 查询未带 `LIMIT 1`（性能/审计要求）→ 最小修改为 `SELECT 1 … LIMIT 1`，不改变授权结果。
2. 无速率限制 → 记录为工程限制，不在本轮实现。
3. 撤回未实现 → 文件授权语义待 V2 定义，记入 known-issues。

## 无 schema 变更

- 只读 `user_files`、`file_objects`、`chat_messages`、`group_messages`、`group_members`；不新增表/列/索引。

## 验收

- 审计结论已写入本日志；下一步做最小修改 + 专项测试。
