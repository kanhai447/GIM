# GIM Server

`server/` is the independent Go backend for GIM. It does not import or compile code from `reference/`.

Command boundaries:

- Gateway
- Auth API/RPC
- User API/RPC
- Chat API/RPC
- Group API/RPC
- File API/RPC
- Settings API/RPC
- Logs API and Kafka consumer

Day 1 provides runnable Gateway, Auth API, User API and User RPC commands. Day 2 adds Chat API. Day 3 Checkpoint 1 adds Group API/RPC domain and membership commands; Checkpoint 2 adds the independent Group WebSocket connection runtime. Group message persistence, ACK and session/unread remain later work.

## Layout

```text
cmd/          service entrypoints added by later checkpoints
internal/     GIM-owned shared and domain implementation
migrations/   versioned MySQL migrations
tests/        integration-test assets
```

Runtime secrets come from the ignored root `.env.local`. Committed configuration must contain placeholders only.

## Network boundary

Production deployments expose the Gateway as the only public backend entrypoint. Auth, User, Chat, Group, File, Settings, and Logs API listeners are internal services registered below `/gim/services/{service}/{instance}` in etcd and must not be exposed directly to the Internet.

The Gateway removes any client-provided `User-ID`, `Role`, and `ValidPath` values, calls Auth `/api/auth/authentication`, and only then injects trusted `User-ID` / `Role` values. Target-service Admin middleware may trust these headers only when the internal API is reachable exclusively through the Gateway/private network.

Auth/User service processes should build registration parameters from `INTERNAL_API_HOST`, their configured API port/instance ID, and `ETCD_SERVICE_TTL`, register with a lease, and close the registration during graceful shutdown. Business service addresses are never hardcoded in Gateway.

With MySQL, Redis and etcd running, migrations applied, and the ignored environment file configured, start the Day 1 services from separate PowerShell terminals in this order:

```powershell
$env:GIM_ENV_FILE = '..\.env.local'
go run ./cmd/user-rpc
go run ./cmd/user-api
go run ./cmd/auth-api
go run ./cmd/chat-api
go run ./cmd/group-rpc
go run ./cmd/group-api
go run ./cmd/gateway
```

`user-api`, `auth-api`, `chat-api`, and `group-api` register leased endpoints in etcd and revoke them on graceful exit. Auth, Chat, and Group connect to User RPC at `INTERNAL_API_HOST:USER_RPC_PORT`; Group also exposes the minimal internal membership RPC at `GROUP_RPC_PORT`. RPC is an internal configured dependency rather than a Gateway-discovered HTTP service. Only Gateway should be exposed publicly.

Group WebSocket uses `/api/group/ws/chat?token=<JWT>` through Gateway/Auth. Configure `GROUP_WS_ALLOWED_ORIGINS`, bounded queue and heartbeat values from `.env.example`. It is a user-level multi-device socket, does not bind to one group, and never contributes to Chat-owned global Presence.
