# Config Snapshot Registry

Config Snapshot Registry 是一个保存环境配置快照并计算结构化差异的 Go 服务。它面向需要追踪配置变化的工程团队，支持快照登记、按服务和环境查询、嵌套配置差异、审计记录和本地持久化。

项目不是单文件 demo：领域模型、确定性差异引擎、乐观并发存储、JSON 文件存储、HTTP 适配器和后台保留期清理器各自独立，当前版本使用 Go 标准库，不依赖第三方包。

## Run

```bash
go run ./cmd/server
go test ./...
go test -race ./...
go vet ./...
```

环境变量：`LISTEN_ADDR`（默认 `:8080`）、`STATE_FILE`（默认 `./data/state.json`）、`RETENTION_INTERVAL`（默认 `1h`）、`RETENTION_AGE`（默认 `720h`）。

## API

- `POST /v1/snapshots` 登记一个快照；相同 ID 和相同内容重复提交保持幂等，不同内容复用 ID 会返回冲突
- `GET /v1/snapshots` 按服务、环境过滤并按采集时间倒序返回快照
- `GET /v1/snapshots/{id}` 查询快照详情
- `GET /v1/diffs?from=<id>&to=<id>` 返回嵌套配置项的新增、删除和修改
- `GET /v1/impact?service=<name>&environment=<env>&from=<id>&to=<id>` 变更影响评估：比较同一服务和环境下的两份快照，列出配置变更和可能受影响的服务
- `GET /v1/audit` 查询快照写入和清理事件

差异路径使用点号表示嵌套对象，例如 `database.pool.max`。数组被当作整体值比较，输出顺序按路径稳定排序，方便审计和重放。

## 变更影响评估

`GET /v1/impact` 在差异之外回答“这次变更会影响谁”。两份快照必须属于请求指定的服务和环境，否则返回 `400`（来源不一致）；任一快照不存在返回 `404`。受影响服务通过反向依赖计算：同一环境内，凡是**最新**快照的 `dependencies` 包含被变更服务的作用域都会被列出，并附上声明该依赖的快照 ID 作为证据。被变更服务自身的 `dependencies` 必须在同环境内都有快照记录，否则视为依赖信息不完整，返回 `409` 而不是给出片面结论。
