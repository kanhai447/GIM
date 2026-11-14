# Day 11 — 图片 Phase 1 收尾（2026-11-14）

## 今日目标

补齐图片消息 Phase 1 验证缺口：文件下载授权审计/专项测试、图片代码 race、群聊图片 WS E2E；更新文档；不进入下一 Phase、不打 tag。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| 文件下载授权审计 | 完成 | tombstone/退群/所有者语义已满足；唯一差异是 `COUNT(*)` → 改为 `SELECT 1 … LIMIT 1` |
| 授权专项测试（真实 MySQL） | **PASS** | 所有者、私聊接收方、群成员、非参与者 `2503`、私聊/群 tombstone、退群后 `2503`、未知 fileID `2503` |
| 文本路径测试 | **PASS** | 文本内容提及 fileID 不授权；`MessageBubble` 文本渲染不调用下载 API |
| 图片代码 race（Linux Docker） | **PASS** | 全量 `go test -race ./...`，exit 0，无 `WARNING: DATA RACE` |
| 群聊图片 WS E2E | **PASS** | A 建群邀请 B/C → 发图 → B/C 收到并预览 → 非成员 D `2503` → B 退群后 `2503` → owner 仍可下载 |
| 图片吞吐/预览性能 | **NOT RUN** | 本轮未做 |

## 关键过程

1. 审计 a1ecf64 实现：私聊/群聊均过滤 `admin_deleted_at`；群成员通过 `group_members` JOIN，退群/踢人物理删除成员行；非参与者统一 `2503`；文本 JSON 路径为 NULL。
2. 唯一不符合审计要求的是 `COUNT(*)`；改为 `SELECT 1 … LIMIT 1`，授权条件不变，无 schema/索引变更。
3. 新增真实 MySQL 授权测试矩阵（`TestImageDownloadAuthorizationSemantics`）与文本路径测试（`TestTextMessageDoesNotGrantImageDownload`）。
4. Linux Docker 全量 race PASS。
5. 群聊图片 E2E：首轮“退群后 200”经 SQL 直查确认是浏览器缓存（`group_hits=0`），改 `cache: 'no-store'` 后 `2503`；服务端语义正确，记录为客户端缓存边界。
6. 清理：Compose down、Docker Desktop 停止、临时脚本/SQL 删除。

## 修改文件

- 代码：`server/internal/file/repository/mysql/repository.go`（LIMIT 1）。
- 测试：`server/tests/file_authorization_integration_test.go`、`server/tests/file_text_authorization_integration_test.go`、`web/src/components/chat/MessageBubble.test.ts`。
- 文档：`docs/05`、`docs/06`、`docs/07`、`docs/known-issues.md`、`docs/validation-status.md`、`README.md`。
- 日志：`agent_logs/operations/day-11/001–006`、`day-11.md`、`CURRENT_STATE.md`。

## 问题与修复

1. **浏览器缓存导致 E2E 假失败**：退群后 B 仍拿到缓存图片；改为 `cache: 'no-store'` 后正确 `2503`。服务端未放宽；已记录缓存边界。
2. 授权查询 `COUNT(*)` 全量统计；改为 `LIMIT 1`，行为不变。
3. 无 DATA RACE；无需扩大修复范围。

## 当日验收

- 图片 Phase 1 验证缺口补齐：race、群聊 E2E、授权专项测试全部 PASS。
- 无 schema 变更、无新 tag；`gim-v1`/`gim-v1.1` 未移动。

## 下一步

- 可选：图片吞吐/预览性能；之后进入下一 V2 Phase（普通文件/语音/视频、回复/引用/撤回/@、好友上线提醒、`user_confs`）。
- 是否创建 `gim-v1.2` / `gim-v2.0` tag，待用户确认。
