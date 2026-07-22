# starter-mongodb

[English](README.md) | [中文](README_CN.md)

`starter-mongodb` provides a MongoDB client wrapper based on go.mongodb.org/mongo-driver/v2,
making it easy to integrate and use MongoDB in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-mongodb
```

## Quick Start

### 1. Import the `starter-mongodb` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-mongodb"
```

### 2. Configure the MongoDB Instances

Define one or more named instances under `spring.mongodb.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.mongodb.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.b.uri=mongodb://127.0.0.1:27017
```

### 3. Inject the MongoDB Instance

Refer to the [example.go](example/example.go) file. Each named instance is registered
as a `*mongo.Client` bean under that name; inject the one you need by name.

```go
import "go.mongodb.org/mongo-driver/v2/mongo"

type Service struct {
    Mongo *mongo.Client `autowire:"a"`
}
```

### 4. Use the MongoDB Instance

Refer to the [example.go](example/example.go) file.

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
  becomes an independently configured `*mongo.Client` bean; inject them by name to
  talk to different clusters or databases.

* **Observability**: each client is bridged into go-spring's unified
  observability through a command monitor that emits one client span per MongoDB
  command via the OpenTelemetry global `TracerProvider` that `starter-otel`
  installs. When `starter-otel` is absent that global is a no-op, so spans cost
  nothing and no per-app wiring is needed. (The bridge is implemented directly
  against the v2 driver's event API because the official `otelmongo`
  instrumentation targets the v1 driver and is type-incompatible with the v2
  driver used here.)

* **Service discovery**: set `service-name` on an instance to resolve its address
  through a registered discovery backend instead of the URI hosts. A
  `LiveDialer` is injected as the client's dialer, so each new connection reaches
  a currently-live instance and address changes take effect without rebuilding
  the client. Select the backend with `discovery` (default `default`); a company
  registers its naming service once via `discovery.Register`.

  ```properties
  spring.mongodb.disc.uri=mongodb://0.0.0.0:0/?directConnection=true
  spring.mongodb.disc.service-name=mongo-cluster
  ```

  Note: this bypasses MongoDB's own replica-set / mongos topology discovery — the
  driver dials whatever the naming service hands out. Use it when the intent is
  "reach the service via the company naming service"; leave `service-name` empty
  to let the driver manage topology from the URI.

