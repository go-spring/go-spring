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

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-neo4j"
```

### 2. 配置 Neo4j 实例

在项目的[配置文件](example/conf/app.properties)中添加 Neo4j 配置，比如：

```properties
spring.neo4j.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.graph.username=neo4j
spring.neo4j.graph.password=password
```

### 3. 注入 Neo4j 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/neo4j/neo4j-go-driver/v5/neo4j"

type Service struct {
    Neo4j neo4j.DriverWithContext `autowire:"graph"`
}
```

### 4. 使用 Neo4j 实例

参见 [example.go](example/example.go) 文件。

```go
res, err := neo4j.ExecuteQuery(ctx, s.Neo4j,
    "MATCH (p:Person {name: $name}) RETURN p.age AS age",
    map[string]any{"name": "alice"},
    neo4j.EagerResultTransformer)
```

## 核心功能

[example.go](example/example.go) 文件演示了以下核心 Neo4j 功能：

* **创建节点**：使用 `MERGE ... SET` 创建或更新带属性的节点。
* **查询节点**：使用 `MATCH` 读取节点并检查其属性。
* **关系**：在两个节点之间创建 `KNOWS` 关系并统计数量。

## 高级功能

* **支持多 Neo4j 实例**：可以在配置文件中定义多个 Neo4j 实例，并在项目中使用 name 进行引用。
* **支持 Neo4j 扩展**：可以通过实现 `Driver` 接口来扩展 Neo4j 功能，参见示例中的 `AnotherNeo4jDriver` 实现。
* **服务发现**：在实例上设置 `service-name` 后，地址将通过已注册的 discovery 后端解析，
  而非直接使用 URI 中的 host。端点在启动时解析一次，并拼接进 URI 的 host。
  用 `discovery` 选择后端（默认 `default`）；公司通过 `discovery.Register` 注册一次自己的命名服务即可。

  ```properties
  spring.neo4j.disc.uri=bolt://0.0.0.0:0
  spring.neo4j.disc.username=neo4j
  spring.neo4j.disc.password=password
  spring.neo4j.disc.service-name=neo4j-cluster
  ```

  局限：与支持自定义 dialer 的客户端不同，neo4j 驱动基于 URI 构建连接池，且未暴露 dialer 注入点，
  因此这是**启动时的一次性解析**——启动之后的地址变更需重建客户端才会生效。

## 可观测

neo4j-go-driver 使用二进制 Bolt 协议，**没有官方的 OpenTelemetry instrumentation**，
也没有类似 SQL/MongoDB 驱动的 command-monitor 钩子。因此 starter 没有干净的切面来输出
client span，与 `starter-gorm-*`、`starter-go-redis`、`starter-mongodb` 不同，
**starter 内不接入 tracing**。需要 span 的应用应直接在自己的 `ExecuteQuery` / session
调用外层包裹 OpenTelemetry span。这是受上游驱动能力限制的、已记录的取舍，而非遗漏。

