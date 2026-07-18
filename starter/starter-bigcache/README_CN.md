# starter-bigcache

[English](README.md) | [中文](README_CN.md)

`starter-bigcache` 基于 [BigCache](https://github.com/allegro/bigcache) 提供了进程内缓存封装，
让你在 Go-Spring 应用中轻松集成并使用高性能、对 GC 友好的内存缓存。

## 安装

```bash
go get go-spring.org/starter-bigcache
```

## 快速开始

### 1. 引入 `starter-bigcache` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-bigcache"
```

### 2. 配置 BigCache 实例

在项目的[配置文件](example/conf/app.properties)中添加 BigCache 配置，例如：

```properties
spring.bigcache.main.life-window=10m
```

### 3. 注入 BigCache 实例

参考 [example.go](example/example.go) 文件。

```go
import "github.com/allegro/bigcache/v3"

type Service struct {
    Cache *bigcache.BigCache `autowire:"main"`
}
```

### 4. 使用 BigCache 实例

参考 [example.go](example/example.go) 文件。

```go
err := s.Cache.Set("key", []byte("value"))
value, err := s.Cache.Get("key")
```

## 核心特性

[example.go](example/example.go) 程序演示并断言了三个核心 BigCache 操作：

* **SET/GET** —— 用 `Set(...)` 写入，再用 `Get(...)` 读回。
* **DELETE + 未命中** —— 用 `Delete(...)` 删除键，确认随后的 `Get(...)` 返回 `ErrEntryNotFound`。
* **实例隔离** —— 写入某个命名实例的键在另一个实例中不可见，证明多实例接线正确。

## 高级特性

* **支持多个 BigCache 实例**：你可以在配置文件中定义多个 BigCache 实例，并在项目中按名称引用它们。
* **支持 BigCache 扩展**：你可以通过实现 `Driver` 接口来扩展 BigCache 的创建逻辑。
