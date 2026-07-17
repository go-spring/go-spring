# starter-gorm-clickhouse

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-gorm-clickhouse` provides a clickhouse client wrapper based on gorm,
making it easy to integrate and use clickhouse in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-gorm-clickhouse
```

## Quick Start

### 1. Import the `starter-gorm-clickhouse` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-gorm-clickhouse"
```

### 2. Configure the gorm Instance

Add gorm configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.gorm.clickhouse.primary.user=default
spring.gorm.clickhouse.primary.password=
spring.gorm.clickhouse.primary.addr=127.0.0.1:9000
spring.gorm.clickhouse.primary.db=default
```

### 3. Inject the gorm Instance

Refer to the [example.go](example/example.go) file.

```go
import "gorm.io/gorm"

type Service struct {
    DB *gorm.DB `autowire:""`
}
```

### 4. Use the gorm Instance

Refer to the [example.go](example/example.go) file.

```go
var version string
err := s.DB.Raw("SELECT version()").Scan(&version).Error
```

## Core Features

The [example.go](example/example.go) file demonstrates GORM against ClickHouse.
ClickHouse is an OLAP engine — it does not enforce unique indexes the way OLTP
engines do, and it has no standard multi-statement transactions — so this
example is adapted accordingly:

* **AutoMigrate**: create the table from a Go struct via `s.DB.AutoMigrate(&KV{})`
  with a MergeTree engine set through `db.Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)")`,
  and verify with `s.DB.Migrator().HasTable(&KV{})`.
* **CRUD (Create + First + batch insert + Count)**: insert a row with
  `s.DB.Create(...)`, read it back with `s.DB.First(&got, "kkey = ?", "key")`,
  then batch-insert more rows and verify the total via `s.DB.Model(&KV{}).Count(&count)`.
  Transactions are intentionally not demonstrated: ClickHouse has no standard
  multi-statement transactions.

## Advanced Features

* **Supports multiple gorm instances**: You can define multiple gorm instances in the configuration file and reference
  them by name in your project.
