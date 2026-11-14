# 001 / Task 1 — V2 收口评估文档

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = abc3e72`，工作区干净，五个旧 tag 未移动。
- **结果**：新建 `docs/v2-scope.md`；未改业务代码。

## 文档结构

1. V2 已实现（核心四 Phase）：图片 `type=2`、普通文件 `type=3`、撤回 `type=4`、好友上线提醒（单机）——每项列关键实现与验证证据。
2. V2 后期/不做：`user_confs`、管理员撤回、多实例 Presence、好友申请实时推送、图片/文件吞吐、语音/视频/@/图文、WS Ticket/Gateway 本地验签、Kafka outbox/多实例 WS 路由；逐项写理由。
3. 已知边界：K-14、K-15、K-16、E-8、E-9，引用 `known-issues.md`。
4. 验证状态摘要：单测/集成/前端/race/Compose/E2E/Kafka 为 PASS；完整性能阶梯、soak/restart、图片文件吞吐、远端 CI 为 NOT RUN/NOT OBSERVED；明确禁止“全量验证通过/百万连接”表述。
5. Tag 语义：v1–v1.4 使用实际 peeled commit；v2.0 指向本轮收口 commit（以 `git rev-list -n 1 gim-v2.0` 为准）。
6. 面试/简历表述建议。

## 引用关系

- `docs/validation-status.md`：详细命令与结果来源。
- `docs/known-issues.md`：边界条目来源。
- `docs/06_WEBSOCKET_PROTOCOL.md` §5.6/§5.7：撤回/好友提醒协议。
- `README.md`：V2 范围入口。

## 一致性检查

- 与 `known-issues.md` 的 K-14/K-15/K-16/E-8/E-9 表述一致。
- 与 `validation-status.md` 的 PASS/NOT RUN 项一致；未新增未验证声明。
- 未出现“全量验证通过”“支持百万连接”。

## Git

- commit：`docs: define V2 scope and boundaries`（本记录所在提交）。
