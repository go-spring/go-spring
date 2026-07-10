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

### 2. Configure the MongoDB Instance

Add MongoDB configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.mongodb.uri=mongodb://127.0.0.1:27017
```

### 3. Inject the MongoDB Instance

Refer to the [example.go](example/example.go) file.

```go
import "go.mongodb.org/mongo-driver/v2/mongo"

type Service struct {
    Mongo *mongo.Client `autowire:"__default__"`
}
```

### 4. Use the MongoDB Instance

Refer to the [example.go](example/example.go) file.

```go
coll := s.Mongo.Database("test").Collection("kv")
_, err := coll.InsertOne(ctx, bson.M{"key": "key", "value": "value"})
err = coll.FindOne(ctx, bson.M{"key": "key"}).Decode(&res)
```

## Advanced Features

* **Supports multiple MongoDB instances**: You can define multiple MongoDB instances under
  `spring.mongodb.instances` in the configuration file and reference them by name.
