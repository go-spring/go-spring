# starter-gorm-sqlite

[English](README.md) | [中文](README_CN.md)

`starter-gorm-sqlite` 是 gorm starter 家族的 SQLite 方言：`spring.gorm.sqlite.instances.
<name>` 下每个条目一个 gorm client，复用 `go-spring.org/starter-gorm` 的
池/观测/韧性/健康脚手架。底层是 [glebarez/sqlite](https://github.com/glebarez/sqlite)
（纯 Go、零 CGO，基于 [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)）。

## 安装

```bash
go get go-spring.org/starter-gorm-sqlite
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-gorm-sqlite"
```

### 2. 配置

```properties
spring.gorm.sqlite.instances.primary.file=:memory:
spring.gorm.sqlite.instances.primary.journal-mode=wal
spring.gorm.sqlite.instances.primary.busy-timeout=5000
spring.gorm.sqlite.instances.primary.max-open-conns=1
```

### 3. 注入

```go
import "go-spring.org/starter-gorm"

type Service struct {
    DB *gormcore.DB `autowire:"sqlite.primary"`
}
```

### 4. 使用

```go
err := s.DB.AutoMigrate(&Model{})
err = s.DB.Create(&Model{Name: "x"}).Error
```

注入的 bean 是共享的 `gormcore.DB`，完整 gorm API 原样提升可用，每实例健康
指示器（`gorm:sqlite:<name>`）自动并入 actuator readiness。

## 配置

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `file` | —（必填） | 数据库路径，或 `:memory:` |
| `journal-mode` | `wal` | wal / delete / truncate / persist / memory / off |
| `busy-timeout` | `5000` | 毫秒（`_busy_timeout` pragma） |
| `foreign-keys` | `true` | 启用 `foreign_keys` pragma |

其余共享 gormcore 的池/观测键（`max-open-conns`、`slow-threshold`、
`observe.enabled` 等）见 starter-gorm README。`:memory:` 每个连接各有一份
独立存储，内存往返场景请锁 `max-open-conns=1`。

## 兼容性

SQLite 是进程内数据库 —— 无 TLS、无服务发现可配置，example 无需 docker。
