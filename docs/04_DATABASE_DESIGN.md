# 04 数据库详细设计

以下是逻辑模型。Agent 应生成可重复执行的 migration；不要仅依赖开发环境 AutoMigrate。

## 1. 通用约定

- MySQL 8.x，utf8mb4。
- 主键使用 BIGINT UNSIGNED AUTO_INCREMENT（若保留 GORM uint，注意 64 位环境）。
- 时间：`created_at`, `updated_at`；需要软删除时单独说明，不默认全表软删。
- JSON 字段使用 MySQL JSON 或 GORM serializer，不能把任意 Go struct 无约束塞 TEXT 后再猜结构。

## 2. 用户相关

### users

`id, account, pwd_hash, nickname, abstract, avatar, ip, addr, role, status, open_id, register_source, created_at, updated_at`

约束/索引：`account UNIQUE`；`open_id`（若第三方登录启用）；昵称搜索按实际查询决定普通索引/前缀搜索限制。`pwd_hash` 只存 Auth 生成的强哈希，不存明文；`role` 使用 `1=管理员、2=普通用户`，`status` 使用 `1=启用、2=禁用`。Checkpoint 3 只建立模型与 Repository 契约，正式 migration 在 Day 1 Checkpoint 7 集中创建。

### user_confs

`id, user_id UNIQUE, recall_message, friend_online, sound, secure_link, save_pwd, search_user, verification, verification_question JSON, curtail_chat, curtail_add_user, curtail_create_group, curtail_in_group_chat, created_at, updated_at`

不要把 DB `online` 当 Presence 真相；在线状态来自 Redis/Chat Hub。

### friends

保留参考项目双向关系一行模型：`send_user_id, rev_user_id, send_user_notice, rev_user_notice`。

索引：`send_user_id`, `rev_user_id`, `(send_user_id, rev_user_id)`。Service 必须防止反向重复关系和并发重复创建。

### friend_verifies

保留 reference 字段。索引：`send_user_id`, `rev_user_id`, `status`, `created_at`。

## 3. 私聊消息

### chat_messages（可沿用 chat_models 表名）

| 字段 | 类型/约束 |
|---|---|
| id | PK |
| send_user_id | NOT NULL |
| rev_user_id | NOT NULL |
| client_msg_id | VARCHAR(64) NOT NULL |
| msg_type | TINYINT NOT NULL |
| msg_preview | VARCHAR(255) |
| msg | JSON NOT NULL |
| system_msg | JSON NULL |
| created_at/updated_at | timestamp/datetime |

关键约束：`UNIQUE(send_user_id, client_msg_id)`。

索引：

- `(send_user_id, rev_user_id, id)`
- `(rev_user_id, send_user_id, id)`
- `created_at`

### chat_sessions

每个用户对每个好友一行：

`id, user_id, peer_user_id, last_message_id, last_message_at, last_read_message_id, unread_count, is_top, created_at, updated_at`

约束：`UNIQUE(user_id, peer_user_id)`。

发送 A->B：更新 A->B 和 B->A 两行；B unread +1（如果未处于已读状态）；A 不增加自己的 unread。

原 `top_user_model` 可在迁移后由 session.is_top 取代；V1 兼容期间可双写，最终只保留一个真相源。

原 `user_chat_delete_model` 可先保留，实现“仅自己删除某条历史”。不要把它和撤回混淆。

## 4. 群

### groups / group_members / group_verifies

字段基本沿用参考项目。关键约束：`UNIQUE(group_id, user_id)` on group_members。索引：`group_id`, `user_id`, `role`。

### group_messages

新增 `client_msg_id VARCHAR(64) NOT NULL`；约束 `UNIQUE(send_user_id, client_msg_id)`；索引 `(group_id, id)`, `(send_user_id, id)`。

保留 `group_member_id` 以记录发送时成员身份。

### group_sessions

`id, user_id, group_id, last_message_id, last_message_at, last_read_message_id, unread_count, is_top, created_at, updated_at`

约束：`UNIQUE(user_id, group_id)`。

原 group top 表可迁移到 `is_top`，同样避免长期双真相。

## 5. 文件

### file_objects

`id, uid UUID UNIQUE, sha256 CHAR(64) UNIQUE, size BIGINT, path VARCHAR(512), mime_type VARCHAR(128), created_at, updated_at`

一份物理内容对应一行。

### user_files

`id, user_id, file_object_id, file_name, created_at, updated_at`

索引：`user_id`, `file_object_id`；可根据产品语义设置 `UNIQUE(user_id, file_object_id, file_name)` 或允许同名多记录。

文件秒传：先根据 SHA-256 找 file_object；存在则不重复写物理文件，只创建 user_files 关系。

## 6. 消息 ID 与分页

数据库自增 ID 作为 V1 服务端 messageId。历史记录使用游标优先：`beforeId`/`afterId`；为兼容原页面可暂留 page/limit，但 Agent 不应新增依赖 OFFSET 深分页的核心逻辑。
