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

Day 1 builds these capabilities incrementally. Checkpoint 1 contains only the module boundary and compile-safe project layout; it intentionally contains no HTTP, RPC, WebSocket, migration, or business implementation.

## Layout

```text
cmd/          service entrypoints added by later checkpoints
internal/     GIM-owned shared and domain implementation
migrations/   versioned MySQL migrations
tests/        integration-test assets
```

Runtime secrets come from the ignored root `.env.local`. Committed configuration must contain placeholders only.
