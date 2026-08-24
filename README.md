# 声景证据册

声景证据册是面向生态声学研究团队的本地治理服务。它把野外录音从批次登记、内容校验、双人盲标、共识计算和分类学仲裁，持续推进到数据集冻结与科研发布，并使用 SHA-256 内容寻址、JSON Lines 事件哈希链、冻结清单和发布证书保留可验证证据。

服务只提供 JSON HTTP API 与 multipart 录音上传入口，不依赖外部数据库或第三方服务。批次写操作按批次串行，不同批次可并行；所有写入使用 `X-Expected-Version` 检测过期版本，并使用 `Idempotency-Key` 保证重试返回首次结果。

## 构建、运行与测试

标准构建：

```bash
go build ./cmd/soundledger
```

标准运行：

```bash
go run ./cmd/soundledger -addr=127.0.0.1:19081 -data-dir=./data
```

默认监听地址是 `127.0.0.1:19081`，不会绑定 `0.0.0.0`。也可以不传 `-addr`，通过 `PORT` 指定端口；例如 `PORT=19123` 会绑定 `127.0.0.1:19123`。显式 `-addr` 的优先级高于 `PORT`，且服务仅接受回环监听地址。

标准测试：

```bash
go test ./...
```

运行真实网络主流程自检：

```bash
go run ./cmd/soundledger -selfcheck -addr=127.0.0.1:19081
```

自检会建立临时数据目录，在实际配置地址启动 HTTP 服务，依次完成创建批次、multipart 上传、两人盲标、争议生成、匿名证据查询、专家仲裁、冻结、发布和事件/证书查询，然后在超时内关闭服务并自行退出。

## API 使用约定

写请求必须携带以下请求头：

- `X-Actor-ID`：操作者编号。
- `X-Role`：角色，支持 `administrator`、`annotator`、`arbiter` 和 `publisher`。
- `X-Expected-Version`：提交前看到的批次版本；创建批次使用 `0`。
- `Idempotency-Key`：本次业务操作的唯一幂等键。
- `X-Request-ID`：请求关联标识，会进入审计事件。

主要端点如下：

- `POST /api/v1/batches`：登记地点边界、采样时段、授权声明和双人标注规则。
- `POST /api/v1/batches/{batchID}/clips`：以 multipart 的 `audio` 字段上传录音，并提交 `clipId`、`recordedAt`、`durationMillis`、`mediaType`、`recorderCode` 和 `habitatNote`。
- `POST /api/v1/batches/{batchID}/clips/{clipID}/annotations`：提交独立物种标注。
- `POST /api/v1/batches/{batchID}/consensus`：比较两份标注，形成共识或结构化争议。
- `GET /api/v1/batches/{batchID}/disputes/{disputeID}`：查询去除标注员身份的仲裁证据。
- `POST /api/v1/batches/{batchID}/disputes/{disputeID}/arbitrate`：给出最终分类和理由，或退回指定片段重新标注。
- `POST /api/v1/batches/{batchID}/freeze`：校验完整性、双标覆盖、争议和授权并固化清单。
- `POST /api/v1/batches/{batchID}/publish`：为冻结清单签发不可变发布证书。
- `GET /api/v1/batches/{batchID}/manifest`、`GET /api/v1/batches/{batchID}/certificate` 和 `GET /api/v1/batches/{batchID}/events`：查询发布证据。

JSON 错误使用稳定的 `error.code`、`error.message`、可选 `error.field` 和 `error.requestId`。版本冲突、重复内容和非法状态返回 `409`，输入校验失败返回 `422`，越权操作返回 `403`。

冻结请求会返回结构化 `validation`。校验通过时同时返回 `manifest`；校验未通过时，该次校验仍作为成功执行的业务命令返回 `200`，批次进入 `remediation`，并在 `batch.validationIssues` 与 `validation.issues` 中携带可整改的问题清单。

## 本地证据存储

`-data-dir` 目录包含以下内容：

- `objects/`：按 SHA-256 摘要分层保存的录音正文；上传先写临时文件并 `Sync`，再原子落位。
- `events.jsonl`：包含连续序号、前序摘要和当前摘要的领域事件日志，是恢复依据。
- `projections/`：每个批次的查询投影，通过临时文件 `Sync` 后原子替换；缺失或损坏时由事件日志重建。
- `idempotency.json`：幂等键与首次响应索引。

服务启动时会验证事件 `schemaVersion`、序号和完整哈希链。校验失败会拒绝启动，避免在证据已损坏的情况下继续接受写入。
