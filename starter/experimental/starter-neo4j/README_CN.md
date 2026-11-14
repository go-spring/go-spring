# starter-neo4j

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-neo4j` 提供了基于官方 neo4j-go-driver 的 Neo4j 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Neo4j 图数据库。

## 安装

```bash
go get go-spring.org/starter-neo4j
```

## 快速开始

### 1. 引入 `starter-neo4j` 包

参见 [main.go](example/main.go) 文件。

```go
import _ "go-spring.org/starter-neo4j"
```

### 2. 配置 Neo4j 实例

在项目的[配置文件](example/conf/app.properties)中添加 Neo4j 配置，比如：

```properties
spring.neo4j.instances.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.instances.graph.username=neo4j
spring.neo4j.instances.graph.password=password
```

### 3. 注入 Neo4j 实例

参见 [main.go](example/main.go) 文件。

```go
import StarterNeo4j "go-spring.org/starter-neo4j"

type Service struct {
    Neo4j *StarterNeo4j.Client `autowire:"graph"` // 内嵌 neo4j.DriverWithContext
}
```

### 4. 使用 Neo4j 实例

参见 [main.go](example/main.go) 文件。

```go
res, err := neo4j.ExecuteQuery(ctx, s.Neo4j,
    "MATCH (p:Person {name: $name}) RETURN p.age AS age",
    map[string]any{"name": "alice"},
    neo4j.EagerResultTransformer)
```

## 核心功能

[main.go](example/main.go) 文件演示了以下核心 Neo4j 功能：

* **创建节点**：使用 `MERGE ... SET` 创建或更新带属性的节点。
* **查询节点**：使用 `MATCH` 读取节点并检查其属性。
* **关系**：在两个节点之间创建 `KNOWS` 关系并统计数量。

## 高级功能

* **支持多 Neo4j 实例**：可以在配置文件中定义多个 Neo4j 实例，并在项目中使用 name 进行引用。
* **支持 Neo4j 扩展**：可以通过实现 `Driver` 接口来扩展 Neo4j 功能，参见示例中的 `AnotherNeo4jDriver` 实现。
* **服务发现**：在实例上设置 `service-name` 后，将通过已注册的 discovery 后端解析**路由种子地址**，
  而非直接使用 URI 中的 host。种子在构造期拼接进 URI 的 host；随后驱动从它读取 routing table
  并自行选择服务器，同一个 resolver 继续作为驱动的 `AddressResolver` 兜底，用于路由表重建。
  用 `discovery` 选择后端；公司只需贡献一个 `discovery.Discovery` bean（见 `example/discovery.go`）。

  ```properties
  spring.neo4j.instances.disc.uri=neo4j://0.0.0.0:0
  spring.neo4j.instances.disc.username=neo4j
  spring.neo4j.instances.disc.password=password
  spring.neo4j.instances.disc.service-name=neo4j-cluster
  ```

  `service-name` 要求 routing scheme（`neo4j`、`neo4j+s`、`neo4j+ssc`）；与直连 scheme
  （`bolt`、`bolt+s`、`bolt+ssc`）组合会在绑定期被拒绝，因为直连驱动会把种子固化成唯一地址。
  这不是客户端负载均衡：集群（或 Neo4j 4.0+ 单机，同样提供 routing table）的均衡由驱动完成；
  同一名字下互相独立的实例**不会**被驱动均衡——应在其前面放 VIP/L4，或将它们组成集群。

## 可观测

neo4j-go-driver 使用二进制 Bolt 协议，**没有官方的 OpenTelemetry instrumentation**，
也没有类似 SQL/MongoDB 驱动的 command-monitor 钩子，因此没有能拦截每个请求的透明切面。
观测按调用点可选开启，经由 `StarterNeo4j.Query` / `Operation` 两个辅助函数——它们只**声明**
每个操作是什么（`observe.go`）：操作名、`db.system`/`db.operation` 标签，以及作为 `db.statement`
span/日志细节的 Cypher 文本。信号本身由 resilience 层发射——它是 executor 链条上唯一看得见
整次调用（含重试）的点——所以除 call 级 `db.client.operation.duration` 直方图外，每次调用还多一条
attempt 级的 `db.client.attempt.duration` 直方图，另有 `db.client.active_requests` gauge 与
tag 为 `_app_neo4j_access` 的访问日志。未导入 starter-otel 时全部为空操作。直接调
`neo4j.ExecuteQuery` / session 调用会绕过它——这是受上游驱动能力限制的、已记录的取舍，而非遗漏。

