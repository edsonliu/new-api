# JDOTOKEN Seedance 第一阶段接口

本模块提供素材库、上传、任务管理及能力查询。生成、轮询、计费继续使用 new-api 的 Doubao 任务插件。默认关闭；本地实现不代表已经发布生产。

## 配置

复用原生管理员 `PUT /api/option/` 保存键 `JDOTOKENSeedance`，`value` 为 [config.example.json](config.example.json) 内容的 JSON 字符串。也支持启动环境变量 `JDOTOKEN_SEEDANCE_CONFIG`，数据库配置优先。不存储或复制渠道密钥；每次从现有渠道加载。

示例渠道编号对应 2026-10-22 的项目记录，仅为配置样例，发布前必须重新检查渠道身份、模型映射和分组。示例默认 `enabled:false`；确认后设为 `true`。`name` 是客户可见的服务名称，`provider`、`channel_id` 不返回给客户。每个渠道只能有一个服务绑定。`models` 使用本站请求模型名，`upstream_model` 必须匹配当前渠道映射；未填时视为同名。渠道停用、无模型权限、映射改变、凭据轮换、多 Key 渠道或覆盖鉴权请求头会使对应素材能力不可用。

样例未开放音频、自动入库、真人认证、frames、跨账号复制；这些功能需要独立验收。音频 URL 入库和音频生成引用分别由 `store_types`、`reference_types` 控制，不能混为一个开关。取消能力应只对已验证的渠道开放；移动云和 SDGO 样例保持关闭。

上传还需要三个启动参数：

```dotenv
JDOTOKEN_SEEDANCE_UPLOAD_DIR=/data/seedance-uploads
JDOTOKEN_SEEDANCE_PUBLIC_URL=https://www.jdotoken.com
JDOTOKEN_SEEDANCE_UPLOAD_SECRET=<独立生成的至少32字符随机密钥>
```

目录必须为专用持久化目录，不在静态网站根目录内，应用需要读写权限。通过应用签名接口访问文件，不增加 Nginx 任意文件写入接口。公网地址必须为可被渠道访问的 HTTPS Origin。多实例必须共享目录、数据库和签名密钥；不支持各节点独立的临时磁盘。存储未配置时 URL 入库仍可用，能力查询的上传开关为 false。

上传限制：PNG/JPEG/WebP、MP4/MOV；单文件最多 100 MiB；每用户最多 20 个未过期预留；全站最多 5 GiB 预留。预留通过数据库写锁串行化。签名和源文件保存 24 小时，申请新上传时清理过期文件，若无新申请则过期文件留待下一次清理。磁盘应另外设置配额和容量监控。签名下载地址属于临时访问凭据，反向代理访问日志应使用不含查询参数的 `$uri`，不要记录 `signature`。客户自行保存 `upload://` 引用，但它不是永久素材 ID。

## 接口清单

除签名文件 PUT/GET/HEAD 外均使用本站 `Authorization: Bearer <API_KEY>`，复用原生令牌认证。素材按用户隔离，同一用户的 API Key 共享素材；创建、修改、引用同时校验当前令牌可用模型及分组。列表只查询本站归属，不转发渠道共享账号的列表。

| 接口 | 用途 |
|---|---|
| `POST /doubao/api/v3/assets/?Action=...&Version=2024-01-01` | 素材/素材组十个 Action |
| `GET /v1/seedance/capabilities` | 当前令牌可用服务、模型和能力 |
| `POST /v1/seedance/uploads/presign` | 申请签名上传会话 |
| `PUT /v1/seedance/uploads/:id/content?...` | 一次性签名上传 |
| `POST /v1/seedance/uploads/:id/complete` | 确认上传并获取 `upload://` 引用 |
| `GET/HEAD /v1/seedance/uploads/:id/content?...` | 限时签名下载，支持 Range；通常交给上游使用 |
| `GET /doubao/api/v3/contents/generations/tasks` | 自己的任务列表 |
| `DELETE /doubao/api/v3/contents/generations/tasks/:id` | 非终态取消 / 终态本地隐藏 |
| `GET /v1/videos`、`DELETE /v1/videos/:id` | OpenAI 路径的列表 / 管理入口 |

原有视频 POST、单任务 GET、下载、草稿路径保持使用任务插件。新增列表采用 `page` / `page_size`（1–200）分页，任务按创建顺序排列；支持 `filter.task_ids`（重复参数或逗号分隔）、`filter.model`、`filter.status`、`filter.service_tier`。这是页面分页契约，不宣称实现 OpenAI SDK 的全部游标分页选项。`/v1/videos` 列表限定 Seedance/Doubao 任务，不是全平台所有视频厂商的管理接口。

## 素材管理

支持 `CreateAssetGroup` / `GetAssetGroup` / `UpdateAssetGroup` / `DeleteAssetGroup` / `ListAssetGroups` 和 `CreateAsset` / `GetAsset` / `UpdateAsset` / `DeleteAsset` / `ListAssets`。

所有创建必须带 `Idempotency-Key`。同一用户同一类资源、同一 Key 和同一请求重试返回已有资源；请求不同返回 409。上游超时、HTML 200、缺少 ID 等不确定结果不会自动重发创建，原 Key 返回 409，需要管理员核对上游后处理。响应 `Id` 永远为本站 `jdg_` / `jda_` ID。

同素材组内的创建、状态查询、修改和删除保留数据库互斥保护，适用于多实例部署。短时竞争由服务器自动等待，最长 15 秒；请求取消即停止等待，不会在后台继续排队提交。等待超时仍返回 `409 resource_busy`，并携带 `Retry-After: 2`；客户端可在至少 2 秒后加随机退避重试同一请求与同一 `Idempotency-Key`。这只代表等待资源锁超时，不代表上游审核失败。不同组独立处理，列表查询不参与此锁。获取锁后再次检查幂等记录，防止同时重试重复提交；`creation_unconfirmed` 仍需核对上游，不能当作 `resource_busy` 自动换 Key 重发。

2026-10-24 并发修复仅修改本目录的锁等待与幂等复查，未新增表、字段、依赖或上游核心接入点。升级时保留 `busy_until` 的条件更新语义，并回归同组并发提交/查询、幂等、取消、删除和跨用户隔离；旧应用可直接读取现有数据，发布回滚不需数据迁移。


```bash
curl "$BASE/doubao/api/v3/assets/?Action=CreateAssetGroup&Version=2024-01-01" \
  -H "Authorization: Bearer $JDOTOKEN_KEY" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: project-001' \
  -d '{"Service":"seedance-standard","Model":"doubao-seedance-2-0-mini-260615","Name":"项目素材"}'
```

创建组要求 `Service`、`Name`；`Model` 可选，建议传入以检查目标模型权限；可选 `Description`。第一阶段仅允许 `GroupType:AIGC`，不提供真人组认证。

```bash
curl "$BASE/doubao/api/v3/assets/?Action=CreateAsset&Version=2024-01-01" \
  -H "Authorization: Bearer $JDOTOKEN_KEY" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: image-001' \
  -d '{"GroupId":"jdg_REPLACE","AssetType":"Image","Name":"参考图","URL":"https://your-cdn.example/image.png"}'
```

创建素材要求自己的 `GroupId`、`AssetType` 和 `URL` 或 `UploadId` 二选一；使用组绑定的服务。URL 只允许公开 HTTP(S) 地址的 80/443 端口，并应用站点 Fetch 限制。不会将 `Moderation.Skip` 等审核跳过参数透传上游。上传成功不等于审核通过；创建响应可能是 `Processing`，轮询 `GetAsset` 至 `Active` 再生成。

Get/Update/Delete 使用 `{"Id":"..."}`。UpdateAsset 支持 `Name`；UpdateAssetGroup 支持 `Name`、`Description`。删除组会同时使组内本站素材不可用。单个 GetAsset 返回可用的上游预览 `URL`（可能限时）；列表中的状态是最后一次观察结果，使用 GetAsset 刷新，生成前也会检查上游最新状态。

列表支持 `Page` / `PageNumber`、`PageSize`、`NextToken`（下一页数字）、`GroupIds` / `GroupId`、`Status`、`AssetType`、`Service`；返回 `Items`、`TotalCount`、`PageNumber`、`PageSize`、`NextToken`。列表不枚举或导入发布前直接在上游创建的资产。

## 本地文件上传

```json
POST /v1/seedance/uploads/presign
{"service":"seedance-standard","model":"doubao-seedance-2-0-mini-260615","content_type":"image/png","size":12345}
```

1. 根据返回的 `upload_url` 和 `headers` PUT 原始文件，长度必须匹配。签名只能使用一次；校验文件签名和类型，失败需要重新申请。
2. 使用本站 API Key POST `complete_url`，确认成功得到 `source:upload://jdu_...`。确认接口检查归属、账号、有效期、磁盘文件及类型。
3. 对 Lingxi / 移动云，可用 `CreateAsset` 的 `UploadId` 入永久素材库；也可直接将 `upload://jdu_...` 放入生成媒体字段，平台转换为临时下载 URL。SDGO 仅支持后者，使用本站上传存储，不转发其专有 TOS 预签名协议。

用户不能通过上传会话读取别人的文件。服务端签名 URL 本身是临时持有者凭据，可供渠道无需本站 API Key 下载；不要公开分享。

## 引用与渠道绑定

```json
POST /doubao/api/v3/contents/generations/tasks
{
  "model":"doubao-seedance-2-0-mini-260615",
  "content":[
    {"type":"text","text":"让参考图中的小机器人挥手"},
    {"type":"image_url","image_url":{"url":"asset://jda_REPLACE"},"role":"reference_image"}
  ],
  "duration":4,
  "generate_audio":false
}
```

平台校验所有者、Active 状态、模型可引用类型及账号，将本站 ID 换成上游 ID，固定到对应渠道，禁止跨账号重试。最多 16 个引用，仍需满足上游更严格的输入数量限制。也支持 OpenAI JSON 的 `metadata.content` 和无文件的 multipart metadata；保持现有插件字段语义。可用顶层 `seedance_service` 显式选择服务；该字段会在发给渠道前移除。

启用后，结构化输入的裸上游 `asset://` ID 被拒绝，以防共享渠道账号越权。已有调用方需要通过本站创建资产并更新为本站 ID。此项是明确的接入迁移要求，不自动收编历史共享素材。换 Key / 换账号后默认停用旧映射，需人工确认并重新入库；不承诺同账号换 Key 自动继承。

## 取消、删除与账单

非终态 DELETE 先调用启用的上游取消接口，再通过现有轮询/终态 CAS/计费收尾确认状态。返回 `cancelled` 才表示确认取消，202 `cancel_requested` 表示待确认；任务已经完成返回 409，不因上游 DELETE 返回 200 就退款。成功、失败、取消与后台轮询竞争时使用原有 CAS 保障一次结算。

终态 DELETE 返回 `deleted:true, upstream_deleted:false, billing_records_retained:true`。本站 API 隐藏记录与结果，账单/审计保留；不声称上游物理文件删除。管理员任务审计仍可见。

取消仅对记录了本模块提交账号指纹的新任务启用；历史任务缺少可靠账号来源返回 409。素材与草稿混用时，草稿也必须具有同账号提交来源；历史无来源草稿不能与本站素材混用。新任务记录准确提交 Key，后台查询继续使用原账号。原模型计价、额度单位、余额语义和退款算法均复用上游。

## 升级与回滚

独立实现位于 `jdtoken/seedance/`，三张表 `jdtoken_seedance_resources`、`jdtoken_seedance_uploads`、`jdtoken_seedance_task_deletions`；资源表还保存上传容量预留锁。任务私有 JSON 新增可选账号指纹，无任务表列变更，无依赖升级。

上游接入点：`model/main.go` 增量迁移；`model/task.go` 私有 JSON 字段；`controller/relay.go` 提交来源记录；`middleware/distributor.go` 最终账号校验；三个路由注册位置的生成前处理；`router/main.go` 的独立 NoRoute 分发。i18n 标识及三个后端语言文件追加本模块错误文案。没有修改任务 JS 插件、通用渠道选择或计费算法。上游将来新增同路径 GET/DELETE 时需人工决定采用原生还是本模块行为；回归路由优先级、权限、账号固定、取消结算、上传链接及数据库迁移。

并发测试另外发现上游日志计数器竞争；`logger/logger.go` 改为原子计数和轮换标志，不改变日志格式。

部署前备份数据库和上传目录，在生产副本演练增量迁移；按项目 `jdtoken-vYYYY.MM.DD.N` Git tag 流程发布。回滚保留新增表与文件，不恢复覆盖发布后的业务交易；关闭功能配置会停止新素材请求，旧程序不能处理本站资产引用。回滚前处理在途任务，并考虑旧程序再次保存任务私有 JSON 会丢失新指纹，需要后续人工恢复或重新建立来源。

## 验证与安全范围

测试包括十个 Action、用户隔离、ID/账号绑定、模型映射变更、SSRF 输入、HTML 200、创建幂等、签名上传/下载/HEAD、重复 PUT、过期清理、分页跨批次、取消与轮询竞争、误报取消不退款、历史数据与索引保留。使用真实 SQLite/MySQL/PostgreSQL；提供方 HTTP 端为按已实测文档编写的契约测试服务。本次开发验证没有向生产发布，也没有新增真实计费视频。

认证复用现有 TokenAuth，没有新建登录或 Session 流程。设计参考 OWASP ASVS 5.0.0 的访问控制、文件处理及凭据保护要求，以及 [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[Authorization](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html) 指南；这是本功能范围内验证，不是全站 ASVS 合规认证。
