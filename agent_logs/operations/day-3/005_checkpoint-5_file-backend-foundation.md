# Day 3 / Checkpoint 5 — File Service Backend Foundation

- **时间:** 2026-10-13 13:13 +08:00
- **开始状态:** `main`，`HEAD == origin/main == 4134d30a542af8d0e495d47b3615bd1034498d51`，working tree clean。
- **授权范围:** 仅 File 后端基础、真实 MySQL/文件系统/Gateway 验收与 Chat/Group 回归；未进入 File Frontend、Chat/Group 文件消息、Admin File、S3/OSS 或 Day 4。

## File Architecture

- 自主实现 `UserFile -> FileObject -> Physical Object` 三层模型；Domain/Repository 不依赖 multipart 或 `*os.File`。
- `FileObjectRepository` 提供 SHA/ID/UID 查询、创建和引用计数；`UserFileRepository` 提供创建、owner-only ID/UID 查询、用户列表、存在性和删除引用；MySQL `Attach` 在一个事务内解析/创建两层记录。
- 复用既有 `005_file`：`file_objects.sha256 UNIQUE` 与 `user_files(user_id,file_object_id,original_name) UNIQUE` 已满足一致性和产品语义，因此无 migration 变化，未修改 001–006。

## Upload Flow / Streaming / MIME

- HTTP 使用 `multipart.NewReader` + `http.MaxBytesReader`；Service 使用 `io.LimitReader(max+1)` + 32 KiB `io.CopyBuffer`，一次流式写 temp、SHA-256 和最多 512 bytes MIME prefix。
- production File 路径不存在 `io.ReadAll`、`os.ReadFile`、`ParseMultipartForm` 或 `FormFile`；大文件内存不随正文线性增长。
- 默认 `FILE_MAX_UPLOAD_BYTES=52428800`（50 MiB），另有 1 MiB 有界 multipart overhead；配置范围 1 byte–1 GiB。
- MIME 以 `http.DetectContentType` 为准，不信任客户端声明；下载一律 `attachment + nosniff`。零字节文件允许并按空内容 hash 去重。

## Filename / Storage / Atomic Publish

- 文件名统一 `\ -> /` 后取 basename，拒绝空名、`.`/`..`、控制字符/CRLF、无效 UTF-8 和超过 255 bytes；Unicode 原样安全保存，只作为 metadata/header，不参与路径。
- temp 与 objects 均位于配置化 storage root；最终路径仅为 `objects/{sha[0:2]}/{sha[2:4]}/{sha256}`，原文件名不进入磁盘路径。
- temp 权限 0600、object 0640、目录 0750；startup 创建 root/temp/objects、执行写探针并清理 stale `upload-*.part`。
- `Sync -> Close -> same-filesystem Rename` 原子发布；同 hash 由有引用计数的 keyed lock 串行化，锁条目完成后释放。MySQL UNIQUE 仍是最终竞争屏障。

## Dedup / UserFile Semantics

- 50 并发同用户、同内容、同规范化名称：1 个 FileObject、1 个物理对象、1 个 UserFile，全部请求返回相同 UserFile ID。
- 同内容、同用户、不同名称：新增 UserFile；同内容、不同用户：各自 UserFile；物理对象和 FileObject 始终各 1 个。
- DB 已有对象但物理文件丢失时，相同内容重传会用 temp 原子修复；正常 download 遇到缺失物理文件返回安全 `2503`，不泄露路径。

## Compensation / Crash Model

- temp create/write、stream/size、commit/move 任一步失败均 Abort temp；commit 后 `Attach` 失败时，仅当本请求创建物理对象且 DB 再查仍无 FileObject 才删除。
- 已有共享对象 + 第二用户引用失败不会删除共享物理文件；`file_objects + user_files` 的 Attach 事务失败整体回滚。
- DB 完全不可查询或进程在 move 后崩溃时，保留按 hash 可寻址对象；后续同内容上传原位采用并补齐 DB，不生成随机不可寻址垃圾。V1 是单 File API + 本地 root；多实例共享 object storage/分布式 lock 后续设计。

## Ownership / List / Detail / Download / Delete

- 用户身份只读取 Gateway/Auth 覆盖的 `User-ID/Role`；真实 Gateway 测试验证外部伪造 `User-ID=999999/Role=1` 被替换为 token 用户。
- `GET /api/file/user-files`、`GET/DELETE /api/file/user-files/:id` 为 owner-only；`GET /api/file/:id-or-uuid` 数字按 UserFile ID，UUID 兼容 FileObject UID，但仍必须存在当前用户引用。
- 响应只包含安全 metadata/URL，不返回 root/storage path。下载流式输出，包含 server MIME、Content-Length、safe Content-Disposition 和 nosniff；`http.ServeContent` 同时提供标准 Range 行为。
- 删除只删当前用户 UserFile；共享 FileObject/物理对象保持。V1 延迟 GC，避免引用竞争误删；Admin File 不在本 Checkpoint。

## API / Runtime / Gateway

- 兼容 `POST /api/file/file`（field `file`）、`POST /api/file/image`（field `image`）、`GET /api/file/:id-or-uuid`；新增用户自助 `/api/file/user-files`，保留 `/api/file/files` 给 Day 4 Admin。
- File API runtime 读取 MySQL/storage/max/etcd 配置，注册 `file_api` lease，复用通用 graceful REST shutdown；Redis 不参与。
- 错误码：2501 invalid、2502 too large、2503 not found/unauthorized（统一防枚举）、2504 unavailable；内部 SQL/路径/磁盘错误不进入响应。

## Real Tests / Failure Injection / EXPLAIN

- 真实 MySQL + temp filesystem：50 concurrent dedup、same/different name、A/B ownership、ID/UUID access、content integrity、empty file、MIME sniff、delete shared safety、restart persistence、missing physical、stale temp、DB table failure compensation全部 PASS。
- failure injection：temp write、final move、FileObject/UserFile transaction、existing shared + second attach failure全部 PASS；oversize/filename traversal/CRLF/header safety PASS。
- EXPLAIN：SHA lookup、user list、owner ID lookup均执行成功；既有 `ux_file_objects_sha256`、`ix_user_files_user_created` 与 PK/owner predicate 可用。
- 真实 etcd + in-memory Auth/User RPC + Gateway + File + MySQL：upload/list/download、missing token、revoked token、spoofed identity全部 PASS。首轮因本机 etcd 未运行真实 FAIL；恢复 localhost 临时 etcd 后原样重跑 PASS，并精确停止/清理 2379/2380 runtime data。

## Regression / Quality / Security

- `go test ./... -count=1`（真实本地 MySQL/Redis/etcd）：PASS。
- File focused unit + integration：PASS；真实 integration coverpkg：62.2% statements；HTTP 66.1%、Service 70.7%（integration view）、MySQL Attach 81.8%。
- Private Chat / Group focused integration：3 rounds PASS；全量测试也包含两域回归。
- Web：12 files / 77 tests PASS；type-check PASS；production build PASS。
- `go vet ./...`、`gofmt -l .`、`git diff --check`：PASS。
- repository safety + staged private-key/token/credential pattern scan：PASS；candidate fixture files复核均为测试占位值；无 `.env.local`、storage、temp、coverage 或 reference 文件进入提交。
- Race：`NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；沿用已确认的 32-bit MinGW `cc1.exe: 64-bit mode not compiled in` 限制，本轮未重复执行；compatible environment/CI PENDING。

## Reference / Autonomous Implementation

- **FIM referenced:** `/api/file/image`、`/api/file/file`、`GET /api/file/:name`、Web 的 `src/url` 上传语义、Admin 存在文件列表/删除能力。
- **未复制:** FIM handler/model/file utility 均未迁移；reference 保持只读且不是编译依赖。
- **GIM improvements:** 替换 FIM `io.ReadAll + MD5 + 用户目录/原文件名语义 + hash 命中直接复用他人 FileModel + os.ReadFile 下载 + delete hook 直接删物理文件`，改为 streaming SHA-256、内容寻址、FileObject/UserFile、owner-only 下载、共享安全删除、结构化补偿与安全 Header。

## Files / Database / Protocol

- 新增：`server/internal/file/{domain,repository,storage,service,transport,config}`、`server/cmd/file-api`、File unit/integration/Gateway tests。
- 修改：`.env.example`、File backend/API/core-flow docs、Gateway forged identity regression。
- Database：无 schema/migration 变化；隔离测试库全部自动 DROP。
- Redis/WS：无变化；未实现任何 Chat/Group 文件消息。

## Git / Result / Boundary

- Implementation commit：`5142da94f5ee9fb5d776c3438e1a2995bc6a50bc feat(file): add secure streaming file service`。
- Implementation push：SUCCESS，`origin/main`。
- Log/status commit：本记录所在 `docs(day3): record file service checkpoint` 提交。
- `FILE BACKEND V1 = FUNCTIONAL`。
- 下一计划：File Frontend Integration + Day 3 Final Acceptance，必须等待用户另行授权；严格未进入该阶段、Chat/Group 文件消息、Admin File 或 Day 4。
