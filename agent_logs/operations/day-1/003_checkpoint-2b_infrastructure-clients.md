# Day 1 / Checkpoint 2B — MySQL、Redis、etcd 基础设施客户端

- **日期:** 2026-10-07
- **Phase:** Phase 0.5
- **目标:** 为后续 ServiceContext 提供可注入、支持 context、健康检查和显式关闭的 MySQL/GORM、Redis、etcd 客户端。

## 参考范围

- FIM `core/mysql.go`、`core/redis.go`、`core/etcd.go` 的连接需求。
- FIM Auth/User ServiceContext 持有 DB、Redis、RPC 客户端的依赖关系。
- FIM 通过 etcd 读写服务地址的行为边界；本 Checkpoint 不实现注册发现业务。

## GIM 自主实现与改进

- 分为 `database/mysql`、`redis`、`etcd` 三个独立包，未复制 FIM 文件。
- 统一采用 `Config -> Open(ctx) -> Health/Status(ctx) -> Close()` 生命周期，无业务包裸全局变量。
- MySQL 使用 GORM/MySQL driver，支持最大打开/空闲连接数和连接最大生命周期。
- Redis 使用 go-redis v9、RESP2 兼容模式、可配置 DB 与连接池，只提供基础 Ping。
- etcd 使用 v3 client，可检查全部 endpoint status，明确 client Close。
- 初始化/健康错误保留私有 cause，但公开 `Error()` 不含 DSN、密码或 endpoint 内容。
- FIM 的 panic 初始化、Redis 未使用 timeout context、etcd 反复创建且不关闭等行为未沿用。

## 文件变化

- `.env.example`
- `scripts/check-repository-safety.ps1`
- `server/go.mod`
- `server/go.sum`
- `server/internal/platform/database/mysql/client.go`
- `server/internal/platform/database/mysql/client_test.go`
- `server/internal/platform/redis/client.go`
- `server/internal/platform/redis/client_test.go`
- `server/internal/platform/etcd/client.go`
- `server/internal/platform/etcd/client_test.go`
- `server/tests/infrastructure_test.go`
- `agent_logs/CURRENT_STATE.md`
- `agent_logs/day-1.md`
- `agent_logs/AGENT_WORKLOG.md`
- 本操作日志。

## 测试命令与结果

- `go test ./...` -> PASS。
- `GIM_ENV_FILE=[REDACTED] go test -v ./tests -run TestLocalInfrastructure -count=1` -> PASS。
  - MySQL connect/Ping/Close -> PASS。
  - MySQL 错误凭据拒绝且错误文本不含测试凭据 -> PASS。
  - Redis connect/Ping/Close -> PASS。
  - etcd connect/Status/Close -> PASS。
- `go test -cover ./internal/platform/...` -> PASS。
  - apperror 68.8%，config 83.8%，mysql 5.9%，etcd 7.1%，httpresponse 100.0%，redis 7.1%。
- `go vet ./...` -> PASS。
- `gofmt -l .` -> PASS。
- repository safety script -> PASS。
- `git diff --check` -> PASS。
- 禁止范围扫描（migration/业务 Redis key/WS/Kafka）-> PASS，0 命中。
- `go test -race ./internal/platform/...` -> NOT RUN：本机 cgo C 编译器不支持 64 位模式。

## 问题、根因与修复

1. go-redis latest 要求 Go 1.26，最初使 `go.mod` 自动升级。
   - 修复：固定 go-redis `v9.17.2`、etcd client `v3.6.10`、MySQL driver `v1.8.1`，清理 latest 间接依赖并用本机 Go 1.25 tidy 验证。
2. MySQL driver `FormatDSN` 为指针接收器，首次编译不能在复合字面量直接调用。
   - 修复：先构造局部 driver config 再调用；全量测试通过。
3. etcd 进程已停止，真实集成测试连接被拒绝。
   - 修复：按本地单节点配置从已忽略 `.local-data/etcd` 启动，Status 测试通过。
4. 本机 Redis 未设置服务端密码，但 `.env.local` 原为非空值；旧 redis-cli 未应用认证环境变量造成早期误判。
   - 修复：通过脱敏分类确认 `SERVER_HAS_NO_PASSWORD`，将被忽略的本地 `REDIS_PASSWORD` 设为空；安全脚本改为允许 Redis 空密码但要求键存在。
5. Race 仍报 `cc1.exe: 64-bit mode not compiled in`。
   - 处理：如实记录 NOT RUN，不在本 Checkpoint 安装或替换系统编译器。

## Git

- **Commit hash:** PENDING。
- **Push:** PENDING。

## 下一步

- 完成 staged diff、Secret Scan、commit 和 push 后停止。
- 下一 Checkpoint：Checkpoint 3 — User API / RPC 基础能力。
