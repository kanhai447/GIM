# Day 1 / Checkpoint 5 — Gateway + Gateway -> Auth 鉴权链路

- **日期:** 2026-10-08
- **Phase:** Phase 0.5
- **目标:** 自主实现 Gateway 的显式路由、etcd 服务发现、Auth authentication 调用、可信身份 Header 注入与 HTTP Reverse Proxy；不进入 Checkpoint 6 或 Day 2。

## 开始状态

- Branch：`main`。
- HEAD：`f017874cff6da1df8a8c506fe0721a2f1c16e04a`，与 `origin/main` 同步。
- Working tree：clean。
- Checkpoint 4：已完成并推送。

## etcd 恢复

- 复用本机现有 etcd 3.6.10 与忽略目录 `.local-data/etcd/`，未修改系统级配置。
- 以 `gim-dev` 单节点、localhost client/peer listener 和既有数据目录隐藏启动。
- `etcdctl --endpoints=http://127.0.0.1:2379 endpoint health` -> PASS。
- `etcdctl --endpoints=http://127.0.0.1:2379 endpoint status --write-out=table` -> PASS；节点为 leader，无 endpoint error。
- 未把业务服务地址改为硬编码，Checkpoint 5 可以继续。

## FIM Gateway 参考行为

- 参考 `/api/{service}/...` 服务名提取、etcd 中 `{service}_api` 地址查询、Gateway 调 Auth `/authentication`、注入 `User-ID` / `Role` 后反向代理的业务语义。
- 参考 HTTP ReverseProxy 后续兼容 Upgrade 的需求。
- 不复制 FIM 文件或实现，不复用其地址或配置值，`reference/` 不参与编译。

## GIM 自主实现与改进

- 使用显式路由表解析 `/api/{service}/...`，只接受 `auth/user/chat/group/file/settings/logs`，不使用正则、substring 或由外部输入拼接服务名。
- discovery 独立于 Gateway，统一使用 `/gim/services/{service}/{instanceID}`；Resolver 返回有序去重的多端点集合，业务 Handler 不绑定单实例存储结构。
- Registry 使用 etcd lease、keepalive 与显式 revoke/cleanup；注册地址来自 `INTERNAL_API_HOST`、服务端口和实例 ID 配置，不复用 reference 地址。
- Auth Client 独立封装 discovery、调用方 context、超时、有限响应解码、response body 关闭、多端点传输失败重试及安全错误映射；不解析或记录 JWT。
- Gateway 对每个合法请求调用 Auth，不维护第二套公开路径；只转发兼容的 `Token` / `ValidPath` 给 authentication。
- 在认证前删除外部 `User-ID`、`Role`、`ValidPath`；只有 Auth 返回已认证身份后才由 Gateway 写入可信 `User-ID` / `Role`。
- Reverse Proxy 保留 method/path/query/header/body/status/response header/response body，使用原请求 context；普通 HTTP 使用 timeout，Upgrade 请求不套用短请求超时，为后续 WebSocket 代理保留兼容性。
- 上游、Auth、discovery 的原始错误不进入客户端响应；请求取消和超时使用统一公开错误。
- 正式部署边界明确为仅 Gateway 对公网暴露，内部 API Header 只能信任 Gateway 注入值；本次未实现网络隔离本身。
- 相比 FIM，修正了模糊路由提取、每次查询临时创建 etcd client、无 lease 注册、外部身份 Header 可混入、后台 context 替代请求 context、上游错误直出的工程风险。

## 模块结构与修改文件

- `server/cmd/gateway/main.go`：Gateway 可执行入口、配置加载、etcd 复用、HTTP 生命周期与优雅退出。
- `server/internal/gateway/config/`：监听地址与 discovery/Auth/proxy timeout 配置。
- `server/internal/gateway/route/`：显式 API 服务映射与非法路径测试。
- `server/internal/gateway/authclient/`：Gateway -> Auth authentication 客户端及超时、关闭、安全错误测试。
- `server/internal/gateway/proxy/`：动态目标 Reverse Proxy、HTTP 语义与 Upgrade 识别测试。
- `server/internal/gateway/handler.go`、`handler_test.go`：鉴权编排、Header 防伪造、服务发现与统一错误。
- `server/internal/gateway/module.go`、`module_test.go`：Gateway 依赖组装。
- `server/internal/platform/discovery/`：etcd Resolver、lease Registry、配置与测试。
- `server/tests/gateway_auth_user_integration_test.go`：真实 etcd 下的 Gateway/Auth/User 全链路测试。
- `.env.example`、`server/README.md`、`docs/01_PROJECT_ARCHITECTURE.md`、`docs/05_HTTP_API.md`、`docs/08_REDIS_CONFIG.md`：配置、部署安全边界、路由、错误码和注册规则。
- `agent_logs/CURRENT_STATE.md`、`agent_logs/day-1.md`、`agent_logs/AGENT_WORKLOG.md`：Checkpoint 状态与测试事实。

## 协议与数据变化

- HTTP：新增 Gateway 统一入口能力；公开路径和受保护路径均先调用 Auth `/api/auth/authentication`。
- Header：Gateway 消费客户端 `Token`，向 Auth 发送 `Token` / `ValidPath`；转发业务请求前删除客户端身份 Header，再注入 Auth 返回的 `User-ID` / `Role`。
- Discovery：新增 `/gim/services/{service}/{instanceID}` key 契约，value 为配置生成的内部 HTTP endpoint，并附带 TTL lease。
- Error：新增 Gateway `1301` 至 `1306` 公开错误语义。
- DB/Redis：无 schema、migration 或 Redis key 变化。
- WebSocket：未实现业务；仅确保代理的 Upgrade 检测与 timeout 设计不阻断后续接入。

## 核心集成链路

- 使用真实本地 etcd 注册带 lease 的 `auth_api` 与 `user_api` 测试实例。
- Gateway -> Register -> Auth -> User RPC：PASS。
- Gateway -> Login -> Auth -> User RPC，签发认证材料：PASS；测试和日志不输出完整值。
- 认证材料 -> Gateway -> Auth authentication -> Gateway 注入身份 -> User API：PASS。
- 缺少认证材料、非法认证材料、Logout 后重复访问：均按 Auth 业务错误拒绝。
- 客户端伪造 `User-ID: 1`、`Role: 1`：Gateway 删除并覆盖为实际普通成员身份，未发生管理员越权。
- 未注册 `chat_api`：返回可控服务未注册错误；discovery timeout 单测返回可控超时错误。
- 测试结束后注册 key 已清理，`/gim/services/` 前缀无残留。

## 测试命令与结果

- `go test ./...` -> PASS。
- `go test -cover ./internal/gateway/... ./internal/platform/discovery` -> PASS。
- Gateway 相关汇总 coverage（含全链路集成测试）：**79.5% statements**。
- 关键包：Gateway 72.7%，Auth Client 83.1%，Gateway Config 73.9%，Proxy 89.3%，Route 100.0%；discovery 独立包 47.2%。
- `go vet ./...` -> PASS。
- 全仓 `gofmt -l` -> PASS（无输出）。
- `GIM_ENV_FILE=[REDACTED] go test ./tests -run 'Test(GatewayAuthUserChain|AuthRedisBlacklistTTL|LocalInfrastructure)' -count=1 -v` -> PASS；Gateway/Auth/User、Redis blacklist 与 MySQL/Redis/etcd 本地检查均通过。
- repository safety/Secret Scan -> PASS；`.env.local`、`reference/`、私钥、真实 Secret 和完整 JWT 未进入 Git。
- `git diff --check` -> PASS。
- `go test -race ./internal/gateway/... ./internal/platform/discovery` -> **NOT RUN - environment limitation**：本机 cgo C 编译器不支持 64 位模式。

## 问题、根因与处理

1. Checkpoint 开始时 etcd 未运行。
   - 处理：复用现有二进制和忽略的数据目录恢复单节点，health/status 通过后才进入实现。
2. Auth Client 不可用场景的首版测试错误地假设安全外层错误不能保留内部 cause。
   - 处理：保持客户端只能看到稳定公开文本、内部仍可 `Unwrap` 定位原因；修正断言后全量重跑通过。
3. 代码复核发现 Handler 对第三方 Resolver 返回空 slice 缺少防御。
   - 处理：空端点统一映射为“服务未注册”，新增回归用例，避免未来扩展时下标越界。
4. 首次最终汇总 coverage 命令的相对多包 `coverpkg` 模式被当前 Go 工具链识别为单一无匹配模式。
   - 处理：先用 `go list` 解析精确 import path，再以逗号连接重跑；集成测试通过并得到 79.5% 汇总覆盖率。
5. 收尾时首次使用了不存在的 `verify-repository-safety.ps1` 名称。
   - 处理：用 `rg --files scripts` 定位仓库实际的 `check-repository-safety.ps1`，按真实路径重跑并通过；未绕过安全检查。
6. race 仍被本机 64 位 cgo 工具链阻断。
   - 处理：按要求记录 NOT RUN，不在本 Checkpoint 更换工具链。

## etcd 最终状态

- endpoint health：PASS。
- endpoint status：PASS；etcd 3.6.10，单节点 leader，无 endpoint error。
- 集成注册使用 lease/keepalive，并在测试 cleanup 中 revoke；服务前缀无测试残留。

## Git

- **Commit hash:** `eb9eebc6bfe4ee41c388d6eb6297cbfca43316bf`。
- **Push:** SUCCESS — `origin/main`。

## 下一步

- Checkpoint 5 实现、验收、commit 与 push 已完成；在稳定节点立即停止。
- 下一 Checkpoint：Checkpoint 6 — Gateway -> Auth -> User 集成测试；本次不进入。
