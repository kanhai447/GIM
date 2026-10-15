# File and Audit Design

## File upload

File API 使用 `MaxBytesReader` 限制 multipart 总体积，随后以流式方式完成：

1. 在 storage root 的 `temp` 目录创建临时文件。
2. 边读边计数并计算 SHA-256，不使用 `io.ReadAll`。
3. 从内容前缀探测 MIME，不信任客户端声明。
4. 将对象提交到 `objects/<hash-prefix>/<sha256>`；temp 与 objects 在同一 volume，支持原子 rename。
5. 事务内 upsert `file_objects` 并创建当前用户的 `user_files` 引用。
6. 数据库失败时执行补偿；并发相同内容依赖 SHA-256 唯一键收敛。

下载和删除以 `user_files` ownership 为边界。响应只返回逻辑 ID、UID、hash、MIME 和公开下载 URL，不返回物理 storage path。路径在 Local Storage 层清理并限制在根目录内。

删除的是用户引用，不同步删除内容对象。最后一个引用消失后物理对象可安全复用，但 V1 没有孤儿 GC；浏览器下载也会将响应物化成 Blob。

## Structured audit

业务服务在核心操作成功后创建新的不可变事件值，不共享可变 Pusher。事件只包含：

- `eventId`、schema version、event type、service/action；
- actor user/role；
- resource type/id、request id、status、occurred time；
- 白名单、低敏 metadata。

明确禁止密码、密码 hash、JWT、Authorization、Cookie、第三方 Secret、DSN、消息正文、文件内容和物理存储路径。

Producer 使用固定容量队列、短 publish timeout、有限次数重试和 drop counter。队列满或 Kafka 不可用时，审计可以丢弃但核心请求不会被拖死。Logs consumer 写入 MySQL，`event_id` 唯一键使重复消费幂等；Admin Logs 只通过 Gateway/Auth 和 Logs 服务端角色校验访问。

## Failure semantics

Audit 是 post-commit best-effort：

- Kafka 短故障：业务继续，恢复后的新事件继续消费。
- 长故障/队列满：事件可能丢失，可由 queue/drop metrics 观察。
- 无 transactional outbox、DLQ、磁盘 spool、跨 Kafka/MySQL exactly-once 或保留策略。

这套取舍适合 V1 展示“审计不拖垮核心 IM”，但不等价于合规审计系统。生产升级应优先评估 transactional outbox、DLQ、重放/保留、告警和数据分级。

## Verification

- 1 MiB/10 MiB 文件与并发 dedup、ownership、MIME、限长、分页、下载和删除均通过真实 Gateway 测试。
- Kafka 正常消费、不可用时业务有界返回、恢复后新事件继续、重复事件幂等和 consumer restart 均已验证。
- Admin UI 不使用 `v-html`；Logs list/detail 不暴露敏感 payload。
