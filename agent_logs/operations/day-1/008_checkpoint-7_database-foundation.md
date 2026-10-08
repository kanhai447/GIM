# Day 1 / Checkpoint 7 — 正式数据库 Migration + Day 1 数据底座

- **日期:** 2026-10-08
- **Phase:** Phase 1
- **目标:** 建立 GIM V1 版本化 MySQL Schema、迁移执行器、关键约束/索引与真实 UP -> DOWN -> UP 验收；不进入 Checkpoint 8 或 Day 2。

## 开始状态

- Branch：`main`。
- HEAD：`fa66ab0bac029935f87f1c8b069543b6ebb2c580`，与 `origin/main` 同步。
- Working tree：clean。
- `.env.local`、`reference/`：均未跟踪。
- Checkpoint 6：已完成并推送。

## Reference 审计

- 只读查看 FIM User/Friend/UserConf、Chat message/top/delete、Group/member/message/verify/top/delete、File、Settings model 与相关查询语义。
- 只参考其业务实体、消息类型、好友/入群验证、置顶/删除、群角色和 Settings 公开读取/Admin 修改边界。
- 不复制 FIM GORM Model、SQL、表结构或配置；不复用其 Secret、地址或 MD5/FileModel 所有权设计。
- GIM 将改进：好友状态语义、clientMsgId 幂等键、显式 chat/group session、用户维度删除、SHA-256 物理对象/用户引用分层，以及明确事务边界。

## GIM 设计决策

- Migration：`server/migrations/` 六个有序版本，每版 `.up.sql` / `.down.sql`；`schema_migrations` 记录已应用版本。
- 外键：V1 统一不创建数据库 FK。当前虽共用 MySQL，但表归属不同逻辑服务；通过 Service transaction、unique/check/not-null/index 与存在性校验保证一致性，避免跨域删除耦合并保留后续拆库边界。
- 好友关系：`friends` 使用 A -> B / B -> A 双行，每行保存当前用户备注，`UNIQUE(user_id, friend_id)`；接受好友申请必须在同一事务写入双向行并更新申请状态。
- Session：`chat_sessions` / `group_sessions` 分开；置顶、未读、last read、last message 与当前用户 `hidden_at` 均为用户维度状态。
- File：`file_objects` 以 SHA-256 去重物理内容，`user_files` 表示用户拥有/引用关系，不复用其他用户记录。
- 临时群禁言继续由 Redis TTL 管理，不在 group_members 建第二套 TTL 状态。
- 日志/Kafka 与 Mock-only dashboard 表不属于本 Checkpoint。

## 执行状态

- Migration / Runner：PASS。新增六个严格连续且成对的嵌入式 SQL 版本、`schema_migrations` dirty 状态、status/up/down/all-down 命令；正式 Schema 不依赖 AutoMigrate。
- 真实 MySQL UP/DOWN/UP：PASS。使用随机命名的隔离 `gim_migration_test_*` 数据库，从空库 UP，ALL DOWN 确认业务表清空，再次 UP；测试结束 DROP，无测试库残留。
- Constraint / EXPLAIN：PASS。实际重复写入与 11 类查询计划均验证完成。
- User/Auth 回归：PASS。真实迁移库上的 GORM User Repository、User gRPC、Auth 注册/登录和 User lookup 全部成功。
- Test/Vet/Coverage/Secret Scan：PASS。
- Race：NOT RUN - environment limitation。本机 cgo 编译器报 `64-bit mode not compiled in`，未修改工具链。
- Implementation commit：`b38a893fd5ffe84c85dd0f6685e223f9cd18a28f`（短 hash `b38a893`）。
- Implementation push：SUCCESS — `origin/main`。
- Status/log commit：本操作记录与状态文件所在提交。
- Status/log push：SUCCESS — `origin/main`（提交后验证）。

## 正式表结构

- Identity：`users`、`user_confs`。
- Friend：`friends`、`friend_verifies`。
- Private Chat：`chat_messages`、`chat_sessions`、`chat_message_deletions`。
- Group：`groups`、`group_members`、`group_verifies`、`group_messages`、`group_sessions`、`group_message_deletions`。
- File：`file_objects`、`user_files`。
- Settings：`settings`。
- Migration metadata：`schema_migrations`。

## Constraints

- Identity：`account` 唯一且非空；可空 `open_id` 唯一；role/status 有默认值及枚举 CHECK；只存在强 Hash 字段，不存在明文凭据列。
- Friend：`(user_id, friend_id)` 唯一且两端不得相同；pending 申请使用规范化 pair 生成列唯一，阻止反向/并发重复。
- Chat：`(send_user_id, client_msg_id)` 幂等唯一；`(user_id, peer_user_id)` session 唯一；unread 使用 unsigned；置顶默认 false。
- Group：member `(group_id, user_id)`、message `(send_user_id, client_msg_id)`、session `(user_id, group_id)` 唯一；pending verify 使用生成列唯一；角色/状态/布尔字段有 CHECK。
- File：UUID 与 SHA-256 各自唯一，SHA-256 固定 64 字符；用户引用 `(user_id, file_object_id, original_name)` 唯一。
- Settings：key 唯一且非空，value 使用 MySQL JSON，公开标记默认 false。

## Indexes / EXPLAIN

真实 MySQL EXPLAIN 结论：

- 登录 account → `ux_users_account`。
- 收到的 pending 好友申请 → `ix_friend_verifies_receiver_status_created`。
- 好友列表 → `ux_friends_user_friend`。
- 私聊双方 + ID cursor → `ix_chat_messages_conversation_cursor` 可用；极小数据量下 optimizer 选择 PRIMARY 反向扫描，组合索引仍出现在 possible keys 且谓词顺序匹配。
- 私聊 session 排序 → `ix_chat_sessions_user_sort`。
- 群成员 / 我的群 → `ix_group_members_group_role_user` / `ix_group_members_user_group`。
- 群历史 + ID cursor → `ix_group_messages_group_cursor` 可用；极小数据量下 optimizer 选择 PRIMARY 反向扫描。
- 群 session 排序 → `ix_group_sessions_user_sort`。
- 用户文件 → `ix_user_files_user_created`。
- SHA-256 去重 → `ux_file_objects_sha256`。

未为每个字段机械建索引；索引围绕上述访问路径设计。

## Transactions / consistency

- V1 统一不使用数据库外键。跨逻辑服务关系由 Service 存在性检查、事务、唯一/非空/CHECK/索引维护，避免跨域删除耦合并保留拆库边界。
- 好友接受：锁定 pending request，双向 friend rows + request 状态同事务。
- 私聊发送：message + sender/receiver sessions 同事务；持久化成功后才可 ACK。
- 群创建：group + owner member 同事务。
- 群消息：message + 相关 group sessions 需一致性事务/批处理策略，持久化成功后才可 ACK。
- 文件：FileObject/UserFile 分层；数据库与物理存储跨资源失败由后续 File Service 做补偿。
- 不做模板式全表软删除；session `hidden_at` 和 message deletion 表均是当前用户维度，不影响其他用户/群成员。

## Constraint tests

实际 INSERT 均得到 MySQL duplicate-key 拒绝：重复 account、同向 friend relation、反向 pending friend request、私聊 clientMsgId、私聊 session、群成员、群消息 clientMsgId、群 session、FileObject SHA-256。另校验全部 16 张业务表 collation、关键列类型/长度/default/nullable/generated 属性、关键索引、无数据库 FK 与无明文凭据列。

## Migration / regression results

- EMPTY → UP ALL：PASS，6/6 clean applied。
- 再次 UP：PASS，不重复执行完成版本。
- ALL DOWN：PASS，逆序删除全部业务表，metadata 归零。
- RE-UP：PASS，6/6 clean applied，结构复核通过。
- User/Auth：真实 GORM Repository + gRPC 注册、登录、按 ID lookup PASS；Hash 与输入值不同，扩展字段/默认值正确。
- MySQL DDL 非事务性通过 dirty 状态处理：执行前标 dirty，失败停止且后续 UP 拒绝，须审阅 DOWN/备份后恢复。

## Tests

- `go test ./... -count=1`（带本地配置，MySQL/Redis/etcd/Gateway/Auth/User）：PASS。首次运行因 etcd 未启动失败；根因是本机进程缺失，按已有 localhost 开发配置恢复 etcd 后完整重跑通过，无代码规避；确认 `/gim/services/` 无测试 key 后停止本轮临时进程。
- 核心汇总 coverage（migration + MySQL + User + Auth，含真实集成）：75.7% statements。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- Secret Scan / repository safety / staged local-secret comparison：PASS；`.env.local`、`reference/`、私钥与本地敏感配置均未进入 Git。
- `go test -race ./...`：Race: NOT RUN - environment limitation；原因同既有 Windows cgo 64 位编译器限制。

## Problems and fixes

- 既有 User GORM model 尚未覆盖 `ip/addr/open_id/register_source` 且 role/status/时间类型未完全写明：统一 Domain、record tags、映射与 round-trip/隐私测试，并在真实迁移表回归。
- 多语句 Migration 需要原生 `database/sql`：在公共 MySQL client 增加不泄露 DSN 的 `OpenSQL`，复用连接池与 context health check。
- MySQL DDL 非事务性：增加 dirty migration 状态和 fail-fast 恢复规则；未引入重型迁移框架。
- 首次全量测试 etcd 拒绝连接：启动既有 localhost 单节点后重跑 PASS；未更改应用逻辑。

## Reference → GIM autonomous implementation → improvements

- Reference：只读审计 FIM User/Friend/UserConf、Chat message/top/delete、Group/member/message/verify/top/delete、File 与 Settings 的业务实体、查询语义和关系。
- GIM：依据自身文档从零编写 SQL、Runner、测试与 GORM 对齐，不复制原 SQL/Model/配置。
- Improvements：清晰 pending 状态与并发唯一约束；正式 clientMsgId 幂等；显式私聊/群聊 session；用户维度隐藏/删除；群临时禁言只用 Redis TTL；SHA-256 物理对象与用户引用分层；Settings 公开业务配置与部署 Secret 分离。

## 停止点 / 下一步

- Checkpoint 7 完成后停止；未实现 Friend/Chat/Group/File Service、WebSocket、ACK、上传、Web/Admin。
- 下一 Checkpoint：Checkpoint 8 — Web/Admin 基础工程与 build，仅在用户明确指令后开始。
