# starter-mongodb

[English](README.md) | [中文](README_CN.md)

`starter-mongodb` provides a MongoDB client wrapper based on
go.mongodb.org/mongo-driver/v2 for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-mongodb
```

## Quick Start

### 1. Import the `starter-mongodb` Package

```go
import _ "go-spring.org/starter-mongodb"
```

### 2. Configure the MongoDB Instances

Define one or more named instances under `spring.mongodb.instances.<name>` in your
project's [configuration file](example/conf/app.properties):

```properties
spring.mongodb.instances.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.instances.b.uri=mongodb://127.0.0.1:27017
```

### 3. Inject the MongoDB Instance

Each named instance is registered as a `*StarterMongoDB.Client` bean under that name (it embeds `*mongo.Client`, so the whole driver method set is promoted onto it); inject the one you need by name.

```go
import StarterMongoDB "go-spring.org/starter-mongodb"

type Service struct {
    Mongo *StarterMongoDB.Client `autowire:"a"` // embeds *mongo.Client
}
```

### 4. Use the MongoDB Instance

```go
coll := s.Mongo.Database("test").Collection("kv")
_, err := coll.InsertOne(ctx, bson.M{"key": "key", "value": "value"})
err = coll.FindOne(ctx, bson.M{"key": "key"}).Decode(&res)
```

## Core Features

The [example.go](example/example.go) exercises three core MongoDB operations end-to-end:

* **InsertOne** — insert a document and verify `InsertedID` is returned.
* **FindOne** — read the document back and assert the field value.
* **UpdateOne** — `$set` a field, assert `ModifiedCount == 1`, then re-read to confirm the new value.

## Advanced Features

* **Multiple MongoDB instances**: Every entry under `spring.mongodb`
  becomes an independently configured `*StarterMongoDB.Client` bean; inject them by name to
  talk to different clusters or databases.

* **Observability**: each client carries module-local instrumentation wired
  through a command monitor that emits one span per MongoDB command,
  `db.client.*` metrics, and an `_app_mongodb_access` access log via the
  OpenTelemetry globals that `starter-otel` installs. When `starter-otel` is
  absent those globals are no-ops, so signals cost nothing and no per-app
  wiring is needed. (The bridge is implemented directly against the v2
  driver's event API because the official `otelmongo` instrumentation targets
  the v1 driver and is type-incompatible with the v2 driver used here.)

  This is the one client starter that emits locally rather than declaring an
  operation for the framework's single resilience emitter to apply, and that is
  by driver constraint: the mongo driver v2 has no per-command hook (only a
  dialer and observer-only monitors), so observation lives at the command layer
  while protection lives at the dial layer through the resilience executor.
  The emitted vocabulary is deliberately the same as the unified emitter's —
  bounded `db.system` + `db.operation` labels, `db.statement` as unbounded
  detail on the span and log only, an internal span named after the command,
  and the same `status` / `duration_ms` access-log fields — except that the
  status words are `success` / `error`, since a command monitor cannot see a
  resilience rejection.

* **Service discovery**: set `service-name` on an instance to resolve its address
  through a registered discovery backend instead of the URI hosts. A
  Resolver-backed dialer is injected as the client's dialer, so each new
  connection reaches a currently-live instance and address changes take effect
  without rebuilding the client. Select the backend with `discovery` (required —
  there is no default backend); a company registers its naming service once via
  `discovery.Register`. In mesh mode (`mesh.Enabled()`) the sidecar owns
  discovery+LB, so `service-name` is skipped and the URI hosts are dialed
  directly.

  ```properties
  spring.mongodb.instances.disc.uri=mongodb://0.0.0.0:0/?directConnection=true
  spring.mongodb.instances.disc.service-name=mongo-cluster
  ```

  Note: this bypasses MongoDB's own replica-set / mongos topology discovery — the
  driver dials whatever the naming service hands out. Use it when the intent is
  "reach the service via the company naming service"; leave `service-name` empty
  to let the driver manage topology from the URI.

