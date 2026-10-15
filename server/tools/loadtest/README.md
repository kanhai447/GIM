# GIM load test harness

This is the single reproducible performance tool for Day 4 / Checkpoint 4. It
always uses the public Gateway URL. MySQL is used only to create the missing
friend fixture, verify persistence/session correctness, and clean prefixed test
data; it is never used as the measured request path.

Required runtime secrets:

```text
GIM_PERF_PASSWORD   password for disposable perf users
GIM_PERF_MYSQL_DSN  host MySQL DSN used by prepare/verify/cleanup
```

The tool never prints those values, JWTs, message content, or the DSN.

```powershell
go run ./tools/loadtest prepare -base-url http://127.0.0.1:8080 -users 40 -manifest ../.tmp/perf/manifest.json
go run ./tools/loadtest http -manifest ../.tmp/perf/manifest.json -workload profile -concurrency 10 -duration 15s
go run ./tools/loadtest private -manifest ../.tmp/perf/manifest.json -connections 50 -duration 30s -message-rate 20
go run ./tools/loadtest group -manifest ../.tmp/perf/manifest.json -group-size 5 -duration 30s -message-rate 10
go run ./tools/loadtest file -manifest ../.tmp/perf/manifest.json -size 1048576 -concurrency 5
go run ./tools/loadtest cleanup -manifest ../.tmp/perf/manifest.json
```

Every workload emits one JSON result containing attempts, successes, failures,
timeouts, protocol/business errors, throughput, p50/p95/p99, and workload-
specific correctness fields. `maximum tested` is intentionally a report label,
not a capacity claim.
