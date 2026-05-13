# Go-Spring 实战第 28 课：别再复制初始化代码，用 Starter 封装组件

HTTP Server 展示的是一个内置组件怎样接入生命周期。把视角再放大一点，很多基础设施组件也会遇到同样的问题：配置怎样绑定，Bean 怎样注册，条件怎样判断，资源怎样释放。

如果一个组件要被多个项目反复集成，直接复制注册代码很快就会失控。

Starter 是 Go-Spring 推荐的组件模块化方式。它把一组 Bean 注册、配置绑定、启用条件和生命周期管理封装到独立包中，让应用通过一次导入获得完整组件能力。

对于应用来说，Starter 降低集成成本；对于组件作者来说，Starter 提供可复用的封装约定。换句话说，它把“怎么接入”这件事从业务项目里拿出去。

## 核心机制

Starter 通常通过 Go 的 `init()` 注册：

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

空白导入触发包内注册逻辑。只要 starter 包被导入，Go-Spring 就能在启动时发现它注册的 Bean、Module 或 Group。

导入 Starter 不等于立即创建实例。容器仍然按需创建：只有满足条件、进入依赖图的 Bean 才会实例化。所以空白导入只是让注册信息进入系统，并不等于资源马上被打开。

## Provide：注册单个 Bean

`gs.Provide` 适合提供单个实例：

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

这里同时体现了配置绑定、条件启用、Bean 命名和生命周期释放。这样我们把组件集成所需的动作放在 Starter 内部，应用侧只需要提供配置。

## Module：按条件动态注册

`gs.Module` 适合注册逻辑需要读取配置后展开的场景：

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

## Group：注册多个同类型实例

`gs.Group` 面向多实例配置，例如多数据库、多 Redis、多客户端：

```go
func init() {
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

配置通常是字典结构：

```yaml
spring:
  gorm:
    instances:
      db1:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
      db2:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
```

接着，字典 key 作为 Bean 名称，value 作为实例配置。

## 自定义 Starter 约定

官方 Starter 通常采用“默认单实例 + 可选多实例”的模式：

```go
func init() {
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

建议遵循：

- 配置前缀使用 `spring.xxx` 或 `spring.xxx.yyy`。
- 默认单实例使用 `__default__` 作为 Bean 名称。
- 默认单实例通过关键配置项触发。
- 多实例配置放在 `spring.xxx.instances` 下。
- 资源型组件提供 Destroy 函数。

这些约定能让不同 Starter 在应用侧有一致使用体验。否则每个 Starter 都有自己的命名和配置习惯，集成成本会重新变高。

## 官方 Starter

Go-Spring 提供常见基础设施 Starter：

| Starter | 说明 |
|---------|------|
| `starter-gorm-mysql` | MySQL 集成，基于 GORM |
| `starter-go-redis` | Redis 集成，基于 go-redis |
| `starter-redigo` | Redis 集成，基于 redigo |
| 内置 HTTP Server | 默认 Web 服务接入 |
| `starter-pprof` | pprof 性能分析服务 |

## Starter 是封装，不是新机制

Starter 本质上仍然使用 Bean 注册 API。它不是另一套机制，而是把 Provide、Module、Group、条件注册、配置绑定和生命周期封装成可复用包。

组件能封装，也要能验证。接下来进入测试体系，看看 Go-Spring 项目如何组织纯单测、IoC 测试、断言和 Mock。
