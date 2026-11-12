# 001 / Task 1A — Linux Docker race 全量验证

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = bce78cc`，工作区干净；本轮开始时 Docker daemon 未运行。
- **结果**：全量 `go test -race ./... -count=1` **PASS**（exit 0，无 race warning）。

## 尝试顺序（如实记录）

### 1. 本机 Windows native race — 失败（工具链）

```powershell
$env:CC='D:\describe\dc\MinGW\bin\gcc.exe'
go test -race ./internal/chat/... ./internal/user/friend/... -count=1
# -> cc1.exe: sorry, unimplemented: 64-bit mode not compiled in

$env:CC='C:\PROGRA~1\mingw64\bin\gcc.exe'
go test -race ./internal/chat/ -count=1
# -> fatal error: stddef.h: No such file or directory（64-bit MinGW 缺 libc headers）
```

### 2. WSL 尝试 — 不可用

Ubuntu WSL 存在，但未安装 Go/gcc，且 `sudo` 需要密码，无法非交互安装编译器。未使用。

### 3. Docker 尝试 — 成功

启动 Docker Desktop 后（29.4.0 Linux engine）：

```powershell
docker run --rm `
  -v 'K:\it\gim\server:/src' `
  -v 'C:\Users\hfhy0\go\pkg\mod:/gomodcache' `
  -w /src `
  -e CGO_ENABLED=1 -e GOMODCACHE=/gomodcache -e GOPROXY=off -e GOFLAGS=-mod=mod `
  golang:1.25-bookworm bash -c "go version; go test -race ./... -count=1"
```

> 首次未挂载 module cache 时，容器内访问 `proxy.golang.org` 被拒绝（`dial tcp ...: connect: connection refused`），这是网络/依赖下载失败，不是 race 结果；改为挂载本机已下载的 module cache 并 `GOPROXY=off` 后重跑。

## 真实结果

```text
go version go1.25.14 linux/amd64
...
ok  github.com/kanhai447/GIM/server/internal/auth              1.015s
ok  github.com/kanhai447/GIM/server/internal/chat              2.731s
ok  github.com/kanhai447/GIM/server/internal/chat/message/admin/mysql 1.083s
ok  github.com/kanhai447/GIM/server/internal/user/friend       1.013s
ok  github.com/kanhai447/GIM/server/internal/user/friend/transport/http 1.031s
...（全部包 ok）
exit code 0
```

重点包 `internal/chat/...` 与 `internal/user/friend/...` 均通过；未出现 `WARNING: DATA RACE`。

说明：容器内未设置 `GIM_ENV_FILE`，因此 `server/tests` 的 20 个真实依赖集成用例按设计 skip；这只是 race 套件，不代表集成验证。

## 问题与修复

- 首次 race 容器因模块下载网络失败而 `[setup failed]`；挂载 Windows module cache + `GOPROXY=off` 修复。
- Windows native 与 WSL 两条路径均不可用，已在 `docs/known-issues.md` E-5 记录。

## 验收

- race：**PASS**（Linux Docker，全量包）。

## 日志

- 本文件；未修改仓库代码。
