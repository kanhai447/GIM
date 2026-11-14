# 006 / Task 4 — 文档与验证状态（文件消息）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 5e2abb3`；E2E 已完成。
- **结束状态**：本任务提交 `docs: record file message support and validation`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/06_WEBSOCKET_PROTOCOL.md` | 类型表加入 `type=3`；新增 §5.5 普通文件消息（payload、校验、错误码、授权、兼容性） |
| `docs/05_HTTP_API.md` | 上传与 `type=3` 引用关系；`2215–2217`/`2416–2418` 错误码；下载授权覆盖文件消息 |
| `docs/07_CORE_FLOWS.md` | 新增 §2.2 普通文件消息流程；下载授权步骤 |
| `docs/04_DATABASE_DESIGN.md` | `msg` JSON type=3 结构；`[文件]` preview；无新表/列 |
| `README.md` | V2 Phase 1 增加普通文件；未实现列表移除普通文件；验证状态加入文件单测/集成/E2E；限制说明更新 |
| `docs/validation-status.md` | 新增“V2 Phase 1 普通文件消息（2026-11-14）”；NOT RUN 增加文件消息性能 |
| `docs/known-issues.md` | K-1/K-6 更新（文本/图片/文件已实现）；新增 V-13（文件消息集成+E2E）；V-12 改为图片/文件性能 |

## 事实核对

- 文件消息无 schema/索引/迁移变更：复用 `msg` JSON 与 `user_files`/`file_objects`。
- 文件消息不限制 MIME 前缀，大小受 `FILE_MESSAGE_MAX_BYTES`（25 MiB）限制；与图片的 `image/*` 校验区分。
- 下载授权复用图片语义，授权 SQL 同时匹配 `imageMsg.fileID` 与 `fileMsg.fileID`。
- 群聊文件消息为单测覆盖；浏览器 E2E 覆盖私聊文件发送/下载/非参与者。
- 未打 `gim-v2.0`；`gim-v1.2` 已在前序任务推送。

## 测试

```text
go build ./... / go vet ./... / gofmt -l .        PASS / PASS / 无输出
go test ./... -count=1                            PASS（69 packages）
vue-tsc --noEmit                                  PASS
vitest run                                        PASS（21 files / 122 tests）
```

文件集成/E2E 证据见 003/005。

## Git

- commit：`docs: record file message support and validation`（本记录所在提交）。
