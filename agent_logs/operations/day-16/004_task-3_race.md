# 004 / Task 3 — 好友上线提醒全量 race

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：Linux Docker 全量 `go test -race ./... -count=1` **PASS**（exit 0，无 `WARNING: DATA RACE`）。

## 命令

```powershell
docker run --rm `
  -v 'K:\it\gim\server:/src' `
  -v 'C:\Users\hfhy0\go\pkg\mod:/gomodcache' `
  -w /src `
  -e CGO_ENABLED=1 -e GOMODCACHE=/gomodcache -e GOPROXY=off -e GOFLAGS=-mod=mod `
  golang:1.25-bookworm bash -c "go version; go test -race ./... -count=1"
```

## 真实结果

```text
go version go1.25.14 linux/amd64
ok  github.com/kanhai447/GIM/server/internal/chat/friendnotify   1.227s
ok  github.com/kanhai447/GIM/server/internal/chat               2.744s
ok  github.com/kanhai447/GIM/server/internal/chat/presence      1.454s
ok  github.com/kanhai447/GIM/server/internal/user/friend        1.014s
...（全部包 ok）
exit code 0
```

- 无 DATA RACE；无失败包。
- `server/tests` 未设 `GIM_ENV_FILE` 时按设计 skip 集成用例。

## 验收

- 好友提醒 + 全量 race：**PASS**。
