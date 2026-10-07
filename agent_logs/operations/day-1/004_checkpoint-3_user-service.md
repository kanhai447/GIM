# Day 1 / Checkpoint 3 — User API / RPC 基础能力

- **日期:** 2026-10-07
- **Phase:** Phase 0.5
- **目标:** 建立可供后续 Auth、Chat、Group、Gateway 和 Admin 复用的 User Domain、Repository、Service、HTTP 与 RPC 基础边界。

## FIM User 参考内容

- 参考 FIM `UserModel` 的昵称、头像、角色等基础字段语义，以及 User API 的 `/api/user/user_info` 路径。
- 参考 FIM User RPC 为 Auth/其他服务提供创建用户与基础资料查询的调用关系。
- 参考 FIM API/RPC 通过 ServiceContext 持有 DB/RPC 依赖的思路。
- 未复制 FIM 文件、目录或实现；未引用 `reference/` 参与编译。

## GIM 自主实现与工程化调整

- 采用 `HTTP/gRPC -> User Service -> Repository -> GORM/MySQL` 分层，HTTP 与 RPC 共享同一 Service，不重复业务逻辑。
- 新增独立 `account` 与 `status` 领域字段；`account` 的唯一约束由模型声明，正式 migration 留在 Checkpoint 7。
- `CreateUser` 接受 Auth 已生成的 `PasswordHash`；User 不接收明文密码、不重复 Hash，也不记录密码材料。
- `GetUserByID` 只返回公开 `UserInfo`；`GetUserByAccount` 的 `UserCredential` 仅供后续内部 Auth RPC 使用。
- 公开 DTO 与领域对象 JSON 均不会序列化 `PasswordHash`。
- Repository 使用 `WithContext(ctx)`；Service 保留 cancellation/deadline，并把底层错误转换为统一 GIM 业务错误。
- MySQL duplicate key 映射为 duplicate account；`gorm.ErrRecordNotFound` 映射为 user not found。
- gRPC 错误映射到标准 codes，未知数据库错误只返回安全的通用错误。
- 使用 go-zero REST Route；gRPC 注册函数可直接交给 go-zero zRPC Server，未引入全局可变单例。
- 相比 FIM，未在 User RPC 内 Hash 密码、未用 JSON bytes 承载完整用户模型、未把数据库原始错误直接返回外层。

## 架构分层与文件变化

- `server/api/user/v1/user.proto`：User RPC 契约。
- `server/api/user/v1/user.pb.go`、`user_grpc.pb.go`：由本地 protoc 插件生成。
- `server/internal/user/domain/user.go`：Role、Status、User、公开 UserInfo。
- `server/internal/user/repository/repository.go`：Repository 接口和稳定错误。
- `server/internal/user/repository/mysql/repository.go`：GORM/MySQL 实现与模型映射。
- `server/internal/user/service/service.go`：创建和按 ID/account 查询的共享业务逻辑。
- `server/internal/user/transport/http/profile.go`：`GET /api/user/user_info` Handler 与 go-zero Route。
- `server/internal/user/transport/grpc/server.go`：RPC Adapter、注册函数和错误转换。
- `server/internal/user/module.go`：显式依赖注入组装。
- 对应 `_test.go` 文件：领域、Repository 映射、Service、HTTP 与内存 gRPC 集成测试。
- `server/go.mod`、`server/go.sum`：固定 go-zero `v1.10.1`，加入 protobuf/gRPC 直接依赖；go-zero 的最小版本要求使 MySQL driver 和 go-redis 依赖升级。
- `docs/04_DATABASE_DESIGN.md`、`docs/05_HTTP_API.md`：同步 account/status、唯一约束、公开资料字段及 User 错误码。

## RPC 定义

- `CreateUser`：接收 account、nickname、password_hash、role、status、avatar，返回 user_id。
- `GetUserByID`：返回不含密码哈希的公开 UserInfo。
- `GetUserByAccount`：返回仅供内部 Auth 使用的 UserCredential。
- 所有 RPC 都透传调用方 `context.Context`，取消和超时分别转换为标准 gRPC 状态。

## 数据库变化

- 新增 GORM `users` 模型契约，`account` 声明唯一索引，`pwd_hash` 不允许公开序列化。
- **未执行 migration、未创建或修改任何本地业务表、未写入测试数据。**
- 正式 users migration 仍按计划在 Day 1 Checkpoint 7 集中实施；因此本 Checkpoint 未对真实 MySQL 表执行 Repository 集成测试。

## 测试命令与结果

- `go test ./...` -> PASS。
- `go test -cover ./internal/user/...` -> PASS。
  - module 100.0%；domain 33.3%；repository/mysql 30.4%；service 77.5%；transport/grpc 69.2%；transport/http 66.7%。
- `go vet ./...` -> PASS。
- 全仓 `gofmt -l` -> PASS（无输出）。
- repository safety script -> PASS；`.env.local`、`reference/`、密钥等仍被忽略。
- 内存 gRPC Server + 生成客户端调用 -> PASS。
- 覆盖场景：CreateUser、duplicate account、GetUserByID、not found、GetUserByAccount、公开 DTO 不含 hash、Repository 私有错误不泄露、Context 透传/取消。
- `go test -race ./internal/user/...` -> **NOT RUN - environment limitation**：`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。
- `git diff --check`、范围扫描、reference 依赖扫描 -> PASS。

## 问题、根因与修复

1. 首次 `gofmt -w api/user/v1/*.go` 在 PowerShell 下不会由外壳展开通配符。
   - 修复：使用 `rg --files ... -g '*.go'` 生成明确文件列表后重新执行 gofmt，并重跑全部测试。
2. 引入 go-zero `v1.10.1` 后，最小版本选择升级了 MySQL driver、go-redis、protobuf 与部分间接依赖。
   - 处理：保持 Go `1.25.0` 不变，执行 `go mod tidy -compat=1.25.0`，全量 test/vet 通过。
3. 本机 race 构建仍由缺少 64 位支持的 cgo 编译器阻断。
   - 处理：按要求记录 NOT RUN，不在本 Checkpoint 更换系统工具链。

## Git

- **Commit hash:** PENDING。
- **Push:** PENDING。

## 下一步

- 完成 staged diff、Secret Scan、commit 和 push 后停止。
- 下一 Checkpoint：Checkpoint 4 — Auth 注册、登录、JWT、Logout 基础链路。
