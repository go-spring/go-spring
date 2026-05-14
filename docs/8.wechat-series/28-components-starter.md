# Go-Spring 实战第 28 课：Starter 机制：把组件注册、配置和生命周期封装成可复用包

Go-Spring 的 HTTP Server 展示的是一个内置组件怎样接入生命周期。把视角再放大一点，很多基础设施组件也会遇到同样的问题，即配置怎样绑定，Bean 怎样注册，条件怎样判断，资源怎样释放。

如果一个组件要被多个项目反复集成，那直接复制注册代码很快就会失控。因为复制的不只是几行初始化代码，还有配置约定、启用条件和资源释放规则。

这时候，Starter 就是 Go-Spring 推荐的组件模块化方式。它把一组 Bean 注册、配置绑定、启用条件和生命周期管理封装到独立包中，让应用通过一次导入获得完整组件能力。

对于应用来说，Starter 降低集成成本；对于组件作者来说，Starter 提供了一套可复用的封装约定。组件怎么接入这件事，就从业务项目里移到了组件包里。

## Starter 通过空白导入进入注册表

应用侧通常只需要空白导入 starter 包。这个导入不会直接创建资源，只会触发包内 `init()`，把注册信息放进 Go-Spring 注册表。

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

空白导入触发包内注册逻辑。只要 starter 包被导入，Go-Spring 就能在启动时发现它注册的 Bean、Module 或 Group。

导入 Starter 不等于立即创建实例。容器仍然按需创建，即只有满足条件、进入依赖图的 Bean 才会实例化。所以空白导入只是让注册信息进入系统，并不等于资源马上被打开。

## Provide 封装默认单实例

`gs.Provide` 适合封装默认单实例。下面的 starter 只有在配置了 `spring.gorm.dsn` 时才创建默认数据库 Bean，并在容器关闭时释放连接。

```go
func init() {
	gs.Provide(NewDB, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__").
		Destroy(CloseDB)
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

这里同时体现了 Go-Spring 的配置绑定、条件启用、Bean 命名和生命周期释放。这样我们把组件集成所需的动作放在 Starter 内部，应用侧只需要提供配置。

## Module 封装配置驱动的动态注册

`gs.Module` 适合注册逻辑需要读取配置后展开的场景。下面的模块先看总开关，再根据配置选择只读实现或默认实现。

```go
func init() {
	gs.Module(
		gs.OnProperty("enable-mysql").HavingValue("true"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			if s, _ := p.Value("enable-readonly"); s == "true" {
				r.Provide(NewReadOnlyDB)
			} else {
				r.Provide(NewDB)
			}
			return nil
		})
}
```

如果 Starter 需要按配置切换实现、注册一组相关 Bean 或进行较复杂条件判断，Module 比 Provide 更合适。

## Group 封装多实例配置

`gs.Group` 面向多实例配置，例如多数据库、多 Redis、多客户端。下面这行表示从 `spring.gorm.instances` 字典下为每个条目生成一个数据库 Bean。

```go
func init() {
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

配置通常是字典结构。

```yaml
spring:
  gorm:
    instances:
      db1:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
      db2:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
```

接着，字典 key 就会作为 Bean 名称，value 作为实例配置。

## Starter 统一命名和配置约定

官方 Starter 通常用“默认单实例 + 可选多实例”的模式。

```go
func init() {
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

常见约定包括下面几类。

- 配置前缀使用 `spring.xxx` 或 `spring.xxx.yyy`。
- 默认单实例使用 `__default__` 作为 Bean 名称。
- 默认单实例通过关键配置项触发。
- 多实例配置放在 `spring.xxx.instances` 下。
- 资源型组件提供 Destroy 函数。

这些约定能让不同 Starter 在应用侧有一致使用体验。反过来，如果每个 Starter 都有自己的命名和配置习惯，集成成本会重新变高。

## 官方 Starter 提供了基础设施集成

Go-Spring 提供了常见基础设施 Starter。

| Starter | 说明 |
|---------|------|
| `starter-gorm-mysql` | MySQL 集成，基于 GORM |
| `starter-go-redis` | Redis 集成，基于 go-redis |
| `starter-redigo` | Redis 集成，基于 redigo |
| 内置 HTTP Server | 默认 Web 服务接入 |
| `starter-pprof` | pprof 性能分析服务 |

## Starter 只是注册能力的封装

Starter 本质上仍然使用 Go-Spring 的 Bean 注册 API。它不是另一套机制，而是把 Provide、Module、Group、条件注册、配置绑定和生命周期封装成可复用包。

组件封装之后，还需要配套验证。Go-Spring 的测试体系会继续说明纯单测、IoC 测试、断言和 Mock 怎样分层使用。
