# Day 1

## 今日目标

- 仅执行 Phase 0、0.5、1，并按稳定 Checkpoint 完成测试、日志、commit、push 闭环。

## 完成项

- [x] Checkpoint 1：安全配置检查与 GIM 独立工程骨架。
- [x] Checkpoint 2A：配置、统一错误与 HTTP 响应。
- [x] Checkpoint 2B：MySQL、Redis、etcd 客户端。
- [x] Checkpoint 3：User API / RPC 基础能力。
- [x] Checkpoint 4：Auth 注册、登录、JWT、Logout。
- [ ] Checkpoint 5：Gateway 与 Auth 鉴权链路。
- [ ] Checkpoint 6：Gateway -> Auth -> User 集成测试。
- [ ] Checkpoint 7：Day 1 数据库底座与 migration。
- [ ] Checkpoint 8：Web/Admin 基础工程与 build。
- [ ] Checkpoint 9：Day 1 全量验收。

## Checkpoint 1

- 日期：2026-10-07
- Phase：0
- 主要改动：独立 Go module、三工程空骨架、仓库安全检查脚本、可恢复状态文件。
- DB/API/WS 变化：无。
- 测试：安全脚本、gofmt、Go test/vet、reference 零依赖、工程边界均 PASS。
- 问题：首次测试脚本相对路径错误；修正调用路径后完整重跑通过。
- Commit：`81340911fe58df2ccaee3e3fc540b6c6c7cbf662`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- 从 Checkpoint 2B MySQL、Redis、etcd 客户端恢复。

## Checkpoint 2A

- 日期：2026-10-07
- Phase：0.5
- 主要改动：安全配置解析、统一应用错误、V1 HTTP 响应与单元测试。
- DB/API/WS 变化：没有业务路由或字段变化；公共响应保持 `{code,msg,data}`。
- 测试：普通单测、覆盖率、vet、gofmt、安全脚本 PASS。
- Race：本机 cgo 编译器不支持 64 位模式，真实状态为 NOT AVAILABLE。
- Commit：`e6811c9df4248c3af9e84b905da6bdb498958e2a`。
- Push：SUCCESS — `origin/main`。

## Checkpoint 2B

- 日期：2026-10-07
- Phase：0.5
- 主要改动：MySQL/GORM、Redis、etcd 可注入客户端，context 健康检查与显式 Close，真实本地集成测试。
- DB/API/WS 变化：无 migration、无业务表、无业务 Redis key、无 API/WS 变化。
- 测试：全量单测、覆盖率、vet、gofmt、安全脚本、MySQL/Redis/etcd 集成测试 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN。
- Commit：`b255dd3221a7fc83f15aeb3a15cfaf70c7257b93`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 2B 已完成并推送；下一次从 Checkpoint 3 User API / RPC 基础能力恢复。

## Checkpoint 3

- 日期：2026-10-07
- Phase：0.5
- 主要改动：User Domain/Repository/Service、公开资料 HTTP Handler、Create/GetByID/GetByAccount gRPC 契约与生成客户端集成测试。
- DB/API/RPC 变化：建立 users GORM 模型契约但未 migration；实现 `GET /api/user/user_info`；新增三个内部 User RPC。
- 安全：Auth 负责 Hash，User 只接收 `password_hash`；公开 DTO 不返回密码哈希，底层数据库错误不泄露。
- 测试：全量单测、User 覆盖率、vet、gofmt、安全脚本、内存 gRPC 调用 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- Commit：`3aa0150ffbeecadbcc872344c70b67cf24a0ce0a`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 3 已完成并推送；下一次从 Checkpoint 4 Auth 注册、登录、JWT、Logout 基础链路恢复。

## Checkpoint 4

- 日期：2026-10-08
- Phase：0.5
- 主要改动：Auth 注册/登录、bcrypt、JWT Claims 与验证、精确公开路径、authentication、Logout Redis blacklist、User RPC 适配与 HTTP Handler。
- DB/API/Redis 变化：无 migration；实现四个 Auth HTTP 路由；新增统一 `gim:auth:logout:{tokenHash}`，TTL 为 JWT 剩余时间。
- 安全：签名材料仅从安全配置读取；不记录口令/完整认证材料；Redis key 使用 SHA-256 fingerprint；登录失败统一外部提示。
- 测试：全量单测、72.8% Auth 汇总 coverage、vet、gofmt、安全脚本、真实 Redis TTL 集成测试 PASS。
- Race：本机 cgo C 编译器不支持 64 位模式，NOT RUN - environment limitation。
- Commit：`519282dbdfdbead9efeb556e6013dd14104d1c08`。
- Push：SUCCESS — `origin/main`。

## 次步入口

- Checkpoint 4 已完成并推送；下一次仅在用户明确指令下进入 Checkpoint 5 Gateway。
