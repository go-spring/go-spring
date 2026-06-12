# starter-gorm-mysql

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-gorm-mysql` provides a mysql client wrapper based on gorm,
making it easy to integrate and use mysql in Go-Spring applications.

## Installation

```bash
go get github.com/go-spring/starter-gorm-mysql
```

## Quick Start

### 1. Import the `starter-gorm-mysql` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

### 2. Configure the gorm Instance

Add gorm configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.gorm.main.url=xxx
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
err := s.DB.Raw("SELECT VERSION()").Scan(&version).Error
```

## Advanced Features

* **Supports multiple gorm instances**: You can define multiple gorm instances in the configuration file and reference
  them by name in your project.
