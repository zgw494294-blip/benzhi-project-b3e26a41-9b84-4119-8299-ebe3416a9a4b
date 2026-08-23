# 实验室废弃物合规交接台

实验室废弃物合规交接台面向实验室安全员和废弃物接收复核员，使用一个有状态的交接批次约束废弃物登记、容器封装、标签核验、相容性审查、问题整改、现场确认和归档凭据签发。服务提供原生响应式浏览器工作台和同源 JSON API；本地 SQLite 保存批次聚合、条目、审查历史、幂等结果及不可变归档凭据。

## 构建

在项目根目录执行：

```text
go build ./cmd/server
```

## 运行

默认只监听回环地址 `127.0.0.1:19081`，数据文件为 `handover.db`：

```text
go run ./cmd/server
```

也可以显式指定回环端口和数据路径：

```text
go run ./cmd/server -addr=127.0.0.1:19400 -data=./var/handover.db
```

运行环境提供 `PORT` 时，服务会绑定 `127.0.0.1:<PORT>`；`PORT` 仅接受 1 到 65535 之间的端口号。

浏览器访问 `http://127.0.0.1:19081/`。页面中的“当前身份”用于演示安全员与接收复核员的角色边界，写请求同时携带 `requestId` 和 `expectedVersion`，重复提交会重放原结果，过期版本会返回冲突。

## 测试

完整回归测试：

```text
go test ./...
```

有界自检会创建临时 SQLite 数据库，实际走通退回整改、重新审查、冻结、双方确认和凭据核验，然后自行退出：

```text
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
```

## API 概览

`GET /api/batches` 查询批次，`POST /api/batches` 创建批次；列表支持 `dueStatus=normal|due_soon|due_today|overdue` 到期筛选。批次详情、时间线和凭据分别使用 `GET /api/batches/{batchID}`、`GET /api/batches/{batchID}/timeline` 和 `GET /api/batches/{batchID}/receipt`，归档凭据核验与下载使用 `/receipt/verification` 和 `/receipt/download`。条目录入、修改、封装、批量封装、改期、审查、批量裁决、整改、冻结和确认端点都在 `/api/batches/{batchID}` 下，并使用 JSON 请求体；所有写请求继续携带 `requestId` 与 `expectedVersion`。批次详情投影包含相容性预检、封装缺项清单和审查原因代码。服务同时提供 `GET /api/health` 健康检查。
