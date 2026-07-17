# starter-gorm-postgres

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-gorm-postgres` provides a postgres client wrapper based on gorm,
making it easy to integrate and use postgres in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-gorm-postgres
```

## Quick Start

### 1. Import the `starter-gorm-postgres` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-gorm-postgres"
```

### 2. Configure the gorm Instance

Add gorm configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.gorm.postgres.primary.host=127.0.0.1
spring.gorm.postgres.primary.port=5432
spring.gorm.postgres.primary.user=postgres
spring.gorm.postgres.primary.password=123456
spring.gorm.postgres.primary.db=test
spring.gorm.postgres.primary.sslmode=disable
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

The [example.go](example/example.go) file demonstrates three core GORM features against PostgreSQL:

* **AutoMigrate**: create the table from a Go struct via `s.DB.AutoMigrate(&KV{})` and verify with
  `s.DB.Migrator().HasTable(&KV{})`.
* **CRUD (Create + First)**: insert a row with `s.DB.Create(...)` and query it back with
  `s.DB.First(&got, "kkey = ?", "key")`.
* **Transaction**: update the row inside `s.DB.Transaction(func(tx *gorm.DB) error { ... })` and confirm the change
  after commit.

## Advanced Features

* **Supports multiple gorm instances**: You can define multiple gorm instances in the configuration file and reference
  them by name in your project.
