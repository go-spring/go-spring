# starter-neo4j

[English](README.md) | [中文](README_CN.md)

`starter-neo4j` provides a Neo4j client wrapper based on the official
neo4j-go-driver for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-neo4j
```

## Quick Start

### 1. Import the `starter-neo4j` Package

```go
import _ "go-spring.org/starter-neo4j"
```

### 2. Configure the Neo4j Instance

Add Neo4j configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.neo4j.instances.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.instances.graph.username=neo4j
spring.neo4j.instances.graph.password=password
```

### 3. Inject the Neo4j Instance

```go
import StarterNeo4j "go-spring.org/starter-neo4j"

type Service struct {
    Neo4j *StarterNeo4j.Client `autowire:"graph"` // embeds neo4j.DriverWithContext
}
```

### 4. Use the Neo4j Instance

```go
res, err := neo4j.ExecuteQuery(ctx, s.Neo4j,
    "MATCH (p:Person {name: $name}) RETURN p.age AS age",
    map[string]any{"name": "alice"},
    neo4j.EagerResultTransformer)
```

## Core Features

The [main.go](example/main.go) file demonstrates the following core Neo4j features:

* **Create nodes**: create or update a node with properties using `MERGE ... SET`.
* **Query nodes**: read a node back with `MATCH` and inspect its properties.
* **Relationships**: create a `KNOWS` relationship between two nodes and count it.

## Advanced Features

* **Supports multiple Neo4j instances**: you can define multiple Neo4j instances in the configuration file and reference them by name.
* **Support Neo4j extensions**: implement the `Driver` interface to extend Neo4j functionality — see the
  example implementation `AnotherNeo4jDriver`.
* **Service discovery**: set `service-name` on an instance to resolve its
  **routing seed** through a registered discovery backend instead of the URI
  host. The seed is spliced into the URI host at construction; the driver then
  reads the routing table from it and owns server selection, while the same
  resolver stays behind the driver's `AddressResolver` fallback for
  router refresh. Select the backend with `discovery`; a company contributes a
  `discovery.Discovery` bean (see `example/discovery.go`).

  ```properties
  spring.neo4j.instances.disc.uri=neo4j://0.0.0.0:0
  spring.neo4j.instances.disc.username=neo4j
  spring.neo4j.instances.disc.password=password
  spring.neo4j.instances.disc.service-name=neo4j-cluster
  ```

  `service-name` requires a routing scheme (`neo4j`, `neo4j+s`, `neo4j+ssc`) and
  is rejected at bind time with a direct scheme (`bolt`, `bolt+s`, `bolt+ssc`),
  where the driver would pin the seed forever. This is not client-side load
  balancing: for a cluster (or a single Neo4j 4.0+ instance, which also exposes
  a routing table) the driver balances. Independent instances published under
  one name are **not** balanced by the driver — put a VIP/L4 in front of them or
  join them into a cluster.

## Observability

The neo4j-go-driver speaks the binary Bolt protocol and ships **no official
OpenTelemetry instrumentation**, nor a command-monitor hook comparable to the
SQL/MongoDB drivers — there is no transparent seam to intercept every request.
Observation is therefore opt-in per call site via the `StarterNeo4j.Query` /
`Operation` helpers, which **declare** what each operation is (`observe.go`):
the operation name, the `db.system`/`db.operation` labels, and the Cypher text
as `db.statement` span/log detail. The signals themselves are emitted by the
resilience layer — the one point on the executor chain that sees a whole call,
retries included — so beside the call-level `db.client.operation.duration`
histogram every call also reports an attempt-level `db.client.attempt.duration`
one, plus the `db.client.active_requests` gauge and an `_app_neo4j_access` access
log. All of it is a no-op unless `starter-otel` installs providers. Direct
`neo4j.ExecuteQuery` / session calls bypass it — a documented gap driven by
upstream driver support, not an oversight.

