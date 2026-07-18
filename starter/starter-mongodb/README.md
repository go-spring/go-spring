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

Define one or more named instances under `spring.mongodb.instances.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.mongodb.instances.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.instances.b.uri=mongodb://127.0.0.1:27017
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

* **Multiple MongoDB instances**: Every entry under `spring.mongodb.instances`
  becomes an independently configured `*mongo.Client` bean; inject them by name to
  talk to different clusters or databases.
