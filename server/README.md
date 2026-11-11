# GIM Server

`server/` is the independent Go backend for GIM. It does not import or compile code from `reference/`.

## Command boundaries

- `cmd/gateway` — public HTTP/WebSocket entrypoint, Auth call, identity header injection.
- `cmd/auth-api` — register / login / logout / authentication.
- `cmd/user-api` + `cmd/user-rpc` — profile, friends, Admin user management, internal user RPC.
- `cmd/chat-api` — private chat HTTP + Chat WebSocket + Presence + Redis delivery.
- `cmd/group-api` — group HTTP, group messages/sessions and Group WebSocket (uses MySQL/Redis/User RPC directly).
- `cmd/file-api` — streaming upload, content-addressed storage, owned download/delete.
- `cmd/settings-api` — public settings, Admin settings and read-only Admin overview.
- `cmd/logs-api` — Admin-only structured audit query and Kafka consumer.
- `cmd/migrate`, `cmd/admin-bootstrap` — one-shot migration and first-admin tools.

V1 implements text private/group messaging, the friend loop, file ownership, Admin management and the bounded Kafka audit pipeline. Rich message types, friend online notifications, `user_confs` editing and delivery/read receipts are V2 and are not exposed.

## Layout

```text
cmd/          service entrypoints
internal/     GIM-owned shared and domain implementation
api/          generated gRPC contracts (user, group)
migrations/   versioned MySQL migrations (001-008)
tests/        gated integration tests (require GIM_ENV_FILE)
tools/        load harness
```

Runtime secrets come from the ignored root `.env.local`. Committed configuration must contain placeholders only.

## Network boundary

Production deployments expose the Gateway as the only public backend entrypoint. Auth, User, Chat, Group, File, Settings, and Logs API listeners are internal services registered below `/gim/services/{service}/{instance}` in etcd and must not be exposed directly to the Internet.

The Gateway removes any client-provided `User-ID`, `Role`, and `ValidPath` values, calls Auth `/api/auth/authentication`, and only then injects trusted `User-ID` / `Role` values. Target-service Admin middleware may trust these headers only when the internal API is reachable exclusively through the Gateway/private network.

Auth/User service processes build registration parameters from `INTERNAL_API_HOST`, their configured API port/instance ID, and `ETCD_SERVICE_TTL`, register with a lease, and close the registration during graceful shutdown. Business service addresses are never hardcoded in Gateway.

With MySQL, Redis, etcd and (optionally) Kafka running, migrations applied, and the ignored environment file configured, start the services from separate PowerShell terminals in this order:

```powershell
$env:GIM_ENV_FILE = '..\.env.local'
go run ./cmd/migrate -command up
go run ./cmd/user-rpc
go run ./cmd/user-api
go run ./cmd/auth-api
go run ./cmd/chat-api
go run ./cmd/group-api
go run ./cmd/file-api
go run ./cmd/settings-api
go run ./cmd/logs-api
go run ./cmd/gateway
```

`user-api`, `auth-api`, `chat-api`, `group-api`, `file-api`, `settings-api` and `logs-api` register leased endpoints in etcd and revoke them on graceful exit. Auth, Chat and Group connect to User RPC at `INTERNAL_API_HOST:USER_RPC_PORT`. RPC is an internal configured dependency rather than a Gateway-discovered HTTP service. Only Gateway should be exposed publicly.

## Friend API

`user-api` serves `/api/user/search`, `/api/user/friends` (GET/POST/PUT/DELETE), `/api/user/valid`, `/api/user/valid_status` and `/api/user/friend_info`. Acceptance writes both friendship rows and the request status in one transaction; `GET /api/user/friends` reads Redis Presence (`isOnline`) without modifying it. See `docs/05_HTTP_API.md` for payloads and error codes.

## WebSockets

Group WebSocket uses `/api/group/ws/chat?token=<JWT>` through Gateway/Auth; private chat uses `/api/chat/ws/chat?token=<JWT>`. Configure allowed origins, bounded queues and heartbeat values from `.env.example`. Both sockets are user-level multi-device sockets; only Chat WS contributes to global Presence.

`logs-api` serves the admin-only structured audit list/detail routes and consumes `gim.audit.events` into MySQL. All business services share the same bounded producer implementation; Kafka failure degrades audit visibility but never changes committed business success. Apply migration 007 before starting Logs and configure Kafka only through deployment environment values.
