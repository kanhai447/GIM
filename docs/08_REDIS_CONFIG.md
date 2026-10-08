# 08 Redis、配置与运行约定

## 1. Redis Key

统一前缀 `gim:`，禁止各服务随意拼不一致 key。

建议：

```text
gim:auth:logout:{tokenHash}          -> 1, TTL=JWT remaining
gim:presence:user:{uid}              -> metadata/count, TTL refreshed by Chat WS
gim:group:prohibition:{memberId}     -> 1, TTL=mute duration
```

若缓存用户基础信息：

```text
gim:user:base:{uid}                  -> JSON, bounded TTL
```

Token 黑名单 key 不建议直接把完整 JWT 暴露在 Redis key/log；可以存 token hash/jti。

GIM Auth V1 选择 SHA-256 token fingerprint 作为 `tokenHash`：Logout 写入 `gim:auth:logout:{tokenHash}`，值为 `1`，TTL 严格使用 JWT 的剩余有效时间；authentication 先完成验签/过期校验，再查询该 key。原始 JWT 不进入 Redis key、value 或日志。

## 2. Presence 规则

- Chat WS register 首连接写/刷新；
- Chat WS 心跳刷新 TTL；
- 最后连接 unregister 删除，或异常情况下依赖 TTL 兜底；
- Group WS 不修改 `gim:presence:*`。

V1 单 Chat WS 实例。未来多实例时 Presence 和消息路由需要 Redis Pub/Sub/Stream 或独立 gateway，但不在 V1。

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
upload root/max size/allowed mime
frontend public base url
```

Gateway/服务发现还使用：`GATEWAY_DISCOVERY_TIMEOUT`、`GATEWAY_AUTH_TIMEOUT`、`GATEWAY_PROXY_TIMEOUT`、`INTERNAL_API_HOST`、各 API port/instance ID、`ETCD_SERVICE_TTL`。API 服务注册 key 为 `/gim/services/{service}/{instance}`，value 为配置生成的内部 HTTP endpoint；通过 lease keepalive 维持并在优雅退出时清理。正式部署不得把内部 API endpoint 暴露公网。

本地开发使用 `.env.example`/YAML example + docker-compose。任何 `192.168.x.x`、作者机器路径必须清理。

## 4. Docker Compose

V1 至少一键启动 MySQL、Redis、etcd。Kafka/ZooKeeper 不应成为 `docker compose up` 的必选依赖。
