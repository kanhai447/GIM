# GIM Server

`server/` is the independent Go backend for GIM. It does not import or compile code from `reference/`.

Planned command boundaries:

- Gateway
- Auth API/RPC
- User API/RPC
- Chat API/RPC
- Group API/RPC
- File API/RPC
- Settings API/RPC
- Logs API and Kafka consumer

Day 1 builds these capabilities incrementally. Checkpoint 5 adds the authentication-aware HTTP Gateway, etcd discovery/lease registration primitives, Auth client, trusted identity Header injection, and a WebSocket-compatible reverse-proxy boundary. Chat/Group WebSocket business behavior is not part of this checkpoint.

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

With local etcd running and the ignored environment file configured, start the Gateway from `server/` with:

```powershell
$env:GIM_ENV_FILE = '..\.env.local'
go run ./cmd/gateway
```
