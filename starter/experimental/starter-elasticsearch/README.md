# starter-elasticsearch

[English](README.md) | [中文](README_CN.md)

`starter-elasticsearch` provides an Elasticsearch client wrapper based on the
official [go-elasticsearch](https://github.com/elastic/go-elasticsearch) client.
Use it to integrate Elasticsearch in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-elasticsearch
```

## Quick Start

### 1. Import the `starter-elasticsearch` Package

Refer to the [main.go](example/main.go) file.

```go
import _ "go-spring.org/starter-elasticsearch"
```

### 2. Configure the Elasticsearch Instance

Add Elasticsearch configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.elasticsearch.instances.docs.addresses=http://127.0.0.1:9200
```

### 3. Inject the Elasticsearch Instance

Refer to the [main.go](example/main.go) file.

```go
import StarterElasticsearch "go-spring.org/starter-elasticsearch"

type Service struct {
    ES *StarterElasticsearch.Client `autowire:"docs"` // the API tree + lifecycle, not the raw client
}
```

### 4. Use the Elasticsearch Instance

Refer to the [main.go](example/main.go) file.

```go
res, err := s.ES.Index("index", strings.NewReader(`{"title":"hello"}`), s.ES.Index.WithDocumentID("1"))
res, err := s.ES.Get("index", "1")
res, err := s.ES.Search(s.ES.Search.WithIndex("index"), s.ES.Search.WithBody(query))
```

## Core Features

The [main.go](example/main.go) file demonstrates the following core Elasticsearch features:

* **Cluster Info**: verify connectivity to the cluster with `Info`.
* **Index a document**: store a JSON document with `Index`, using `WithRefresh` to make it immediately searchable.
* **Get a document**: retrieve a document by ID with `Get`.
* **Search documents**: query documents with `Search` using a `match` query.

## Advanced Features

* **Supports multiple Elasticsearch instances**: You can define multiple Elasticsearch instances in the configuration
  file and reference them by name in your project.
* **Support Elasticsearch extensions**: You can extend Elasticsearch functionality by implementing the `Driver`
  interface — see the example implementation `AnotherESDriver`.
* **Observability**: the starter only DECLARES what each request is — the
  `db.system`/`db.operation` vocabulary plus the URL path, put on the request
  context by the transport stack. The resilience layer EMITS every signal from
  that declaration: the one client span, the call-level `db.client.operation.duration`
  and attempt-level `db.client.attempt.duration` histograms, the in-flight
  `db.client.active_requests` gauge, and the single `_app_elasticsearch_access`
  access log. All ride the OpenTelemetry globals that `starter-otel` installs;
  when `starter-otel` is absent those globals are no-ops, so it stays a
  zero-config opt-in.
* **Service discovery**: set `service-name` on an instance to resolve its node
  addresses through a registered discovery backend instead of the static
  `addresses` list. Each discovered `host:port` endpoint is turned into a node
  address using `discovery-scheme` (default `http`). Select the backend with
  `discovery` (required — there is no default backend); a company registers its naming service once
  via `discovery.Register`.

  ```properties
  spring.elasticsearch.instances.disc.service-name=es-cluster
  spring.elasticsearch.instances.disc.discovery-scheme=http
  ```

  Limitation: this is a **one-shot resolution at startup** — the node list is
  fixed for the client's lifetime. Elasticsearch cluster addresses are typically
  stable VIPs, so this is usually sufficient; when it is not, leave
  `service-name` empty and configure `addresses` directly.

## Design Notes

* **Fail at boot, not on first use.** A one-shot `Info` probe runs during
  construction, so a bad address, certificate or credential surfaces at startup;
  the health indicator and that probe share the same `HealthCheck`.
* **Exactly one provisioning mode.** The node list comes from `addresses`,
  `cloud-id` or `service-name` — exactly one; leaving all three empty fails
  startup, since the probe has nothing to reach.
* **The v8 client has no `Close`.** Its transport reuses idle `net/http`
  connections, so the destroy callback is deliberately a no-op — lifecycle
  symmetry with the other client starters, not an oversight.
* **A custom `Driver` is process-wide.** The Driver is a single container bean
  serving every instance, so a custom driver should delegate to the bundled
  `DefaultDriver` to keep per-instance behaviour. Transport wrapping (e.g. an
  APM/OTel transport) belongs in the driver's `CreateClient`; the base transport
  stays plain `net/http`, so an app that never imports `starter-otel` pays nothing.

