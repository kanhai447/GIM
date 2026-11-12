# 005 / Task 4 — 文档与验证状态

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = a1ecf64`（接收方下载修复）。
- **结束状态**：本任务提交 `docs: record image message support and validation`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/06_WEBSOCKET_PROTOCOL.md` | 图片消息 payload/校验/错误码/兼容性（任务一已写入）；补充“接收方可按 fileID 下载，文件服务仅对消息参与者放行” |
| `docs/05_HTTP_API.md` | 上传与消息引用的两步关系、`2211–2214`/`2412–2415` 错误码表；`GET /api/file/:id` 授权说明更新为“所有者或消息参与者” |
| `docs/07_CORE_FLOWS.md` | 新增 §2.1“图片消息”完整流程；修正群聊“group.send 未实现”的过期表述 |
| `docs/04_DATABASE_DESIGN.md` | `chat_messages.msg` / `group_messages.msg` 的 type=1/2 结构；图片仅元数据、无新表/列；preview 为 `[图片]` |
| `README.md` | V2 Phase 1 图片已实现；未实现列表移除图片；验证状态新增图片单测/集成/E2E 与 Web 21/114；未验证项补充图片 race/群聊 E2E |
| `docs/validation-status.md` | 新增“V2 Phase 1 图片消息”验证表（单测/聊天集成/群聊单测/Web/E2E + NOT RUN）；历史记录补充 day-10 |
| `docs/known-issues.md` | K-1/K-6 更新（图片已集成）；新增 K-11（尺寸仅客户端提示）、V-7/V-8（已验证）、V-9/V-10（未验证）、E-8（下载授权 JSON 查询无索引） |

## 事实核对

- 图片消息**无 schema 变更**：复用 `msg` JSON 与 `user_files`/`file_objects`。
- 群聊图片：单测覆盖解析/归一化/幂等/错误码；群聊图片 WS E2E **未单独运行**，文档如实标注。
- 图片代码 race **NOT RUN**（race 在图片功能之前执行），文档与 known-issues 如实标注。
- 浏览器图片 E2E 首轮失败暴露“接收方无下载权限”；修复提交 `a1ecf64` 后 PASS，validation-status 已注明。
- 未修改 `docs/00_DECISIONS.md`；未打 tag。

## 测试

本任务只改文档：

```text
git diff --check -> PASS
```

代码测试在 002/003/004 中执行；最终全量测试见本轮汇总。

## 操作日志

- `agent_logs/operations/day-10/001_task-1_protocol-design.md`
- `002_task-2_backend-image.md`
- `003_task-3_frontend-image.md`
- `004_task-3b_image-e2e-and-access-fix.md`
- `005_task-4_docs-and-validation.md`（本记录）

## Git

- commit：`docs: record image message support and validation`。
