# starter-gorm-sqlite

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-gorm-sqlite` 提供了基于 gorm 的 sqlite 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 sqlite。SQLite 基于本地文件或内存，
无需部署服务、也不需要 docker 容器，只要一个 DSN 指向本地文件或内存库即可。

## 安装

```bash
go get go-spring.org/starter-gorm-sqlite
```

底层驱动 `gorm.io/driver/sqlite` 依赖 `mattn/go-sqlite3`，属于 cgo 组件，
构建时需要 `CGO_ENABLED=1` 以及可用的 C 编译器（macOS 上是 clang，Linux 上是 gcc）。

## 快速开始

### 1. 引入 `starter-gorm-sqlite` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-gorm-sqlite"
```

### 2. 配置 gorm 实例

在项目的[配置文件](example/conf/app.properties)中添加 gorm 配置，比如：

```properties
spring.gorm.sqlite.primary.dsn=file:primary?mode=memory&cache=shared
```

DSN 可以是 `sqlite3_open` 支持的任意形式：例如 `test.db` 这样的文件路径、
`:memory:`，或者带查询参数的 `file:` URI。

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
err := s.DB.Raw("SELECT sqlite_version()").Scan(&version).Error
```

## 核心功能

[example.go](example/example.go) 演示了 GORM 在 SQLite 上的三项核心能力：

* **AutoMigrate**：通过 `s.DB.AutoMigrate(&KV{})` 由 Go 结构体建表，并使用
  `s.DB.Migrator().HasTable(&KV{})` 校验建表结果。
* **CRUD（Create + First）**：使用 `s.DB.Create(...)` 写入一条记录，再通过
  `s.DB.First(&got, "kkey = ?", "key")` 查询回读。
* **事务（Transaction）**：在 `s.DB.Transaction(func(tx *gorm.DB) error { ... })` 中更新记录，
  提交后再次查询确认字段已变更。

## 高级功能

* **支持多 gorm 实例**：可以在配置文件中定义多个 gorm 实例，并在项目中使用 name 进行引用。
