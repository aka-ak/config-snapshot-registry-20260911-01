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
- `GET /v1/audit` 查询快照写入和清理事件

差异路径使用点号表示嵌套对象，例如 `database.pool.max`。数组被当作整体值比较，输出顺序按路径稳定排序，方便审计和重放。
