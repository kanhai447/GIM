# Day 14 — 撤回实现（2026-11-14）

## 今日目标

实现撤回消息 `type=4` 完整闭环：后端校验/落库/ACK/推送/历史替换/文件授权撤销，前端撤回入口与占位替换，单测 + 集成 + E2E，文档改为“已实现”；不打 tag。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） |
| 撤回集成（真实 MySQL/Redis/WS） | **PASS** | ACK、接收、历史占位、重复撤回、非本人/非参与者/tombstone/窗口、文件下载撤销 |
| 文件授权联动 | **PASS** | 撤回后接收方 `ErrNotFound`（2503），owner 仍可下载 |
| Web 类型/测试/构建 | **PASS** | `vue-tsc`；`vitest` 21 files / 131 tests；`vite build` |
| 浏览器撤回 E2E | **PASS** | A 撤回文本→B 占位；A 撤回文件→B 卡片消失、下载 2503；owner 200 |
| 撤回窗口浏览器 E2E | **NOT RUN** | 默认 120s；单测/集成覆盖 2220 |
| 撤回代码 race | **NOT RUN** | 本轮未要求；上一轮全量 race 覆盖文件/图片阶段 |

## 关键过程

1. 协议已在前一轮定义；本轮实现解析（type=4 + withdrawMsg）、RecallRepository、校验顺序、事务落库/幂等、历史/session 替换。
2. 文件授权查询增加 `NOT EXISTS` 排除被撤回原消息；保持 `LIMIT 1`，追加安全测试。
3. 前端：撤回入口（120s UX）、type=4 解析、发送/ACK/推送替换、占位渲染、合并隐藏重复撤回行。
4. E2E 首轮因定位器随原文替换失效而假失败；截图确认成功后改用 `.withdrawn` 计数并重跑 PASS。
5. 文档从设计改为实现；未打 tag。

## 修改文件

- 后端：两个 protocol、两个 message service/repository（+mysql）、两个 config、两个 cmd、file 授权 SQL、env 模板、测试。
- 前端：类型、utils、两个 socket、两个 store/reconcile、MessageBubble、两个消息列表、两个 view、测试。
- 文档：docs/06/05/07/04、README、validation-status、known-issues。
- 日志：`agent_logs/operations/day-14/001–004`。

## 问题与修复

1. 授权查询必须排除被撤回原消息；已用 `NOT EXISTS` + 安全测试覆盖。
2. E2E 定位器假失败；非产品缺陷，脚本修正后 PASS。
3. 并发不同 clientMsgId 撤回同一消息在无唯一索引下可能产生两条撤回消息；记录 K-14，不新增索引。

## 当日验收

- 撤回闭环完成；无 schema 变更；三个 tag 未移动；未打新 tag。
- 全量后端/前端测试 PASS；E2E PASS；窗口 E2E 与撤回 race 如实 NOT RUN。

## 下一步

- 好友上线提醒 / `user_confs` / 图片与文件性能；或考虑撤回窗口短配置 E2E、撤回 race。
- 是否在 Phase 整体验收后打 `gim-v2.0`，等确认。
