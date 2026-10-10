# 08 Redis、配置与运行约定

## 1. Redis Key

统一前缀 `gim:`，禁止各服务随意拼不一致 key。

建议：

```text
gim:auth:logout:{tokenHash}          -> 1, TTL=JWT remaining
gim:presence:user:{uid}              -> ZSET(instanceID -> expiresAtMillis), key TTL
gim:chat:delivery                    -> Pub/Sub channel, private message realtime fanout
gim:group:prohibition:{memberId}     -> 1, TTL=mute duration
```

若缓存用户基础信息：

```text
gim:user:base:{uid}                  -> JSON, bounded TTL
```

Token 黑名单 key 不建议直接把完整 JWT 暴露在 Redis key/log；可以存 token hash/jti。

GIM Auth V1 选择 SHA-256 token fingerprint 作为 `tokenHash`：Logout 写入 `gim:auth:logout:{tokenHash}`，值为 `1`，TTL 严格使用 JWT 的剩余有效时间；authentication 先完成验签/过期校验，再查询该 key。原始 JWT 不进入 Redis key、value 或日志。

## 2. Presence 规则

- Hub 只在本 Chat API 实例的用户连接 `0->1` 时提交 online transition，在 `1->0` 时提交 offline transition；`1->N` / `N->1` 不产生用户级 transition。
- Presence worker 将 instance ID 作为 ZSET member，score 为该 contribution 的过期毫秒时间；至少一个未过期 member 即全局在线。
- Presence worker 使用独立合理周期刷新活跃 contribution，不把每个 WebSocket Ping 直接变成 Redis 写入。
- 正常 shutdown 在 Hub 清空连接后删除本实例 contribution；进程 crash 或清理失败时由 member 过期时间和 key TTL 兜底。
- Redis 操作带 context timeout；故障时保留最终期望状态并重试，不在 Hub event loop 中执行无界 Redis I/O。
- Group WS 不修改 `gim:presence:*`；Chat WS 是全局 Presence 唯一权威来源。

多实例 Presence contribution 与私聊 realtime fanout 已分离：Presence 使用 ZSET；私聊持久化后统一发布 `gim:chat:delivery`，所有 Chat 实例订阅并只投递本实例 receiver Clients。同实例不再额外直投，避免重复。Pub/Sub 是 best-effort 通知而不是消息存储；publish 失败不撤销 MySQL 持久化或 Server ACK，history 才是恢复路径。Group/更通用的跨实例 WebSocket routing 仍留待对应 Checkpoint/V2。

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
```

Gateway/服务发现还使用：`GATEWAY_DISCOVERY_TIMEOUT`、`GATEWAY_AUTH_TIMEOUT`、`GATEWAY_PROXY_TIMEOUT`、`INTERNAL_API_HOST`、各 API port/instance ID、`ETCD_SERVICE_TTL`。API 服务注册 key 为 `/gim/services/{service}/{instance}`，value 为配置生成的内部 HTTP endpoint；通过 lease keepalive 维持并在优雅退出时清理。正式部署不得把内部 API endpoint 暴露公网。

本地开发使用 `.env.example`/YAML example + docker-compose。任何 `192.168.x.x`、作者机器路径必须清理。

## 4. Docker Compose

V1 至少一键启动 MySQL、Redis、etcd。Kafka/ZooKeeper 不应成为 `docker compose up` 的必选依赖。
