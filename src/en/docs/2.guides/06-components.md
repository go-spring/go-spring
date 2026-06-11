# Components and the Starter Mechanism

Starter is the component modularization approach recommended by Go-Spring.
It can encapsulate the registration logic, configuration binding, enablement conditions, and lifecycle management for a group of Beans in an independent package,
allowing an application to obtain complete component capabilities with a single import.

For business applications, the value of Starter is that it reduces integration costs:
infrastructure such as databases, Redis, HTTP Server, and pprof does not need repeated initialization code in every project.
For component authors, Starter provides a clear set of encapsulation conventions, making publishing, reuse, and maintenance easier.

## Core Mechanism

A Starter usually completes registration through Go's `init()` function.
An application only needs to import the starter package, and the registration logic in the package will execute before the program starts:

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

Blank imports are suitable for scenarios where only side-effect registration is triggered. As long as the starter package can be seen by the Go linker,
Go-Spring can discover and process the Beans, Modules, or Groups it registered when the application starts.

It is important to note that importing a starter does not mean instances are created immediately.
Go-Spring's IoC container **instantiates Beans on demand**:
only objects that are depended on, satisfy conditions, and enter the container creation process will actually be initialized.

## Registration Forms

There are three common registration forms for Starters: `gs.Provide`, `gs.Module`, and `gs.Group`.
They are intended for simple single instances, dynamic registration logic, and multi-instance configuration scenarios respectively.

### Provide: Register a Single Bean

`gs.Provide` is used to directly register a Bean and is suitable when a component provides a single instance.
Through chained calls, you can declare configuration injection, enablement conditions, Bean names, and destroy functions at the same time.

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	// Basic usage
	gs.Provide(NewDB)

	// Complete usage: configuration injection + conditional enablement + naming + lifecycle management
	gs.Provide(NewDB, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")). // Create only when the property exists
		Name("__default__").                         // Bean name
		Destroy(CloseDB)                             // Clean up resources on destroy
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

In the example above, `gs.TagArg("${spring.gorm}")` means binding the configuration under the `spring.gorm` prefix to the constructor argument;
`gs.OnProperty("spring.gorm.dsn")` means this Bean is enabled only when that property is configured.

### Module: Dynamically Register by Condition

`gs.Module` is suitable when registration logic needs to expand dynamically based on configuration or environment.
It can read configuration during the registration phase and decide which Beans to register accordingly.

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

func init() {
	// Register a Module, supporting a preceding Condition
	gs.Module(
		// Register only when enable-mysql is true
		gs.OnProperty("enable-mysql").HavingValue("true"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			// Dynamically determine registration logic based on configuration
			if s, _ := p.Value("enable-readonly"); s == "true" {
				r.Provide(NewReadOnlyDB)
			} else {
				r.Provide(NewDB)
			}
			return nil
		})
}
```

When a starter needs to switch implementations based on configuration, register a group of related Beans, or perform more complex conditional checks,
`gs.Module` is more suitable than using only `gs.Provide`.

### Group: Register Multiple Instances of the Same Type

`gs.Group` is a registration form for multi-instance configuration, commonly used for scenarios such as multiple databases, multiple Redis instances, and multiple clients.
It iterates over the configuration dictionary and creates an independent Bean for each configuration.

```go
package starter

import (
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Create multiple DB instances from the spring.gorm.instances configuration dictionary
	// Each instance uses one item from the configuration
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

This type of configuration is usually written as a dictionary: the key is the Bean name, and the value is the configuration for the corresponding instance.

```yaml
spring:
  gorm:
    instances:
      db1:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
      db2:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
```

After registration through `gs.Group`, the Starter does not need to manually parse arrays or write separate registration code for each instance.

## Custom Starter

Official Starters usually use the registration pattern of "default single instance + optional multiple instances".
Custom Starters are also recommended to follow this convention so that application-side configuration and usage remain more consistent.

```go
func init() {
	// Register the default single instance.
	// This instance is created only when spring.gorm.dsn is configured.
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	// Register multiple instances.
	// Each instance is created from the configuration in spring.gorm.instances.
	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

It is recommended to follow these naming and configuration conventions:

- Use `spring.xxx` or `spring.xxx.yyy` as the configuration prefix, and keep it consistent with the component name.
- Use `__default__` as the Bean name for the default single instance.
- The default single instance should preferably be triggered by a key configuration item, such as `spring.xxx.addr`.
- Multi-instance configuration should preferably be placed uniformly under the `spring.xxx.instances` configuration dictionary.
- Resource components should provide a `Destroy` function to ensure that connections, file handles, or background tasks can be gracefully released when the application stops.

## Official Starters

Go-Spring provides some common infrastructure component Starters out of the box, which can be used directly for application development.

| Starter | Description |
|---------|------|
| `starter-gorm-mysql` | MySQL integration, based on GORM |
| `starter-go-redis` | Redis integration, based on go-redis |
| `starter-redigo` | Redis integration, based on redigo |
| Built-in HTTP Server | See [Built-in HTTP Server](/en/docs/2.guides/05-http-server.md) |
| `starter-pprof` | pprof performance profiling service |
