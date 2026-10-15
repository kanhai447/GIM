# 08 Redis、配置与运行约定

## 1. Redis Key

统一前缀 `gim:`，禁止各服务随意拼不一致 key。

建议：

```text
gim:auth:logout:{tokenHash}          -> 1, TTL=JWT remaining
gim:presence:user:{uid}              -> ZSET(instanceID -> expiresAtMillis), key TTL
gim:chat:delivery                    -> Pub/Sub channel, private message realtime fanout
gim:group:delivery                   -> Pub/Sub channel, group message realtime fanout
gim:group:prohibition:{memberId}     -> 1, TTL=mute duration
```

若缓存用户基础信息：

```text
gim:user:base:{uid}                  -> JSON, bounded TTL
```

Token 黑名单 key 不建议直接把完整 JWT 暴露在 Redis key/log；可以存 token hash/jti。

Group 临时禁言只使用 `rediskeys.GroupProhibition(memberId)` 生成上述 key。`memberId` 是 `(group_id,user_id)` 唯一 membership 的稳定主键，因此天然隔离不同群关系；设置、查询、解除均传播调用方 context 并使用有界操作超时。TTL 自然过期后 `IsMuted=false`。Group 服务不得访问或修改 `gim:presence:*`。

Group WebSocket connection foundation 不新增任何 Redis connection/presence key。connect、disconnect、heartbeat timeout、slow-client removal 与 shutdown 只改变 Group Hub 内存连接表；真实 Redis 负向测试以 Presence sentinel 和测试用户 key 验证前后未被创建、修改或删除。

GIM Auth V1 选择 SHA-256 token fingerprint 作为 `tokenHash`：Logout 写入 `gim:auth:logout:{tokenHash}`，值为 `1`，TTL 严格使用 JWT 的剩余有效时间；authentication 先完成验签/过期校验，再查询该 key。原始 JWT 不进入 Redis key、value 或日志。

## 2. Presence 规则

- Hub 只在本 Chat API 实例的用户连接 `0->1` 时提交 online transition，在 `1->0` 时提交 offline transition；`1->N` / `N->1` 不产生用户级 transition。
- Presence worker 将 instance ID 作为 ZSET member，score 为该 contribution 的过期毫秒时间；至少一个未过期 member 即全局在线。
- Presence worker 使用独立合理周期刷新活跃 contribution，不把每个 WebSocket Ping 直接变成 Redis 写入。
- 正常 shutdown 在 Hub 清空连接后删除本实例 contribution；进程 crash 或清理失败时由 member 过期时间和 key TTL 兜底。
- Redis 操作带 context timeout；故障时保留最终期望状态并重试，不在 Hub event loop 中执行无界 Redis I/O。
- Group WS 不修改 `gim:presence:*`；Chat WS 是全局 Presence 唯一权威来源。

多实例 Presence contribution 与私聊 realtime fanout 已分离：Presence 使用 ZSET；私聊持久化后统一发布 `gim:chat:delivery`，所有 Chat 实例订阅并只投递本实例 receiver Clients。同实例不再额外直投，避免重复。Pub/Sub 是 best-effort 通知而不是消息存储；publish 失败不撤销 MySQL 持久化或 Server ACK，history 才是恢复路径。

Checkpoint 3 的群消息同样使用独立 `gim:group:delivery`，事件只包含安全消息 envelope、messageId 与事务内 target user IDs。每个 Group 实例向本地目标用户全部 Clients 各投递一次；没有本地旁路，因此不会 local + Pub/Sub 双投。该 channel 不存储消息、不写 `gim:presence:*`，publish 失败由 history 恢复。

## 3. 配置

每个服务提供可提交的 example 配置，不提交密码/secret：

```text
MySQL DSN
Redis addr/password/db
etcd endpoints
JWT secret + expiry
Gateway route/service names
WS allowed origins
WS ping/pong durations
Chat text/payload limits, dependency/delivery timeout
upload root/max size/allowed mime
frontend public base url
Kafka audit brokers/topic/consumer group/bounded queue/timeout/retry
```

Gateway/服务发现还使用：`GATEWAY_DISCOVERY_TIMEOUT`、`GATEWAY_AUTH_TIMEOUT`、`GATEWAY_PROXY_TIMEOUT`、`INTERNAL_API_HOST`、各 API port/instance ID、`ETCD_SERVICE_TTL`。API 服务注册 key 为 `/gim/services/{service}/{instance}`，value 为配置生成的内部 HTTP endpoint；通过 lease keepalive 维持并在优雅退出时清理。正式部署不得把内部 API endpoint 暴露公网。

本地开发使用 `.env.example`/YAML example + docker-compose。任何 `192.168.x.x`、作者机器路径必须清理。

Kafka 部署配置使用 `KAFKA_BROKERS`、`KAFKA_AUDIT_TOPIC`（默认 `gim.audit.events`）、`KAFKA_AUDIT_CONSUMER_GROUP`、`KAFKA_AUDIT_QUEUE_CAPACITY`、`KAFKA_AUDIT_PUBLISH_TIMEOUT`、`KAFKA_AUDIT_RETRY_BACKOFF`、`KAFKA_AUDIT_MAX_ATTEMPTS`。broker 地址属于部署配置，认证凭据/Secret 不进入 Settings DB；未配置 broker 时 producer/consumer 均禁用，核心业务仍可启动。Kafka 只承载 audit event，不承载 Chat/Group realtime、Presence、Auth 状态或数据库事务真相。

## 4. Docker Compose

Day 4 完整 V1 的主 `compose.yaml` 一键启动 MySQL、Redis、单节点 etcd 与 Kafka 4.2.2 KRaft，以及全部应用 runtime；不引入 ZooKeeper。应用通过 Docker DNS service name 访问基础设施，API 仍经 etcd 注册/发现，不能把容器内部依赖配置为 `localhost`。

Compose 运行时直接读取环境变量；`.env.compose.example` 只包含 placeholder，实际 `.env.compose.local` 必须被 Git 忽略。`INTERNAL_BIND_HOST` 用于监听所有容器接口，`*_API_HOST`/`*_RPC_HOST` 用于可达的 Docker DNS endpoint，避免把 bind address 当成 advertised address。完整操作说明见 `docs/deployment/docker-compose.md`。
