# starter-neo4j

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-neo4j` provides a Neo4j client wrapper based on the official
neo4j-go-driver, making it easy to integrate and use the Neo4j graph database in
Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-neo4j
```

## Quick Start

### 1. Import the `starter-neo4j` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-neo4j"
```

### 2. Configure the Neo4j Instance

Add Neo4j configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.neo4j.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.graph.username=neo4j
spring.neo4j.graph.password=password
```

### 3. Inject the Neo4j Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/neo4j/neo4j-go-driver/v5/neo4j"

type Service struct {
    Neo4j neo4j.DriverWithContext `autowire:"graph"`
}
```

### 4. Use the Neo4j Instance

Refer to the [example.go](example/example.go) file.

```go
res, err := neo4j.ExecuteQuery(ctx, s.Neo4j,
    "MATCH (p:Person {name: $name}) RETURN p.age AS age",
    map[string]any{"name": "alice"},
    neo4j.EagerResultTransformer)
```

## Core Features

The [example.go](example/example.go) file demonstrates the following core Neo4j features:

* **Create nodes**: create or update a node with properties using `MERGE ... SET`.
* **Query nodes**: read a node back with `MATCH` and inspect its properties.
* **Relationships**: create a `KNOWS` relationship between two nodes and count it.

## Advanced Features

* **Supports multiple Neo4j instances**: You can define multiple Neo4j instances in the configuration file and reference
  them by name in your project.
* **Support Neo4j extensions**: You can extend Neo4j functionality by implementing the `Driver` interface — see the
  example implementation `AnotherNeo4jDriver`.
