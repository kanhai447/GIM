# Day 4 / Checkpoint 3 — Docker Compose Full Stack Deployment

Date: 2026-10-15

## Architecture

- A single `compose.yaml` runs the full V1 stack on one Docker bridge network named `gim`.
- Infrastructure: MySQL 8.4, Redis 7.4, etcd 3.5.17, and Kafka 4.2.2 in single-node KRaft mode.
- Application runtime: User RPC/API, Auth API, Chat API, Group RPC/API, File API, Settings API, Logs API/Consumer, Gateway, Web, and Admin.
- One-shot jobs: `migration` and `storage-init`; `admin-bootstrap` is an explicit `tools` profile and never runs automatically.
- Only Gateway, Web, Admin, and loopback infrastructure debug ports are published. Internal API/RPC services are not exposed to the host.

## Dockerfiles

- `docker/server.Dockerfile` uses a cached Go builder and a minimal Alpine runtime. All Go binaries share one image so Compose builds the backend once instead of downloading modules per service.
- `docker/frontend.Dockerfile` builds Web/Admin with pinned pnpm and copies only `dist` into unprivileged Nginx.
- Final images do not contain the Go toolchain, pnpm cache, source dependency trees, or development `node_modules`.
- Runtime users: backend `65532:65532`; Web/Admin Nginx user `101`.

## Config

- Runtime values come from ignored `.env.compose.local`; tracked `.env.compose.example` contains placeholders only.
- Container dependencies use Docker DNS names (`mysql`, `redis`, `etcd`, `kafka`), while service-to-service business routing remains etcd discovery.
- No real password, JWT, token, hash, or credential is recorded here. The actual local JWT secret was generated outside tracked files.
- Web/Admin use same-origin `/api`; no internal service URL or `host.docker.internal` is embedded in the production bundles.

## Startup order

- MySQL, Redis, etcd, and Kafka must be healthy before dependent application services start.
- `migration` waits for MySQL and exits successfully before services that use the schema start.
- User RPC becomes healthy before Auth/Chat/Group consumers of that RPC; Gateway starts only after all routed APIs are healthy.
- Long-running services use `restart: unless-stopped`; one-shot jobs use `restart: no`.

## Health checks

- Infrastructure has native readiness checks.
- Go API/RPC/Gateway processes expose or serve minimal readiness checks without configuration or secret details.
- Nginx frontends are checked over HTTP.
- Final public checks: Gateway `200`, Web `200`, Admin `200`.

## Migration

- Clean start applied migrations 001–008 in order.
- A persistent restart reran the job idempotently; final status reports 001–008 `applied`.
- No existing migration was modified.

## Kafka

- Kafka is one 4.2.2 KRaft broker with separate `INTERNAL://kafka:29092` and loopback host debugging listener `127.0.0.1:19092`.
- Application containers use only the internal advertised listener.
- Real login, private message, group message, and file upload events traversed Kafka -> Logs Consumer -> MySQL -> Admin Logs API.
- With Kafka stopped, Private Chat, Group Chat, and File upload remained successful; audit degraded as designed. After restart, newly produced audit events were consumed. No claim is made that events lost during the outage were backfilled.

## Discovery

- etcd contained live registrations for `auth_api`, `user_api`, `chat_api`, `group_api`, `file_api`, `settings_api`, and `logs_api`.
- Gateway continued to discover all routed services through etcd; Docker deployment did not replace discovery with fixed business URLs.
- Restarting Chat, Group, File, and Logs produced new leases. A graceful Logs stop removed its old registration before a healthy restart.

## File persistence

- `file_storage` mounts the configured File root. `temp` and `objects` are directories under the same mounted filesystem, preserving atomic rename semantics.
- An uploaded object retained the same SHA-256 after File API restart and after `docker compose down` followed by `up -d` without `-v`.
- The real browser file smoke covered upload/list/refresh/progress, 12 MiB content integrity, size preflight, authenticated download, ownership, shared-object delete safety, cursor pagination, safe filename rendering, and reference delete.
- Nginx `client_max_body_size` is 51 MiB so the existing 50 MiB application limit remains authoritative.

## Browser/API smoke

- API: formal Register/Login/Profile/Logout passed; the revoked token was rejected.
- Private Chat real headless-browser smoke: two users, WS connect, pending -> sent ACK, bidirectional realtime/order, unread/read, history/cursor, multi-device reconciliation, safe text rendering, reconnect, logout, and revoked-token terminal behavior passed.
- Group real headless-browser smoke: three users, create/invite, Group WS, ACK/realtime/unread/history, mute/unmute, multi-device, safe text, reconnect, and Chat socket independence passed.
- File real headless-browser smoke passed as described above.
- Admin real headless-browser focused smoke passed Login, Dashboard, Users, and Logs with no API failures.
- The current V1 has no friend-management HTTP flow. For the isolated Private Chat smoke only, two symmetric friend fixture rows were inserted into the Compose test database after both users were created through formal Register. No production configuration or source was changed.
- A Chinese filename uploaded through the formal File API survived download and both restart paths, confirming the Compose MySQL/filesystem UTF-8 path in addition to the existing browser UI's Chinese content.

## Restart/recovery

- Chat, Group, File, and Logs restarts returned to healthy; WS clients reconnected and business operations resumed.
- Logs graceful stop completed with exit code 0 and its etcd lease disappeared.
- A controlled MySQL restart caused one permitted transient dependency error; without restarting application containers, Register/Login/Profile subsequently recovered. No service remained permanently failed.
- Redis-backed logout, presence/delivery, Group mute, and delivery were exercised under normal operation. Redis chaos was intentionally out of scope.

## Clean start

- Ran project-scoped `docker compose down -v --remove-orphans`, then `docker compose up -d`.
- Only GIM Compose volumes were deleted. The empty environment reached healthy state, migration exited 0, and API/browser smokes passed.

## Persistent start

- Ran `docker compose down`, then `docker compose up -d` without `-v`.
- MySQL records, Kafka state, and File content persisted; migration remained idempotent.

## Security

- Actual secrets are runtime env only and are not build args, image layers, tracked docs, or source.
- Admin bootstrap accepts credentials only from ephemeral environment variables, reuses formal bcrypt/User Service/Domain/Repository logic, creates an active admin once, and refuses to reset an existing account.
- `.dockerignore` excludes Git, `reference/`, agent logs, local env files, keys, build output, dependencies, and runtime storage.
- Internal dependency config contains no host `localhost`; host loopback is used only for intended public/debug bindings.
- Application logs use stdout/stderr; audit records remain separately structured and privacy-filtered.

## Secret scan

- Repository safety script: PASS.
- `.env.compose.local`: ignored and untracked.
- Dockerfile/history review found no secret build argument or copied environment file.
- No local E2E password, JWT, database/Redis password, token, private key, or password hash was written to tracked files or this log.

## Image/build

- `docker compose config --quiet`: PASS.
- `docker compose build`: PASS for shared server, Web, and Admin images.
- Core backend image no-cache build: PASS; dependency/build cache mounts were intentionally reused.
- Sizes: backend 280,751,293 bytes; Web 54,862,737 bytes; Admin 55,528,198 bytes.
- Windows Docker Desktop/Linux-container paths, named volumes, permissions, line endings, production static serving, and executable binaries were exercised successfully.

## Regression

- Backend: `go test ./... -count=1` PASS; `go vet ./...` PASS; repository Go format check PASS.
- Web: type-check PASS; 16 test files / 90 tests PASS; production build PASS.
- Admin: type-check PASS; 11 test files / 25 tests PASS; production build PASS.
- `git diff --check`: PASS.

## Reference implementation use

- Reference behavior considered: the documented service boundaries, Gateway/Auth/business-service chain, separate Chat/Group WebSockets, and Admin audit workflow.
- GIM implementation is autonomous: Compose, Dockerfiles, readiness/config adapters, bootstrap command, reverse proxy, and validation were written for the current GIM code. There is no import, runtime read, symlink, or build dependency on `reference/`.

## Known limitations

- Development/single-host only; no HA, Kubernetes, or production cloud deployment.
- MySQL, Redis, etcd, and Kafka are single-node; internal traffic has no TLS and there is no production secret manager.
- No Compose CPU/memory hard limits or read-only root filesystems are enforced in this checkpoint.
- Audit remains best-effort without transactional outbox, DLQ, or exactly-once guarantees.
- Private Chat smoke requires a database friend fixture until a formal friend-management HTTP workflow exists.
- Vite retains the previously known non-blocking Admin chunk-size warning.

## Race

`Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`

`Race Verification Pending`

The known local 32-bit MinGW limitation was not retried. A compatible 64-bit CGO environment/CI remains required.

## Final runtime state

- All long-running Compose services are healthy; migration and storage-init exited 0.
- Compose services were intentionally left running after acceptance for immediate local use.

## Git

- Implementation commit: `36e694e chore(deploy): add full stack docker compose`.
- Documentation/status commit: this record's commit.
- Target: push both commits to `origin/main`, then verify `HEAD == origin/main` and a clean working tree.
