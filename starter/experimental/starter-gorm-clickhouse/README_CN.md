# starter-gorm-clickhouse

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-gorm-clickhouse` 提供了基于 gorm 的 clickhouse 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 clickhouse。

## 安装

```bash
go get go-spring.org/starter-gorm-clickhouse
```

## 快速开始

### 1. 引入 `starter-gorm-clickhouse` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-gorm-clickhouse"
```

### 2. 配置 gorm 实例

在项目的[配置文件](example/conf/app.properties)中添加 gorm 配置，比如：

```properties
spring.gorm.clickhouse.primary.user=default
spring.gorm.clickhouse.primary.password=
spring.gorm.clickhouse.primary.addr=127.0.0.1:9000
spring.gorm.clickhouse.primary.db=default
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
err := s.DB.Raw("SELECT version()").Scan(&version).Error
```

## 核心功能

[example.go](example/example.go) 演示了 GORM 在 ClickHouse 上的能力。
ClickHouse 是 OLAP 引擎，既不像 OLTP 引擎那样强制唯一索引，也没有标准的多语句
事务，因此示例做了相应适配：

* **AutoMigrate**：通过 `s.DB.AutoMigrate(&KV{})` 由 Go 结构体建表，并使用
  `db.Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)")` 指定
  MergeTree 引擎，再用 `s.DB.Migrator().HasTable(&KV{})` 校验建表结果。
* **CRUD（Create + First + 批量插入 + Count）**：使用 `s.DB.Create(...)` 写入
  一条记录，再通过 `s.DB.First(&got, "kkey = ?", "key")` 查询回读，随后批量
  插入多条记录并用 `s.DB.Model(&KV{}).Count(&count)` 校验总数。
  ClickHouse 无标准多语句事务，因此示例刻意不演示 `Transaction`。

## 高级功能

* **支持多 gorm 实例**：可以在配置文件中定义多个 gorm 实例，并在项目中使用 name 进行引用。
