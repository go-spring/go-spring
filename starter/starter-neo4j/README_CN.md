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
