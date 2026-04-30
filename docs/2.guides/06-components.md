# 组件与 Starter 机制

Starter 是 Go-Spring 推荐的组件模块化方式。
它可以把一组 Bean 的注册逻辑、配置绑定、启用条件和生命周期管理封装在独立包中，
让应用通过一次导入即可获得完整的组件能力。

对于业务应用来说，Starter 的价值在于降低集成成本：
数据库、Redis、HTTP Server、pprof 等基础设施不需要在每个项目里重复编写初始化代码。
对于组件作者来说，Starter 则提供了一套清晰的封装约定，便于发布、复用和维护。

## 核心机制

Starter 通常通过 Go 的 `init()` 函数完成注册。
应用只需要导入 starter 包，包内的注册逻辑就会在程序启动前执行：

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

空白导入适合只触发副作用注册的场景。只要 starter 包能够被 Go linker 看到，
Go-Spring 就可以在应用启动时发现并处理它注册的 Bean、Module 或 Group。

需要注意的是，引入 starter 并不等于立即创建实例。
Go-Spring 的 IoC 容器会**按需实例化** Bean：
只有被依赖、满足条件并进入容器创建流程的对象，才会真正完成初始化。

## 注册形式

Starter 常见的注册形式有三种：`gs.Provide`、`gs.Module` 和 `gs.Group`。
它们分别面向简单单实例、动态注册逻辑和多实例配置场景。

### Provide：注册单个 Bean

`gs.Provide` 用于直接注册一个 Bean，适合组件提供单个实例的场景。
通过链式调用可以同时声明配置注入、启用条件、Bean 名称和销毁函数。

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	// 基础用法
	gs.Provide(NewDB)

	// 完整用法：配置注入 + 条件启用 + 命名 + 生命周期管理
	gs.Provide(NewDB, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")). // 属性存在时才创建
		Name("__default__").                         // Bean 名称
		Destroy(CloseDB)                             // 销毁时清理资源
}

type Config struct {
	DSN string `value:"${dsn}"`
}

func NewDB(config Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(config.DSN), &gorm.Config{})
}

func CloseDB(db *gorm.DB) error {
	sqlDB, _ := db.DB()
	return sqlDB.Close()
}
```

在上面的示例中，`gs.TagArg("${spring.gorm}")` 表示将 `spring.gorm` 前缀下的配置绑定到构造函数参数；
`gs.OnProperty("spring.gorm.dsn")` 表示只有配置了该属性时才启用这个 Bean。

### Module：按条件动态注册

`gs.Module` 适合注册逻辑需要根据配置或环境动态展开的场景。
它可以在注册阶段读取配置，并据此决定注册哪些 Bean。

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

func init() {
	// 注册 Module，支持前置 Condition
	gs.Module(
		// 仅当 enable-mysql 为 true 时才注册
		gs.OnProperty("enable-mysql").HavingValue("true"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			// 根据配置动态决定注册逻辑
			if s, _ := p.Value("enable-readonly"); s == "true" {
				r.Provide(NewReadOnlyDB)
			} else {
				r.Provide(NewDB)
			}
			return nil
		})
}
```

当一个 starter 需要根据配置切换实现、注册一组相关 Bean，或者执行较复杂的条件判断时，
`gs.Module` 比单纯的 `gs.Provide` 更合适。

### Group：注册多个同类型实例

`gs.Group` 是面向多实例配置的注册形式，常用于多数据库、多 Redis、多客户端等场景。
它会遍历配置字典，并为每一个配置创建独立的 Bean。

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 根据 spring.gorm.instances 配置字典创建多个 DB 实例
	// 每个实例使用其中一项配置
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

这类配置通常写成字典结构：key 作为 Bean 名称，value 作为对应实例的配置。

```yaml
spring:
  gorm:
    instances:
      db1:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
      db2:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
```

通过 `gs.Group` 注册后，Starter 不需要手动解析数组，也不需要为每个实例单独编写注册代码。

## 自定义 Starter

官方 Starter 通常采用“默认单实例 + 可选多实例”的注册模式。
自定义 Starter 也建议遵循这一约定，这样应用侧的配置和使用方式会更加统一。

```go
func init() {
	// 注册默认单实例。
	// 只有配置了 spring.gorm.dsn 时，这个实例才会被创建。
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	// 注册多实例。
	// 每个实例都会根据 spring.gorm.instances 中的配置创建。
	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

建议遵循以下命名与配置规范：

- 配置前缀使用 `spring.xxx` 或 `spring.xxx.yyy` 格式，并与组件名称保持一致。
- 默认单实例使用 `__default__` 作为 Bean 名称。
- 默认单实例建议通过关键配置项触发，例如 `spring.xxx.addr`。
- 多实例配置建议统一放在 `spring.xxx.instances` 配置字典下。
- 资源型组件应提供 `Destroy` 函数，确保应用停止时可以优雅释放连接、文件句柄或后台任务。

## 官方 Starter

Go-Spring 开箱即用提供了一些常见基础组件的 Starter，可直接用于应用开发。

| Starter | 说明 |
|---------|------|
| `starter-gorm-mysql` | MySQL 集成，基于 GORM |
| `starter-go-redis` | Redis 集成，基于 go-redis |
| `starter-redigo` | Redis 集成，基于 redigo |
| 内置 HTTP Server | 详见 [内置 HTTP Server](05-http-server.md) |
| `starter-pprof` | pprof 性能分析服务 |
