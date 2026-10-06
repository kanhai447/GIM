# 09 分阶段实施计划（三仓库自主实现版）

## Phase 0：三仓库审计与 GIM 空骨架

阅读三个只读参考项目：`reference/fim_server-main`、`reference/fim_web-master`、`reference/fim_admin-master`。先梳理业务、接口、依赖和已知问题，再从空目录创建 `server/`、`web/`、`admin/`。

禁止整文件/整目录复制，禁止批量 `fim -> gim` 改名，禁止把 reference 作为编译依赖。Go module、Vue package name 和目录从创建之初直接使用 GIM 命名。

准备 MySQL、Redis、etcd；自主实现最小 Gateway/Auth/User 骨架并跑通登录链路，再创建 Chat/Group/File/Settings 的最小服务骨架。Kafka/Logs 可准备依赖，但不阻塞核心登录。

验收：
- GIM 是独立新代码树；
- 删除 `reference/` 后 GIM 仍可构建；
- Gateway -> Auth 登录成功；
- 三参考仓库角色与 Mock-only Admin Dashboard 边界已明确；
- 无作者私有 IP、真实 Secret、私钥进入 GIM commit；
- 操作日志记录“参考行为 -> GIM 自主实现”。

## Phase 0.5：基础工程规范

确认 `server/ web/ admin/` 均使用 GIM 命名、独立配置、统一错误/响应约定和启动说明。全量 build 通过后进入 Phase 1。

## Phase 1：数据库与基础工程质量

自主设计 migration、`client_msg_id`、`chat_session/group_session`、索引、Redis key helper、context 传播和配置样例。修正原群消息删除过滤的用户隔离语义。

## Phase 2：Chat Hub/Client + Presence

根据 GIM 设计重新实现 Hub/Client、readPump/writePump、Ping/Pong、多端连接、0->1/1->0 Presence、Origin 校验。不得复用原全局普通 map + 直接 Conn.Write 的实现。

## Phase 3：私聊 ACK/幂等/session/unread

实现独立 Service 层：clientMsgId、事务、ACK、session、未读/read；保持必要的业务协议兼容。

## Phase 4：Group Hub/Client

重新实现 Group Hub/Client/PingPong。Group WS 禁止修改全局 Presence。

## Phase 5：群消息可靠性

实现幂等、ACK、权限/禁言、group_session、未读/read、@、撤回。

## Phase 6：文件服务

重新实现流式 SHA-256、temp + atomic move、FileObject/UserFile、大小/MIME、安全目录权限和失败清理。禁止复制原 `io.ReadAll` + MD5 + 直接所有权模型。

## Phase 7：用户 Web

参考原页面功能和交互，在新的 `web/` 中自主实现/组织 BaseSocket、重连、pending ACK、发送状态、未读、在线状态和 API 类型。可以保持主要 UI 信息架构，但不得把原工程整文件迁移后改名。

## Phase 7.5：Admin 与日志

参考原 Admin 的页面流程和 API 语义，在新的 `admin/` 中自主实现；接入真实 User/Chat/Group/File/Settings/Logs 管理 API并保留服务端 Role 权限。Kafka Logs 使用“共享 Producer + 每请求独立结构化 LogEvent + 敏感字段脱敏”；Admin 安全渲染日志。隐藏或明确标记原 `/api/data/*` Mock-only Dashboard，不新增未经确认的数据微服务。

## Phase 8：全量测试、部署、README、简历材料

运行后端/race/integration 测试；用户端和 Admin build；Docker/启动说明；GitHub secret scan；整理实际架构、优化点和真实面试材料。

每个 Phase 完成后必须停止，等待用户验收。
