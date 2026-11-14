# 004 / Task 4 — 修正 A8/A9/A11

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：migration README、docs/04 系统表与列名已按 001–008 对齐；未改 migration。

## A8：migrations README

- 修改：`Current released sequence ends at 007_audit_logs` → `008_admin_management`；补充“008 只给 chat_messages/group_messages 加 admin_deleted_at，不新增索引”。

## A9：schema_migrations

- `docs/04` §7.1 增加 `schema_migrations` 表说明：`version BIGINT UNSIGNED PK, name VARCHAR(128), dirty TINYINT(1), applied_at DATETIME(3)`；runner 维护，业务代码不读写。
- §7.11 索引表增加该表一行（PK version，CHECK dirty，migration=runner）。

## A11：docs/04 与 migration 对齐（D1–D9）

| 项 | 修正 |
|---|---|
| D1 | `user_confs` 列改为 `add_user/create_group/in_group_chat` |
| D2 | `friend_verifies.message` 改为 `VARCHAR(128)` |
| D3 | `file_objects.storage_path`、`user_files.original_name` |
| D4 | `friends` 改为 `user_id/friend_id/remark` 双行 + 实际索引 |
| D5 | 注册改为“只写 users，不初始化 user_confs；CanChat COALESCE 容忍” |
| D6 | `chat_message_deletions` 标注“生产代码只读，隐藏走 hidden_at” |
| D7 | 索引表核对一致；补 schema_migrations |
| D8 | `chat_sessions.hidden_at` 已在 §7.5；无需改 |
| D9 | README 到 008（A8） |

docs/04 顶部增加“以实现为准，核对日期 2026-11-14”。

## 验收

- 未修改 `server/migrations/*.sql`；未发现 migration 缺索引/列类型错误。
- `docs/04` 与 001–008 的列名/索引/约束一致；schema_migrations 已列出。

## Git

- commit：`docs: align database and migration docs`（本记录所在提交）。
