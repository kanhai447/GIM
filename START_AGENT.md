# START_AGENT — 如何启动 GIM Agent（三仓库版）

## 1. 推荐目录

```text
gim/
  AGENTS.md
  FINAL_AGENT_INSTRUCTION.md
  docs/
  reference/
    fim_server-main/
    fim_web-master/
    fim_admin-master/
  server/
  web/
  admin/
  agent_logs/
```

三个 reference 目录只作为只读参考实现。真正开发、提交 GitHub 的是全新编写的 GIM `server/web/admin`。不要把 reference 复制到新目录后改名。

## 2. 第一次只审计，不写业务代码

把下面内容发给 Agent：

> 你现在接手 GIM 项目。先完整阅读根目录 AGENTS.md、FINAL_AGENT_INSTRUCTION.md 和 docs 下全部文档，然后阅读三个参考源码：reference/fim_server-main、reference/fim_web-master、reference/fim_admin-master。
>
> 本轮禁止修改业务代码。
>
> 先输出：
> 1. Server 的 8 个逻辑服务及 API/RPC/基础设施关系；
> 2. 用户端 fim_web 的 HTTP/两条 WebSocket 调用方式；
> 3. fim_admin 的真实管理页面、真实后端接口与 Mock-only 功能；
> 4. FIM -> GIM 的保留/小改/重构清单；
> 5. 发现的并发、安全、数据模型、日志和部署问题；
> 6. 原源码中必须禁止复制到 GitHub 的敏感配置类别；
> 7. 每个主要模块准备“参考什么行为、GIM 如何自主实现”的对照方案；
> 8. Phase 0 的文件级执行计划和测试计划。
>
> 特别确认：reference 只读且禁止直接迁移；GIM 从空骨架自主编写；Admin 是独立第三个前端；/api/data/* 当前只有 Mock；Chat/Group 两条 WS 保留；Presence 只由 Chat WS 维护；Gateway V1 仍调用 Auth；Hub/Client + readPump/writePump；ACK/clientMsgId；session/unread；文件流式上传；Kafka Logs 保留但必须安全重构。
>
> 从开始执行任何操作起，每完成一个可独立验收操作，立即写入 agent_logs/operations/day-N/。没有日志的操作视为未完成。

## 3. 审计通过后再进入 Day 1

明确告诉 Agent“只执行 Day 1/对应 Phase”，每天完成后检查代码、测试、agent_logs、Git commit/push，再进入下一天。
