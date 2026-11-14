# 003 / Task 2B — 面试问答：群聊、文件、撤回、好友提醒、Admin、审计、migration

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：完成第 3.8–3.16 节；未改代码/旧文档。

## 覆盖

- 3.8 群创建/成员/角色/邀请/申请/禁言
- 3.9 群文本/图片/文件消息
- 3.10 群撤回
- 3.11 Session/未读/已读/置顶/删除（重点）
- 3.12 文件上传/下载授权（重点）
- 3.13 好友上线提醒（重点）
- 3.14 Admin（重点）
- 3.15 审计（重点）
- 3.16 Migration

每节含固定结构；引用 `group/...`、`file/...`、`chat/friendnotify/...`、`logs/...`、`migrations/runner.go` 的真实函数。

## 口径

- 多实例 Presence 写 K-15；offline K-16；E2E K-17；Group 1213 A5/K-10；删除表 A1/A2。
- Admin 错误码写 2230–2232 / 2320–2322（A4 修复后）。
- 文件授权无索引/无限速 E-8，浏览器缓存 E-9。

## Git

- commit：`docs: add interview q&a for group, file and admin`（本记录所在提交）。
