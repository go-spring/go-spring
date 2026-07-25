# starter-gorm-mysql

[English](README.md) | [中文](README_CN.md)

`starter-gorm-mysql` provides a MySQL client wrapper based on gorm for
Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-gorm-mysql
```

## Quick Start

### 1. Import the `starter-gorm-mysql` Package

```go
import _ "go-spring.org/starter-gorm-mysql"
```

### 2. Configure the gorm Instance

Add gorm configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.gorm.mysql.primary.user=root
spring.gorm.mysql.primary.password=123456
spring.gorm.mysql.primary.addr=127.0.0.1:3306
spring.gorm.mysql.primary.db=test
```

### 3. Inject the gorm Instance

```go
import "gorm.io/gorm"

type Service struct {
    DB *gorm.DB `autowire:""`
}
```

### 4. Use the gorm Instance

```go
var version string
err := s.DB.Raw("SELECT VERSION()").Scan(&version).Error
```

## Core Features

The [example.go](example/example.go) file demonstrates three core GORM features against MySQL:

* **AutoMigrate**: create the table from a Go struct via `s.DB.AutoMigrate(&KV{})` and verify with
  `s.DB.Migrator().HasTable(&KV{})`.
* **CRUD (Create + First)**: insert a row with `s.DB.Create(...)` and query it back with
  `s.DB.First(&got, "kkey = ?", "key")`.
* **Transaction**: update the row inside `s.DB.Transaction(func(tx *gorm.DB) error { ... })` and confirm the change
  after commit.

## Advanced Features

* **Supports multiple gorm instances**: you can define multiple gorm instances in the configuration file and reference them by name.
