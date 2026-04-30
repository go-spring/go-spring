# 组件与 Starter 机制

Starter 是 Go-Spring 推荐的组件模块化方式。它可以把一组 Bean 注册、配置绑定、启用条件和生命周期管理封装在独立包中，让应用通过一次导入即可获得完整的组件能力。

对于业务应用来说，Starter 的价值在于降低集成成本：数据库、Redis、HTTP Server、pprof 等基础设施不需要在每个项目里重复编写初始化代码。对于组件作者来说，Starter 则提供了一套清晰的封装约定，便于发布、复用和维护。

## 核心机制

Starter 通常通过 Go 的 `init()` 函数完成注册。应用只要导入 starter 包，包内的注册逻辑就会在程序启动前执行：

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

空白导入适合只触发副作用注册的场景。只要 starter 包能够被 linker 看到，Go-Spring 就可以在应用启动时发现并处理它注册的 Bean、Module 或 Group。

需要注意的是，引入 starter 并不等于立即创建实例。Go-Spring 的 IoC 容器会按需实例化 Bean：只有被依赖、满足条件并进入容器创建流程的对象，才会真正完成初始化。

## 注册形式

Starter 常用的注册形式有三种：`Provide`、`Module` 和 `Group`。它们分别面向简单单实例、动态注册逻辑和多实例配置场景。

### Provide：注册单个 Bean

`Provide` 用于直接注册一个 Bean，适合组件只有一个主要实例的场景。通过链式调用可以同时声明配置注入、启用条件、Bean 名称和销毁函数。

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
		Condition(gs.OnProperty("spring.gorm.addr")).  // 属性存在时才创建
		Name("__default__").                           // Bean 名称
		Destroy(CloseDB)                               // 销毁时清理资源
}

func NewDB(config Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(config.DSN), &gorm.Config{})
}

func CloseDB(db *gorm.DB) error {
	sqlDB, _ := db.DB()
	return sqlDB.Close()
}
```

上例中，`gs.TagArg("${spring.gorm}")` 表示将 `spring.gorm` 前缀下的配置绑定到构造函数参数；`gs.OnProperty("spring.gorm.addr")` 表示只有配置了该属性时才启用这个 Bean；`Destroy(CloseDB)` 则把资源释放逻辑纳入应用停止流程。

### Module：按条件动态注册

`Module` 适合注册逻辑需要根据配置或环境动态展开的场景。它可以在运行注册阶段读取配置，并据此决定注册哪些 Bean。

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

func init() {
	// 注册 module，支持前置 Condition
	gs.Module(gs.OnProperty("enable-mysql", true), func(r gs.BeanProvider, p flatten.Storage) error {
		// 根据配置动态决定注册逻辑
		if p.GetBool("enable-readonly", false) {
			r.Provide(NewReadOnlyDB)
		} else {
			r.Provide(NewDB)
		}
		return nil
	})
}
```

当一个 starter 需要根据配置切换实现、注册一组相关 Bean，或者执行较复杂的条件判断时，`Module` 会比单纯的 `Provide` 更合适。

### Group：注册多个同类型实例

`Group` 是面向多实例配置的注册形式，常用于多数据库、多 Redis、多客户端等场景。它会遍历配置数组，并为每一项配置创建独立 Bean。

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 根据 spring.gorm.instances 配置数组创建多个 DB 实例
	// 每个实例使用数组中对应的配置项
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

这类配置通常写成数组结构，每个元素对应一个实例的配置。通过 `Group` 注册后，Starter 不需要手动解析数组，也不需要为每个实例单独编写注册代码。

## 自定义 Starter

官方 Starter 通常采用“默认单实例 + 可选多实例”的注册模式。自定义 Starter 也建议遵循这一约定，这样应用侧的配置和使用方式会更加统一。

```go
func init() {
	// 1. 默认单实例
	gs.Provide(NewClient, gs.TagArg("${spring.myclient}")).
		Condition(gs.OnProperty("spring.myclient.addr")).
		Name("__default__")

	// 2. 多实例支持（可选，根据组件特性决定是否提供）
	gs.Group("${spring.myclient.instances}", NewClient, DestroyClient)
}
```

建议遵循以下命名与配置规范：

- 配置前缀使用 `spring.xxx` 或 `spring.xxx.yyy` 格式，并与组件名称保持一致。
- 默认单实例使用 `__default__` 作为 Bean 名称。
- 默认单实例建议通过关键配置项触发，例如 `spring.xxx.addr`。
- 多实例配置建议统一放在 `spring.xxx.instances` 数组下。
- 配置结构体建议命名为 `Config`，并放在 starter 包内维护。
- 资源型组件应提供 `Destroy` 函数，确保应用停止时可以优雅释放连接、文件句柄或后台任务。

## 使用建议

- **按需实例化**：IoC 容器只创建被依赖且满足条件的 Bean。即使引入了 starter，未被使用的组件也不会无条件初始化。
- **配置驱动**：推荐通过配置或环境变量启用组件，并使用 `gs.OnProperty()` 表达启用条件。
- **条件组合**：Go-Spring 支持 `OnProperty`、`OnBean`、`OnProfile`、`OnMissingBean` 等条件，可根据组件特性组合使用。
- **模式统一**：基础组件优先采用“单实例 + Group 多实例”模式，降低应用侧理解成本。
- **生命周期完整**：数据库连接、网络客户端、服务端监听器等资源型组件必须接入销毁流程。

## 官方 Starter

Go-Spring 已经提供了一些常见基础组件的 Starter，可直接用于应用开发。

| Starter | 说明 |
|---------|------|
| `starter-gorm-mysql` | MySQL 集成，基于 GORM |
| `starter-go-redis` | Redis 集成，基于 go-redis |
| `starter-redigo` | Redis 集成，基于 redigo |
| 内置 HTTP Server | 详见 [内置 HTTP Server](05-http-server.md) |
| `starter-grpc` | gRPC Server 集成 |
| `starter-thrift` | Thrift Server 集成 |
| `starter-pprof` | pprof 性能分析服务 |
