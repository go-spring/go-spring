# 组件 - Starter 机制

Starter 是 Go-Spring 推荐的组件模块化方式，方便封装和复用组件。

## Starter 机制

### 注册方式

Starter 基于 Go `init()` 函数完成注册：
- 支持空白导入 `import _ "github.com/go-spring/starter-gorm-mysql"` 自动注册
- 只要包能被 linker 看到，就能完成注册

### 注册形式

**1. provide 形式**

一个 starter 直接注册多个 Bean：
```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(NewDB)
}

func NewDB(config Config) (*gorm.DB, error) {
	// 创建 DB 连接
	db, err := gorm.Open(mysql.Open(config.DSN), &gorm.Config{})
	return db, err
}
```

**2. module 形式**

一个 module 可以根据条件注册多个 Bean：
```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

func Module(props flatten.Storage, reg gs.BeanRegistry) error {
	// 根据配置决定是否启用
	if !props.GetBool("enable-mysql", true) {
		return nil
	}
	reg.Provide(NewDB)
	return nil
}

func init() {
	gs.RegisterModule(Module)
}
```

**3. group 形式**

group 是 module 的特殊形式，适合需要注册多个同类型实例的场景（比如多个 Redis 实例）：
```go
// group 形式适合多实例场景
```

### 设计要点

- **按需实例化**：IoC 容器只创建被依赖的 Bean，即使引入 starter，未被使用也不会创建实例
- **配置驱动**：推荐通过配置（或环境变量）启用/禁用组件，不推荐基于 Bean 依赖是否存在来决定

### 自定义 Starter 开发

按照上述三种形式选择合适的方式注册即可，没有特殊要求。

## 官方提供的 Starter

### 资源型组件

| Starter | 说明 |
|---------|------|
| `starter-gorm-mysql` | MySQL 集成，基于 GORM |
| `starter-go-redis` | Redis 集成，基于 go-redis |
| `starter-redigo` | Redis 集成，基于 redigo |

### Server 型组件

| Starter | 说明 |
|---------|------|
| 内置 HTTP Server | 详见 [内置 HTTP Server](http-server.md) |
| `starter-grpc` | gRPC Server 集成 |
| `starter-thrift` | Thrift Server 集成 |

### 工具型组件

| Starter | 说明 |
|---------|------|
| `starter-pprof` | pprof 性能分析服务 |
