# GIM MySQL migrations

`*.up.sql` / `*.down.sql` are the only executable schema source of truth. Versions must be contiguous, names must pair exactly, and released migration files must not be edited in place.

Run from `server/` with an ignored local environment file:

```text
go run ./cmd/migrate -env ../.env.local -command status
go run ./cmd/migrate -env ../.env.local -command up
go run ./cmd/migrate -env ../.env.local -command down -steps 1
```

MySQL DDL is not transactionally atomic. The runner records a dirty version before applying its SQL and refuses further UP operations if that version fails. Review the matching DOWN and the database backup before rolling back. `-steps 0` removes every applied version and is intended for disposable databases, not routine production use.
