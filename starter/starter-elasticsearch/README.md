# starter-elasticsearch

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-elasticsearch` provides an Elasticsearch client wrapper based on the
official [go-elasticsearch](https://github.com/elastic/go-elasticsearch) client,
making it easy to integrate and use Elasticsearch in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-elasticsearch
```

## Quick Start

### 1. Import the `starter-elasticsearch` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-elasticsearch"
```

### 2. Configure the Elasticsearch Instance

Add Elasticsearch configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.elasticsearch.docs.addresses=http://127.0.0.1:9200
```

### 3. Inject the Elasticsearch Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/elastic/go-elasticsearch/v8"

type Service struct {
    ES *elasticsearch.Client `autowire:"docs"`
}
```

### 4. Use the Elasticsearch Instance

Refer to the [example.go](example/example.go) file.

```go
res, err := s.ES.Index("index", strings.NewReader(`{"title":"hello"}`), s.ES.Index.WithDocumentID("1"))
res, err := s.ES.Get("index", "1")
res, err := s.ES.Search(s.ES.Search.WithIndex("index"), s.ES.Search.WithBody(query))
```

## Core Features

The [example.go](example/example.go) file demonstrates the following core Elasticsearch features:

* **Cluster Info**: verify connectivity to the cluster with `Info`.
* **Index a document**: store a JSON document with `Index`, using `WithRefresh` to make it immediately searchable.
* **Get a document**: retrieve a document by ID with `Get`.
* **Search documents**: query documents with `Search` using a `match` query.

## Advanced Features

* **Supports multiple Elasticsearch instances**: You can define multiple Elasticsearch instances in the configuration
  file and reference them by name in your project.
* **Support Elasticsearch extensions**: You can extend Elasticsearch functionality by implementing the `Driver`
  interface — see the example implementation `AnotherESDriver`.
* **Observability**: the default driver wires the transport into go-spring's
  unified observability via `elastictransport.NewOtelInstrumentation`, emitting
  client spans through the OpenTelemetry global `TracerProvider` that
  `starter-otel` installs. When `starter-otel` is absent that global is a no-op,
  so it stays a zero-config opt-in.
* **Service discovery**: set `service-name` on an instance to resolve its node
  addresses through a registered discovery backend instead of the static
  `addresses` list. Each discovered `host:port` endpoint is turned into a node
  address using `discovery-scheme` (default `http`). Select the backend with
  `discovery` (default `default`); a company registers its naming service once
  via `discovery.Register`.

  ```properties
  spring.elasticsearch.disc.service-name=es-cluster
  spring.elasticsearch.disc.discovery-scheme=http
  ```

  Limitation: this is a **one-shot resolution at startup** — the node list is
  fixed for the client's lifetime. Elasticsearch cluster addresses are typically
  stable VIPs, so this is usually sufficient; when it is not, leave
  `service-name` empty and configure `addresses` directly.

