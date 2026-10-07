# Day 1 / Checkpoint 1 — 安全配置检查与独立工程骨架

- **日期:** 2026-10-07
- **Phase:** Phase 0
- **本次目标:** 建立 `server/`、`web/`、`admin/` 的 GIM 自有空骨架，并把仓库安全门禁固化为可重复执行的检查脚本。

## 参考了 FIM 的什么

- 仅参考了 Auth/User/Chat/Group/File/Settings/Logs/Gateway 的服务边界。
- 参考了用户 Web 与 Admin 必须是两个独立应用的事实。
- 参考了原 Admin Dashboard 中 `/api/data/*` 仅为 Mock 的边界。

## GIM 如何自主实现

- 新建独立 Go module `github.com/kanhai447/GIM/server`，未引用 `reference/`。
- 使用空目录占位和最小可编译 package 建立后端边界，没有复制 FIM 的入口或业务代码。
- `web/` 和 `admin/` 仅建立 GIM 自有源码边界，实际 Vue/Vite 工程留到 Checkpoint 8。
- 新增 PowerShell 安全脚本，实际验证 ignore 规则、本地配置完整性与受跟踪敏感路径；敏感字段只输出 `[REDACTED]`。

## 相比 FIM 的调整

- `reference/` 永久排除在 GIM Git 与编译依赖之外。
- 本地 Secret 统一保存在被忽略的根 `.env.local`，提交配置仅允许占位值。
- 从工程初始化阶段即明确 Web/Admin 分离和 Mock-only Admin Dashboard 边界。
- 不继承原项目的目录、硬编码配置、证书或作者私有信息。

## 新增/修改文件

- `agent_logs/CURRENT_STATE.md`
- `scripts/check-repository-safety.ps1`
- `server/go.mod`
- `server/README.md`
- `server/internal/project/doc.go`
- `server/cmd/.gitkeep`
- `server/migrations/.gitkeep`
- `server/tests/.gitkeep`
- `web/README.md`
- `web/src/.gitkeep`
- `admin/README.md`
- `admin/src/.gitkeep`
- `agent_logs/operations/day-1/001_checkpoint-1_project-skeleton.md`
- `agent_logs/day-1.md`
- `agent_logs/AGENT_WORKLOG.md`
- 既有 `agent_logs/operations/pre-day1/` 安全操作记录纳入本次审查范围。

## 执行命令

- `scripts/check-repository-safety.ps1`
- `gofmt -l .`
- `go list -m`
- `go test ./...`
- `go vet ./...`
- `rg ... reference|fim_ server/`
- `git diff --check`
- `git diff --stat`
- `git diff`

## 测试结果

- Repository safety check: PASS。
- Go module: `github.com/kanhai447/GIM/server`。
- `gofmt`: PASS。
- `go test ./...`: PASS。
- `go vet ./...`: PASS。
- reference 编译依赖扫描: PASS，0 命中。
- `server/`、`web/`、`admin/` 边界检查: PASS。

## 问题与根因

- 首次测试命令从 `server/` 目录执行，却使用了根目录相对路径 `.\scripts\check-repository-safety.ps1`，导致脚本未找到，后续测试未继续。

## 修复方式

- 将调用路径修正为 `..\scripts\check-repository-safety.ps1`，重新执行完整测试集并全部通过。

## Git

- **Commit hash:** PENDING（测试与日志完成后创建）。
- **Push:** PENDING，禁止在真实成功前标记 SUCCESS。

## 下一步

- 完成 staged diff 与 Secret Scan，提交并 push Checkpoint 1。
- 稳定节点建立后进入 Checkpoint 2：Server 公共基础模块。
