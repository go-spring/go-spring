# starter-gorm-mysql

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-gorm-mysql` 提供了基于 gorm 的 mysql 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 mysql。

## 安装

```bash
go get go-spring.org/starter-gorm-mysql
```

## 快速开始

### 1. 引入 `starter-gorm-mysql` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-gorm-mysql"
```

### 2. 配置 gorm 实例

在项目的[配置文件](example/conf/app.properties)中添加 gorm 配置，比如：

```properties
spring.gorm.mysql.primary.user=root
spring.gorm.mysql.primary.password=123456
spring.gorm.mysql.primary.addr=127.0.0.1:3306
spring.gorm.mysql.primary.db=test
```

### 3. 注入 gorm 实例

参见 [example.go](example/example.go) 文件。

```go
import "gorm.io/gorm"

type Service struct {
    DB *gorm.DB `autowire:""`
}
```

### 4. 使用 gorm 实例

参见 [example.go](example/example.go) 文件。

```go
var version string
err := s.DB.Raw("SELECT VERSION()").Scan(&version).Error
```

## 核心功能

[example.go](example/example.go) 演示了 GORM 在 MySQL 上的三项核心能力：

* **AutoMigrate**：通过 `s.DB.AutoMigrate(&KV{})` 由 Go 结构体建表，并使用
  `s.DB.Migrator().HasTable(&KV{})` 校验建表结果。
* **CRUD（Create + First）**：使用 `s.DB.Create(...)` 写入一条记录，再通过
  `s.DB.First(&got, "kkey = ?", "key")` 查询回读。
* **事务（Transaction）**：在 `s.DB.Transaction(func(tx *gorm.DB) error { ... })` 中更新记录，
  提交后再次查询确认字段已变更。

## 高级功能

* **支持多 gorm 实例**：可以在配置文件中定义多个 gorm 实例，并在项目中使用 name 进行引用。
