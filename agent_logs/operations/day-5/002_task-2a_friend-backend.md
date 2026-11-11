# 002 / Task 2A — 好友闭环后端 API

- **日期**：2026-11-11（Asia/Shanghai）
- **授权范围**：docs/05 好友相关 8 条路由 + 单元/集成测试；不改动 Hub/Client、Presence 写入、幂等与 session 写路径。
- **开始状态**：HEAD `729b8aa`，working tree clean。
- **结束状态**：HEAD `a97bee6`（本日志所在提交另计）。

## 目标

实现好友搜索 / 申请 / 验证处理 / 好友列表（含 isOnline）/ 备注 / 删除 / 好友详情，并保证同意申请的并发安全。

## 修改文件

| 文件 | 变更 |
|---|---|
| `server/internal/user/friend/friend.go` | 新增：领域类型、错误、8 个服务方法、Presence 降级、错误码映射 |
| `server/internal/user/friend/repository/mysql/repository.go` | 新增：搜索、好友列表/详情、申请创建/列表、事务内同意/拒绝、备注、双向删除 |
| `server/internal/user/friend/transport/http/handler.go` | 新增：8 条 `/api/user/*` 路由、可信 `User-ID`、64 KiB JSON 限制、统一 envelope |
| `server/internal/user/friend/service_test.go` | 新增 9 组服务测试（正常 + 错误码 + Presence 降级） |
| `server/internal/user/friend/transport/http/handler_test.go` | 新增 8 组 HTTP 测试（未认证、参数、状态映射、错误 envelope） |
| `server/tests/friend_integration_test.go` | 新增 gated MySQL 集成测试：全生命周期 + 双并发同意 |
| `server/cmd/user-api/main.go` | 装配 friend service/repository/handler，复用 Presence Redis store |
| `docs/05_HTTP_API.md` | User 章节改为“已实现 / V2 未实现”，补齐 8 条好友路由、字段、错误码、Presence 语义 |
| `docs/04_DATABASE_DESIGN.md` | 7.4 补充备注方向性、申请附言上限、唯一写入路径、Presence 只读 |

## 数据模型

未新增表、未改 migration：直接使用 002 的 `friends`、`friend_verifies`。
- `friends` 唯一 `(user_id, friend_id)`，双行模型；
- `friend_verifies` pending 生成列 `(pending_low_id, pending_high_id)` 唯一，阻止双向并发重复申请；
- 备注是方向性的，删除好友同时删除双向行。

## 关键设计

1. **并发同意**：`Respond` 在事务内 `SELECT ... FOR UPDATE` 锁申请行 → 校验 `receiver_id` → 校验 `status=0` → `INSERT ... ON DUPLICATE KEY UPDATE remark=remark` 写双向关系 → 更新状态。第二个并发请求阻塞后看到 `status=1`，返回 `1118`。
2. **重复/反向申请**：先在同一事务内检查好友关系（返回 `1114`），插入时的 1062 由 pending 唯一键兜底为 `1116`。
3. **Presence**：`GET /api/user/friends`、`friend_info` 逐好友读取 `gim:presence:user:{uid}`，单次 500ms 超时；Redis 故障降级 `isOnline=false`，不阻塞列表。未修改任何 Presence 写入路径。
4. **身份**：只读 Gateway 注入的 `User-ID`；缺失返回 HTTP 401 / `1203`。请求体不含任何“当前用户”字段。
5. **隐私**：`friend_info` 对非好友与不存在统一返回 `1115`，避免枚举；搜索只返回启用账号。

## 执行命令与结果

```powershell
go build ./...                                  -> PASS
go vet ./...                                    -> PASS
gofmt -l .                                      -> 无输出
go test ./internal/user/friend/... -count=1 -v   -> PASS（9 + 8 组测试）
go test ./internal/user/... ./tests -count=1     -> PASS
go test ./... -count=1                           -> PASS（全包）
```

**环境注意**：本机 `t.TempDir()` 默认落在 `C:\Users\...\Temp` 时被拒绝（`Access is denied`），config 类测试因此曾失败；将 `TMP`/`TEMP` 指向工作区内忽略目录 `K:\it\gim\.tmp\wintmp` 后全部通过。这是本机环境问题，不是代码回归。

## 未执行

- `server/tests/friend_integration_test.go` 需要 `GIM_ENV_FILE` + MySQL，本机 Docker 未运行、无 MySQL，**未执行**。因此“双并发同意”只有代码与断言，没有本轮真实运行证据。
- `go test -race` 未执行（Windows native race 不可用，Linux Docker 不可用）。

## Git

- commit：`a97bee6 feat(user): implement the friend loop api`
- push：见 Task 2 汇总提交后的统一推送记录。
- tag：无。

## 下一步

Task 2B：前端通讯录页面 + 测试。
