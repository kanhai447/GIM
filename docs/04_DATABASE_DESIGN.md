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

## 7. Checkpoint 7 正式 Schema（取代前述草案中的冲突项）

可执行真相源是 `server/migrations/*.up.sql` 与配对的 `*.down.sql`；应用代码禁止使用 AutoMigrate 改写生产结构。全部表使用 MySQL 8、InnoDB、`utf8mb4_0900_ai_ci`，业务时间使用 `DATETIME(3)`，JSON 数据使用 MySQL `JSON`。

### 7.1 迁移版本与运行策略

迁移版本严格连续：`001_identity`、`002_friend`、`003_chat`、`004_group`、`005_file`、`006_settings`。`schema_migrations` 记录版本、名称、应用时间和 `dirty` 状态。

MySQL DDL 会隐式提交，无法承诺跨多个 DDL 的事务原子性。Runner 在执行前先写 dirty 状态，成功后清除；失败时后续 UP 会 fail-fast。操作员应检查失败点和备份，再运行已审阅的对应 DOWN，修复原因后重试，禁止直接删除版本记录。部署顺序为备份与兼容性确认、UP、status/健康检查；生产回滚必须先恢复兼容应用并确认可验证备份。

```text
cd server
go run ./cmd/migrate -env ../.env.local -command status
go run ./cmd/migrate -env ../.env.local -command up
go run ./cmd/migrate -env ../.env.local -command down -steps 1
```

### 7.2 跨服务引用与一致性

V1 统一不创建数据库外键。各表属于不同逻辑服务边界，关系完整性由服务层存在性校验、事务以及 `NOT NULL`、`UNIQUE`、`CHECK` 和索引保证，避免跨域删除耦合并为后续拆库保留边界。默认不全表软删除；消息“仅自己删除”和会话隐藏使用用户维度删除表/`hidden_at`，撤回是不同业务状态。

### 7.3 Identity

- `users`：`id, account, pwd_hash, nickname, abstract, avatar, ip, addr, role, status, open_id, register_source, created_at, updated_at`。`account` 和可空 `open_id` 唯一；role 为 1 管理员/2 普通用户，status 为 1 启用/2 禁用。只保存 Auth 强 Hash，不存在明文口令列。
- `user_confs`：每用户唯一；保存撤回提示、好友上线、声音、安全链接、记住口令偏好、搜索/验证策略、验证问题 JSON，以及限制聊天/加好友/建群/群聊开关。Presence 真相不在此表。

### 7.4 Friend

- `friends` 正式采用 A→B、B→A 双行：`id, user_id, friend_id, remark, created_at, updated_at`，唯一 `(user_id, friend_id)` 且禁止自好友。接受申请须在同一服务事务写双向行并更新申请。
- `friend_verifies` 状态固定为 0 pending、1 accepted、2 rejected。pending 时生成规范化 low/high user ID，唯一键阻止同一对用户反向或并发重复申请；已处理历史不受该键限制。

### 7.5 Private Chat

- `chat_messages` 以 `(send_user_id, client_msg_id)` 保证幂等，生成规范化会话双方 ID；`(conversation_low_id, conversation_high_id, id)` 支持双向会话游标历史。
- `chat_sessions` 每用户/好友唯一，保存 last message、last read、unread、置顶和 `hidden_at`；排序索引为 `(user_id, is_top DESC, last_message_at DESC, id DESC)`。
- `chat_message_deletions` 主键 `(user_id, message_id)`，仅表达该用户隐藏一条历史，不等于全局撤回。

### 7.6 Group

- `groups` 保存 owner、资料、公告、启停、搜索/入群策略、验证问题、成员邀请/临时会话/全员禁言开关与人数上限。
- `group_members` 以 `(group_id, user_id)` 唯一，role 为 1 owner、2 admin、3 member，并分别支持群成员和“我的群”查询。
- `group_verifies` 状态为 0 pending、1 accepted、2 rejected，请求类型为 1 主动申请、2 成员邀请；pending 生成列唯一键防并发重复。
- `group_messages` 保留发送时 `group_member_id`，以 `(send_user_id, client_msg_id)` 保证幂等，以 `(group_id, id)` 支持历史游标。
- `group_sessions` 每用户/群唯一；`group_message_deletions` 保存用户维度删除。私聊与群聊继续分离。
- 临时群禁言只存 Redis TTL，不在 MySQL 建第二份过期状态。

### 7.7 File 与 Settings

- `file_objects` 一份物理内容一行：UUID 唯一、SHA-256 唯一，并保存 size、storage path、MIME。V1 不使用 MD5 去重。
- `user_files` 表示用户对物理对象的引用，唯一 `(user_id, file_object_id, original_name)`。秒传复用物理对象，但不复用其他用户的所有权记录。
- `settings` 使用唯一 key、类型安全的 JSON value、公开标记和描述。基础设施地址、凭据、JWT 签名材料等 Secret 禁止进入该表或公开 API。

### 7.8 关键事务与验收

- 注册：Auth Hash 后调用 User；实现默认配置写入时，用户与配置必须原子创建。
- 接受好友：锁定 pending 申请，写双向关系并更新状态后提交。
- 发送消息：幂等插入消息并更新相关 session；唯一冲突时读取原消息返回一致结果。
- 历史/会话使用 message ID 与排序字段游标，不依赖 OFFSET 深分页。
- 文件入库以 SHA-256 竞争唯一键，复用物理对象后创建用户引用。

真实 MySQL 验收从空库 UP，检查表/列/约束/索引与重复写入，执行关键查询 EXPLAIN，在迁移后的 Repository 上回归 User/Auth，再执行 DOWN→UP。
