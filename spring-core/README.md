# Go-Spring

<div>
   <img src="https://img.shields.io/github/license/go-spring/spring-core" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/spring-core" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/spring-core?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/spring-core" >
      <img src="https://codecov.io/gh/go-spring/spring-core/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://goreportcard.com/report/github.com/go-spring/spring-core">
      <img src="https://goreportcard.com/badge/github.com/go-spring/spring-core" alt="Go Report Card"/>
   </a>
   <a href="https://deepwiki.com/go-spring/spring-core">
      <img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki">
   </a>
</div>

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

**Go-Spring is a high-performance framework for modern Go application development,
inspired by Spring / Spring Boot from the Java ecosystem.**

Its design philosophy deeply integrates native Go language features,
inheriting mature development paradigms from the Spring ecosystem —
Dependency Injection (DI), auto-configuration, and lifecycle management —
while avoiding the complexity and performance overhead
that traditional frameworks may incur.

Go-Spring allows developers to enjoy the convenience of high-level abstraction and
automated development while maintaining Go's native style and execution efficiency.

**Whether you're building monolithic applications or
constructing distributed microservices systems,
Go-Spring provides a unified and flexible development experience.**

The framework simplifies project initialization in an "out-of-the-box" way,
reduces boilerplate code, and does not enforce an intrusive architecture,
allowing developers to focus on implementing business logic.

Go-Spring is committed to improving development efficiency,
enhancing maintainability, and ensuring system consistency,
making it a milestone framework in the Go ecosystem.

## 1. 🚀 Features Overview

Go-Spring combines mature design ideas of dependency injection and auto-configuration,
adheres to Go's philosophy of "simplicity is beauty",
and provides rich practical features to help developers
efficiently build modern Go applications:

1. ⚡ **Extreme startup performance, zero reflection at runtime**
   - Pre-registers beans based on Go's native `init()` mechanism, **no runtime scanning**,
     startup takes only milliseconds;
   - Reflection is only used during **initialization phase** to complete dependency injection,
     after initialization **zero reflection throughout runtime**,
     performance comparable to handwritten code.

2. 🧩 **Non-intrusive IoC container**
   - No forced interface dependencies or inheritance structure,
     business logic maintains native Go style, truly non-intrusive;
   - Supports standalone dependency injection usage,
     can also be used for full-stack framework development,
     flexible and unbound, fully compatible with Go standard library;
   - Provides complete bean lifecycle management,
     natively supports `Init` and `Destroy` hooks.

3. 💉 **Flexible and diverse Bean dependency injection**
   - Supports multiple injection methods: struct field injection, constructor injection,
     constructor parameter injection;
   - Supports multiple matching strategies by type, name, and tags,
     covering various scenario requirements.

4. 🏷️ **Convenient Value configuration binding**
   - Configuration values are directly bound to struct fields, no manual parsing required;
   - Supports default value syntax `${key:=default}`, elegant fallback;
   - Built-in field validation, automatically checks configuration correctness.

5. 🎯 **Powerful conditional injection system**
   - Supports dynamically deciding whether to register a bean
     based on configuration, environment, context and other conditions;
   - Provides multiple commonly used condition types,
     supports logical combinations (AND/OR/NOT);
   - Lays a solid foundation for modular auto-wiring.

6. ⚙️ **Layered configuration system**
   - Supports multi-source (command line, environment variables, configuration files, memory) and
     multi-format (YAML, TOML, Properties) configuration loading;
   - Clear configuration priority layering, automatic overriding,
     natively supports multi-environment isolation;
   - Supports configuration import, can integrate remote configuration centers to
     meet cloud-native deployment requirements.

7. 🔄 **Hot configuration reload, real-time effective**
   - Original `gs.Dync[T]` generic natively supports hot reloading,
     configuration changes don't require application restart;
   - **Fully compatible with Value binding syntax**, consistent usage, easy to get started;
   - Configuration automatically synchronizes to fields, enabling gray release and
     online parameter tuning in one step.

8. 🏗️ **Modular auto-wiring**
   - Modular auto-wiring based on conditional injection;
   - Modular design, assembled on demand, truly out-of-the-box;
   - The ecosystem provides rich Starter modules
     for quick integration of various functions.

9. 🔌 **Clear application runtime model**
   - Abstracts two runtime models: `Runner` (one-time tasks) and `Server` (long-running services);
   - Built-in HTTP Server launcher, supports concurrent startup of multiple services;
   - Complete lifecycle hooks, supports graceful startup/shutdown and signal handling.

10. 🧪 **Native `go test` integration for testing**
    - `gs.RunTest()` starts the container with one click, automatically completes dependency injection;
    - Automatic graceful shutdown after tests, no extra scaffolding code required.

11. 🪵 **Out-of-the-box logging system in the ecosystem**
    - Go-Spring ecosystem provides natively integrated structured logging module;
    - Unified logging interface, supports multiple outputs,
      adaptable to various logging implementations.

## 2. 📦 Installation

Go-Spring uses Go Modules for dependency management, installation is straightforward:

```bash
go get github.com/go-spring/spring-core
```

## 3. 🚀 Quick Start

Go-Spring prides itself on being "out-of-the-box".
Below are two examples to quickly experience the framework's powerful capabilities:
- **Example 1** shows how Go-Spring **perfectly integrates with the standard library**
  without changing your coding habits
- **Example 2** shows the core framework features such as dependency injection,
  configuration binding, dynamic refresh, etc.

### Example 1: Minimal API Service (Seamless integration with standard library)

This example demonstrates Go-Spring's **perfect compatibility**
with the standard library `net/http`.
You can use standard library writing directly,
and the framework only handles lifecycle management:

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func main() {
	// Define routes entirely using Go standard library http.Handler
	// Go-Spring won't force you to replace it with framework-specific routing syntax
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world!"))
	})

	// Start the application with just one line, the framework automatically takes over:
	// - Signal handling (graceful exit on Ctrl+C)
	// - Lifecycle management
	// - Automatic waiting for all services to exit
	gs.Run()
}
```

Access the service:

```bash
curl http://127.0.0.1:9090/echo
# Output: hello world!
```

This minimal example already reflects Go-Spring's design philosophy:

- ✅ **Non-intrusive compatibility**: Directly use Go standard library `http`, no code rewriting needed
- ✅ **Zero configuration startup**: No complicated configuration files, just `gs.Run()` to run
- ✅ **Lifecycle enhancement**: The framework automatically handles signal capture and graceful exit,
   eliminating template code handwriting
- ✅ **Gradual integration**: You can use only lifecycle management,
  or gradually introduce DI and configuration capabilities

### Example 2: Core Features Showcase (Dependency Injection + Dynamic Configuration)

This example demonstrates how multiple core features of Go-Spring work together,
showcasing **dependency injection**, **configuration binding**, **dynamic configuration hot reloading**,
**custom configuration at startup** and other capabilities:

```go
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-spring/spring-core/gs"
)

const timeLayout = "2006-01-02 15:04:05.999 -0700 MST"

// Register Bean during init() phase, based on Go native mechanism, no runtime scanning needed
func init() {
	gs.Provide(&Service{})

	// Parameter s *Service will be automatically injected by the framework
	gs.Provide(func(s *Service) *gs.HttpServeMux {
		http.HandleFunc("/echo", s.Echo)
		http.HandleFunc("/refresh", s.Refresh)
		return &gs.HttpServeMux{Handler: http.DefaultServeMux}
	})
}

type Service struct {
	// Auto-inject configuration refresher by type
	AppConfig   *gs.PropertiesRefresher `autowire:""`
	// Bind configuration value to field through value tag
	StartTime   time.Time               `value:"${start-time}"`
	// Use gs.Dync[T] generic to support hot reloading, automatically syncs after configuration changes
	RefreshTime gs.Dync[time.Time]      `value:"${refresh-time}"`
}

func (s *Service) Echo(w http.ResponseWriter, r *http.Request) {
	str := fmt.Sprintf("start-time: %s refresh-time: %s",
		s.StartTime.Format(timeLayout),
		s.RefreshTime.Value().Format(timeLayout))
	w.Write([]byte(str))
}

func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	// Simulate environment variable change, trigger configuration refresh.
	// According to priority rules, environment variables have higher priority
	// than in-memory configuration
	os.Setenv("GS_REFRESH-TIME", time.Now().Format(timeLayout))
	// Call refresh interface, all fields wrapped with Dync will be automatically updated
	s.AppConfig.RefreshProperties()
	w.Write([]byte("OK!"))
}

func main() {
	// Set configuration through code during application startup phase,
	// this belongs to the in-memory configuration level
	gs.Configure(func(app gs.App) {
		app.Property("start-time", time.Now().Format(timeLayout))
		app.Property("refresh-time", time.Now().Format(timeLayout))
	}).Run()
}
```

Access the service:

```bash
curl http://127.0.0.1:9090/echo     # Check current time (start time and refresh time)
curl http://127.0.0.1:9090/refresh  # Trigger hot refresh, refresh time will update
```

This example covers many core features of Go-Spring:

- ✅ **Bean registration and dependency injection**: Register beans via `gs.Provide()`,
  framework automatically completes dependency injection
- ✅ **Custom configuration at startup**: Supports dynamic configuration through code
  during application startup, flexibly adapts to different scenarios
- ✅ **Automatic configuration binding**: `value` tag directly binds configuration
  to struct fields, no manual parsing required
- ✅ **Layered configuration system**: Follows priority rules:
  environment variables > in-memory configuration > default values,
  naturally supports multiple environments
- ✅ **Dynamic configuration hot reloading**: Natively supported via `gs.Dync[T]` generic,
  configuration changes take effect in real-time without application restart
- ✅ **Configuration refresh mechanism**: Provides `PropertiesRefresher`
  to support manual triggering of configuration reload, can be used with configuration centers

## 4. 🧩 Bean Management

In Go-Spring, **Beans are the core building blocks of your application**,
similar to the concept of "components" in other dependency injection frameworks.
The entire system is organized around Bean registration, initialization,
dependency injection, and lifecycle management.

Go-Spring's design philosophy is **"Ready at compile time, minimal at runtime"**:
- No reliance on runtime scanning, all beans complete registration metadata collection
  during the `init()` phase
- Reflection is only used during **initialization phase** to complete dependency injection,
  after initialization **zero reflection throughout runtime**
- Type safety is guaranteed by the Go compiler, runtime performance comparable
  to handwritten code

This design fundamentally avoids the performance overhead and debugging complexity
caused by runtime reflection in traditional IoC frameworks,
making it particularly suitable for building **high-performance, maintainable large-scale systems**.

The framework adopts a combination approach of **"explicit registration + tag declaration + conditional assembly"**:
- **Explicit registration**: All beans must be explicitly registered, no implicit scanning,
  dependencies are clear at a glance
- **Tag declaration**: Concise declaration of injection rules through tags, no redundant configuration
- **Conditional assembly**: Supports dynamic registration decisions based on environment,
  naturally adapts to modular design

Since it doesn't rely on runtime container scanning and there's no "magic configuration",
this approach improves debugging and operational controllability
while maintaining a good development experience,
truly achieving the goal of **zero intrusion, (runtime) zero reflection**.

### 1️⃣ Registration Methods

Go-Spring provides two ways to register beans:

- **`gs.Provide(objOrCtor, args...)`** - Register **global Beans** in `init()` functions
- **`app.Provide(objOrCtor, args...)`** - Register Beans during the **application configuration phase**

Example:

```go
// Register global Bean in init()
func init() {
	gs.Provide(&Service{})        // Register struct instance
	gs.Provide(NewService)        // Register using constructor
	gs.Provide(NewRepo, gs.ValueArg("db")) // Constructor with parameters
}

// Register Bean in application configuration
gs.Configure(func(app gs.App) {
	app.Provide(&MyService{})
	app.Root(&Bootstrap{}) // Mark as root Bean, triggers dependency injection
})
```

> **💡 On-demand instantiation and `Root`**
> Go-Spring defaults to an **on-demand instantiation** strategy —
> only beans that are depended on or marked as `Root` will be instantiated.
> Beans marked via `app.Root()` serve as entry points to the application,
> and the framework automatically completes their dependency injection and instantiation.

### 2️⃣ Injection Methods

Go-Spring provides multiple flexible dependency injection methods.

#### 1. Struct Field Injection

Inject configuration items or beans into struct fields through tags,
**suitable for most scenarios** and is the most commonly used injection method.

```go
type App struct {
	Logger    *Logger      `autowire:""`           // Auto-inject Bean by type
	Filters   []*Filter    `autowire:"access,*?"  // Inject multiple Beans, allowed to not exist
	StartTime time.Time    `value:"${start-time}" // Bind configuration value
}
```

Syntax explanation:

- `value:"${key}"` or `value:"${key:=default}"`: Binds configuration value to field, supports default values
- `autowire:""`: Automatic injection by **type**, matches directly when type is unique
- `autowire:"?"`: Inject by type, allows non-existence, field is nil if not exists
- `autowire:"name?"`: Match by type and name, allows non-existence, field is nil if not exists
- `autowire:"a,*?"`: First match name a, then inject remaining beans of the same type,
  injection order matches registration order
- `autowire:"a,b,c"`: Exact match multiple beans by specified names,
  order strictly matches declaration order
- `autowire:"a,*?,b"`: Exact match multiple specified beans by name,
  keep remaining other beans, overall ordered

#### 2. Constructor Injection

Automatic injection through function parameters, Go-Spring automatically infers
and matches dependent beans.

```go
func NewService(logger *Logger) *Service {
	return &Service{Logger: logger}
}

gs.Provide(NewService)
```

#### 3. Constructor Parameter Injection

You can **explicitly specify the injection behavior** for each parameter
through parameter wrappers, more suitable for complex construction logic.

```go
gs.Provide(NewService,
	gs.TagArg("${log.level}"), // Inject value from configuration
	gs.ValueArg(8080),         // Inject fixed value directly
	gs.BindArg(connectDB),     // Inject after processing through function
)
```

Available parameter types:

| Parameter Wrapper | Purpose | Usage Scenario |
|-----------|------|---------|
| `gs.TagArg("${key}")` | Extract value from configuration and inject | Need configuration value directly as constructor parameter |
| `gs.ValueArg(val)` | Inject fixed value | Know the parameter value definitely, no need to get from container |
| `gs.IndexArg(i, arg)` | Specify injection by parameter position | Need to skip certain parameters, or customize injection for specific parameters |
| `gs.BindArg(fn, args...)` | Inject after processing via function | Need conversion or custom processing for injected values |

While this approach may seem slightly verbose, it gives you **complete control over the injection process**,
which is very useful in complex scenarios.

### 3️⃣ Lifecycle and Configuration Options

Go-Spring provides rich APIs for configuring bean metadata, lifecycle hooks, and dependencies.
Through method chaining, you can fully define all behaviors of a bean.

```go
gs.Provide(NewService).
	Name("myService").                           // Specify Bean name
	Init(func(s *Service) { ... }).              // Initialization function
	Destroy(func(s *Service) { ... }).           // Destruction function
	Condition(gs.OnProperty("feature.enabled")). // Conditional registration
	DependsOn(gs.BeanIDFor[*Repo]()).            // Declare explicit dependency
	Export(gs.As[ServiceInterface]()).           // Export as interface
	Export(gs.As[gs.Runner]())                   // Supports multiple interface exports
```

Complete configuration option description:

| Option | Purpose | Description |
|------|------|------|
| `Name(string)` | Specify Bean name | Used to distinguish when multiple beans of the same type exist, used with `autowire:"name"` |
| `Init(fn)` | Initialization function | Called after bean dependency injection is complete, also supports `InitMethod("Init")` specification by method name |
| `Destroy(fn)` | Destruction function | Called when application shuts down, also supports `DestroyMethod("Close")` specification by method name |
| `DependsOn(...)` | Declare dependency | Specify other beans that this bean depends on, ensuring correct initialization order |
| `Condition(...)` | Conditional registration | Only registers the current bean when condition is met, skips registration otherwise |
| `Export(as)` | Interface export | Register the bean as a specific interface to the container, convenient for injection by interface, supports multiple calls to export multiple interfaces |

### 4️⃣ Configuration Classes and Sub-Beans

Go-Spring supports capabilities similar to Spring Boot's `@Configuration` —
you can mark a bean as a **configuration class**,
and the framework will automatically scan the configuration class's methods
and automatically register method return values as sub-beans.
This approach is very suitable for organizing configuration modularly.

#### Usage

```go
// Define configuration class
type DataSourceConfig struct {}

// Method return values are automatically registered as Beans
func (c *DataSourceConfig) PrimaryDB() *sql.DB {
	// Write your database connection creation logic here
	return &sql.DB{ /* ... */ }
}

// Multiple methods can define multiple related Beans
func (c *DataSourceConfig) ReplicaDB() *sql.DB {
	return &sql.DB{ /* ... */ }
}

func init() {
	// Mark as configuration class via .Configuration(),
	// framework automatically scans and registers all sub-beans
	gs.Provide(&DataSourceConfig{}).Configuration()
}
```

#### Inclusion/Exclusion Rules

You can precisely control which methods need to be scanned and registered using regular expressions:

```go
func init() {
	// Only include methods matching New.* pattern
	gs.Provide(&Config{}).Configuration(gs.Configuration{
		Includes: []string{"New.*"}, // Include patterns
		Excludes: []string{"Test.*"}, // Exclude patterns
	})
}
```

- If you don't specify `Includes`, it defaults to scanning all public methods
- Regex syntax follows Go standard `regexp` package specification,
  please avoid incomplete regular expressions like `*`
- Scanned methods must **return Bean instances**, supports two signatures: `(T)` or `(T, error)`

## 5. ⚙️ Configuration Management

Go-Spring provides a **layered-designed, flexible and powerful** configuration management system
that supports loading configuration from multiple sources, natively meeting enterprise requirements
such as multi-environment isolation and dynamic updates.
Whether for local development, containerized deployment, or cloud-native architecture,
Go-Spring provides a consistent and concise configuration experience.

The framework automatically merges configuration items from different sources at startup
and automatically overrides according to **priority rules**,
so you don't need to manually handle configuration merging logic.

Go-Spring supports three mainstream configuration formats out of the box:
**YAML** (`.yaml`/`.yml`, recommended), **Properties** (`.properties`), and **TOML** (`.toml`).
The framework automatically recognizes the format based on the file extension.

### 1️⃣ 🔖 Configuration Binding

The most convenient way to configure in Go-Spring
is to bind configuration directly to struct fields via the `value` tag,
no manual parsing required:

```go
type ServerConfig struct {
	Port    int    `value:"${server.port:=8080}"`      // With default value
	Host    string `value:"${server.host:=localhost}"` // With default value
	Enabled bool   `value:"${server.enabled:=true}"`   // Boolean type
}
```

**Syntax explanation:**
- `${key}`: Binds the value of configuration key `key` to the field
- `${key:=default}`: Uses `default` as the default value if the configuration key doesn't exist
- Supports almost all Go primitive types: `int`/`int64`/`uint`/`float64`/`bool`/`string`, etc.,
  also supports custom types like `time.Duration`

### 2️⃣ 📌 Configuration Priority

Go-Spring adopts a clear **priority layering** design,
where higher priority configurations automatically override
lower priority configurations with the same name.
Priorities are listed from highest to lowest below:

| Priority | Configuration Source | Description | Usage Scenario |
|:------:|----------------|------|---------|
| 1 ⬆️ | **Command Line Arguments** | `-Dkey=value` | Temporary override configuration, quick debugging verification |
| 2 | **Environment Variables** | System environment variables | Containerized deployment, twelve-factor apps |
| 3 | **profile configuration** | `app-{profile}.ext` | Multi-environment isolation (dev/test/prod) |
| 4 | **app base configuration** | `app.ext` | Default base configuration |
| 5 | **In-memory configuration** | Programmatic setting via `app.Property()` | Unit testing, dynamic override |
| 6 ⬇️ | **Tag default values** | `${key:=default}` | Final fallback, default values |

> **💡 Core Priority Rule**
> **Later-loaded configurations have higher priority**, configurations closer to runtime take precedence,
> which matches intuition.

> **💡 Configuration Import Rule**
> Both base configuration and profile configuration support importing external configurations via `spring.app.imports`,
> **Later imported configurations have higher priority than the file's original configuration**,
> overriding in import order.

### 3️⃣ 📝 Detailed Description of Each Configuration Source

#### 1. Command Line Arguments
Injected using `-Dkey=value` format, highest priority,
suitable for quickly overriding runtime configuration:
```bash
go run main.go -Dserver.port=9090 -Dapp.env=production
```

#### 2. Environment Variables
Directly read from OS environment variables, best practice for containerized deployment:
```bash
export SERVER_PORT=9090
export APP_ENV=production
export SPRING_PROFILES_ACTIVE=dev
```

> 💡 Go-Spring automatically converts underscores in environment variables to dots,
> for example `SERVER_PORT` maps to `server.port`.

#### 3. profile configuration (multi-environment isolation)
Achieves environment isolation by activating different profiles,
file naming format is `app-{profile}.{ext}`:
```bash
# Activate dev environment
export SPRING_PROFILES_ACTIVE=dev
```
The framework automatically loads `app-dev.yaml` (or other formats),
which has higher priority than base configuration.
Configurations imported in profile configuration also follow the later-import-first rule.

#### 4. Base Configuration File
By default loads `conf/app` with extension (e.g., `conf/app.yaml`),
suitable for storing general base configuration:
```
./conf/app.yaml
./conf/app.properties
```
Base configuration supports importing external configurations via `spring.app.imports`.

#### Configuration Import (import)
Both base configuration and profile configuration support importing external configurations
via `spring.app.imports`, making it easy to split and reuse:

```yaml
# app.yaml
spring:
  app:
    imports:
      - "database.yaml"       # Split database configuration
      - "redis.yaml"          # Split Redis configuration
      - "nacos://server.json" # Import from remote configuration center (requires extension)
```

Importing executes in the order declared, **later imported configurations override keys
with the same name from earlier imports and original configuration**.
This mechanism is very suitable for configuration splitting
and integrating remote configuration centers (Nacos, etcd, etc.).

#### 5. Application In-Memory Configuration
Programmatic configuration setting via code during application startup,
commonly used for testing or dynamic scenarios:
```go
gs.Configure(func(app gs.App) {
    app.Property("app.name", "test-app")
    app.Property("feature.enabled", true)
})
```

#### 6. Struct Tag Default Values
Embedded default values through tags, serve as the final fallback for the configuration system:
```go
type Config struct {
	Port int    `value:"${server.port:=8080}"`
	Env  string `value:"${app.env:=development}"`
}
```

## 6. 🔍 Conditional Injection

Drawing inspiration from Spring's `@Conditional` concept,
Go-Spring implements a flexible and powerful conditional injection system.
It dynamically decides whether to register beans based on configuration,
environment, context and other conditions, achieving "assembly on demand".
This is particularly crucial in scenarios such as multi-environment deployment,
plugin architecture, feature toggles, and gray release.

### 1️⃣ 🎯 Common Condition Types

- **`gs.OnProperty("key")`**: Activates when the specified configuration key exists
- **`gs.OnBean[Type]("name")`**: Activates when a bean of the specified type/name exists
- **`gs.OnMissingBean[Type]("name")`**: Activates when a bean of the specified type/name does **not** exist
- **`gs.OnSingleBean[Type]("name")`**: Activates when the specified type/name bean is the only instance
- **`gs.OnFunc(func(ctx gs.ConditionContext) (bool, error))`**: Uses custom conditional logic to determine activation

Example:

```go
gs.Provide(NewService).
	Condition(gs.OnProperty("service.enabled"))
```

`NewService` will only be registered if `service.enabled=true` exists in the configuration file.

### 2️⃣ 🔁 Supports Combined Conditions

Go-Spring supports combining multiple conditions to build more complex judgment logic:

- **`gs.Not(...)`** - Negates a condition
- **`gs.And(...)`** - All conditions must be satisfied
- **`gs.Or(...)`** - Any condition being satisfied is sufficient
- **`gs.None(...)`** - All conditions must not be satisfied

Example:

```go
gs.Provide(NewService).
  Condition(
      gs.And(
          gs.OnProperty("feature.enabled"),
          gs.Not(gs.OnBean[*DeprecatedService]()),
      ),
  )
```

This bean will be enabled when `feature.enabled` is turned on
and `*DeprecatedService` is not registered.

## 7. 📦 Module and Starter Mechanism

Drawing inspiration from Spring Boot's Starter concept,
Go-Spring provides a **Module** mechanism to implement auto-configuration and modular assembly.
Through Module, you can organize related beans together to create "out-of-the-box" functional modules.

### 1️⃣ 🎯 What is a Module?

A Module is **Go-Spring's conditional configuration module** mechanism
that can dynamically decide whether to register a set of related beans
based on configuration properties.
This is ideal for:

- 🧩 **Developing starters for various functions** (such as Redis, MySQL, gRPC, etc.)
- 🏗️ **Organizing code by functional modules**, achieving loosely coupled architecture
- ⚡ **Automatically enabling/disabling features based on configuration**, truly assembled on demand

The core interface is very concise:

```go
gs.Module(condition gs.PropertyCondition, fn func(r gs.BeanProvider, p flatten.Storage) error)
```

- `condition`: Property condition, only when the condition is met
  will the beans inside the module be registered (usually created with `gs.OnProperty("key")`)
- `fn`: Module initialization function, register all beans of this module
  in batch within the function
- `r`: Bean registrar, usage is exactly the same as global `gs.Provide()`
- `p`: Configuration storage, you can read configuration from it for dynamic binding

### 2️⃣ 💡 Typical Scenario: Custom Starter

Suppose you want to develop a Redis Starter, you can organize your code like this:

```go
package redis

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

// Cache interface
type Cache interface {
	Get(key string) (string, error)
	Set(key string, value string) error
}

func init() {
	// Automatically enable the Redis module when redis.host configuration is detected
	// If the condition is not met, beans inside the module won't be registered,
	// doesn't affect application startup
	gs.Module(gs.OnProperty("redis.host"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			// 1. Register Redis Client, specify name, initialization and destruction methods
			r.Provide(NewRedisClient).
				Name("redisClient").
				InitMethod("Connect").   // Call Connect after dependency injection completes
				DestroyMethod("Close"). // Call Close when application shuts down to release resources

			// 2. Register Redis-based Cache implementation and export as Cache interface
			// This allows other components to inject by interface, decoupled from specific implementation
			r.Provide(NewRedisCache).
				Export(gs.As[Cache]())

			// You can continue to register other related beans...
			return nil
		})
}
```

It's very simple for users to use, just add to the configuration file:

```yaml
redis:
  host: localhost
  port: 6379
  password: xxx
  db: 0
```

Go-Spring will automatically detect the configuration,
and when the condition is met, automatically execute the module
to register all related beans. Users **don't need to write any manual registration code**,
truly out-of-the-box!

### 3️⃣ ✨ Special Usage: Group Batch Registration

Go-Spring also provides `gs.Group` convenient syntax for handling a common scenario:
**batch creating multiple beans from a map in configuration**.
Each map entry is automatically converted to a named bean,
with the map key as the bean name. Usage example:

```go
// Batch create multiple HTTP clients from configuration
gs.Group(
	"${http.clients}",           // Path to map-type configuration in configuration
	func(cfg HTTPClientConfig) (*HTTPClient, error) {
		return NewHTTPClient(cfg) // Create a client instance for each configuration entry
	},
	func(c *HTTPClient) error {
		return c.Close()          // Optional: destruction function for resource cleanup
	},
)
```

Corresponding YAML configuration:

```yaml
http:
  clients:
    serviceA:  # map key "serviceA" becomes the bean name
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:  # map key "serviceB" becomes the bean name
      baseURL: "http://b.example.com"
      timeout: 60s
```

This approach is very suitable for scenarios that require batch creation of beans based on configuration,
such as **multiple data sources**, **multi-tenancy**, **dynamic plugins**, etc.

## 8. 🔁 Dynamic Configuration

Go-Spring natively supports a **lightweight configuration hot update** mechanism.
Through the generic type `gs.Dync[T]` and `RefreshProperties()`,
applications can perceive configuration changes in real-time at runtime
without restarting the application.
This feature is very useful in scenarios such as **gray release**,
**dynamic parameter tuning**, and **configuration center integration**
in microservices architecture.

### 1️⃣ 🌡 Usage

Divided into two steps: **declare dynamic field** and **trigger refresh**.

#### 1. Declare dynamic fields using `gs.Dync[T]`

Wrap fields with the generic type `gs.Dync[T]`,
and the framework will automatically listen for configuration changes
and update in real-time:

```go
type Config struct {
	Version gs.Dync[string] `value:"${app.version}"` // Declare as dynamic configuration
}
```

To use, get the latest current value through the `.Value()` method:

```go
version := config.Version.Value() // Always gets the latest value
```

The framework will **automatically update** the internal value when the configuration changes,
no manual handling required from you.

#### 2. Call `RefreshProperties()` to trigger refresh

After external configuration changes, you need to inject `*gs.PropertiesRefresher`
and call its method to trigger the refresh:

```go
func RefreshHandler(w http.ResponseWriter, r *http.Request, refresher *gs.PropertiesRefresher) {
	// Simulate configuration change (in real scenarios it's usually pushed by configuration center)
	os.Setenv("APP_VERSION", "v2.0.1")
	// Trigger refresh, all gs.Dync[T] fields will update automatically
	_ = refresher.RefreshProperties()
	fmt.Fprintln(w, "Version updated!")
}
```

## 9. ⏳ Application Lifecycle and Service Model

Go-Spring abstracts components in the application runtime phase
into two core roles: `Runner` and `Server`, with clear division of responsibilities:

| Role | Execution Method | Typical Scenarios |
|:----:|:--------:|---------|
| **Runner** | One-time execution | Database initialization, cache warming, data migration and other startup tasks |
| **Server** | Long-running | HTTP services, gRPC services, WebSocket services, etc. |

All roles are registered via `.Export(gs.As[Interface]())`.

> **Design Note**: Early versions included a `Job` type for background scheduled tasks,
> but to simplify the model and reduce cognitive burden, it has been removed in the latest version.
> For background tasks that need to run continuously, it's recommended to implement directly
> using the `Server` interface and handle with a loop in the `Run` method.

### 1️⃣ Example: Runner

```go
package main

import (
	"context"
	"fmt"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Register Bootstrap and export as Runner interface
	gs.Provide(&Bootstrap{}).Export(gs.As[gs.Runner]())
}

type Bootstrap struct{}

func (b *Bootstrap) Run(ctx context.Context) error {
	fmt.Println("Bootstrap: Initialization completed...")
	return nil // If returns error, application startup will be terminated
}

func main() {
	gs.Run()
}
```

### 2️⃣ 📌 Custom Server

Go-Spring provides a generic `Server` interface that allows you
to conveniently integrate various service components.
All registered `Server`s automatically integrate into the application lifecycle,
and the framework handles general logic such as **concurrent startup**,
**graceful shutdown**, and **signal handling**.

**Server interface definition:**

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

- `Run(ctx context.Context, sig ReadySignal)`: Start the service,
  wait for the startup signal before officially providing services externally
- `Stop() error`: Gracefully shuts down the service, releases resources

**ReadySignal interface:**

```go
type ReadySignal interface {
	TriggerAndWait() <-chan struct{}
}
```

The role of `ReadySignal` is to **wait for all Servers to complete listening binding
before uniformly providing services externally**, avoiding errors caused
by accepting requests before startup completes.

### 3️⃣ Example: HTTP Server Integration

```go
package main

import (
	"context"
	"net"
	"net/http"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Provide(NewServer).Export(gs.As[gs.Server]())
}

type MyServer struct {
	svr *http.Server
}

// NewServer creates HTTP service instance
func NewServer() *MyServer {
	return &MyServer{
		svr: &http.Server{Addr: ":8080"},
	}
}

func (s *MyServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// Complete port listening binding first
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return err // Binding fails, return directly, terminate startup
	}
	// Wait for all servers to complete startup, then start accepting connections
	<-sig.TriggerAndWait()
	// Officially start serving
	return s.svr.Serve(ln)
}

func (s *MyServer) Stop() error {
	// Gracefully shutdown HTTP service
	return s.svr.Shutdown(context.Background())
}
```

### 4️⃣ Example: gRPC Server Integration

```go
package main

import (
	"context"
	"net"
	"github.com/go-spring/spring-core/gs"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	svr *grpc.Server
}

func (s *GRPCServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	lis, err := net.Listen("tcp", ":9595")
	if err != nil {
		return err
	}
	<-sig.TriggerAndWait() // Wait for all services to complete startup
	return s.svr.Serve(lis)
}

func (s *GRPCServer) Stop() error {
	s.svr.GracefulStop() // Graceful stop
	return nil
}
```

### 5️⃣ 💡 Multiple Servers Running Concurrently

All services registered via `.Export(gs.As[gs.Server]())` will be **started concurrently**
when `gs.Run()` is called, and exit signals will be handled uniformly:

```go
func init() {
	// HTTP and gRPC services run concurrently
	gs.Provide(&HTTPServer{}).Export(gs.As[gs.Server]())
	gs.Provide(&GRPCServer{}).Export(gs.As[gs.Server]())
}
```

After receiving an exit signal (such as Ctrl+C), the framework uniformly calls
the `Stop()` method of all servers to achieve graceful shutdown.

## 10. 🧪 Unit Testing

Thanks to Go-Spring's **non-intrusive design**, you can completely write unit tests
in native Go way, and there's no mandatory requirement to use special testing capabilities
from the framework.

For simple unit tests, just manually instantiate the test object
and pass in dependencies manually:

```go
func TestMyService(t *testing.T) {
	// Manually create dependencies (can use Mock)
	mockRepo := NewMockRepo()
	// Manually instantiate the service under test
	service := NewMyService(mockRepo)

	// Test directly, no need to start container
	result := service.DoSomething()
	assert.Equal(t, "ok", result)
}
```

### 1️⃣ When to use `gs.RunTest()`

When you need to write **integration tests** that require starting the complete container
and automatically completing dependency injection, Go-Spring provides `gs.RunTest()`
with native `go test` integration, which is very convenient:

```go
package main

import (
	"testing"
	"github.com/go-spring/spring-core/gs"
	"github.com/stretchr/testify/assert"
)

func TestExample(t *testing.T) {
	// gs.RunTest automatically creates container, completes dependency injection,
	// automatically closes after test
	gs.RunTest(t, func(ts *struct {
		DB    *MyDB    `autowire:""`
		Cache *Cache  `autowire:""`
	}) {
		// All dependencies are already automatically injected, ready to use directly
		result := ts.DB.Query("SELECT ...")
		assert.NotNil(t, result)
	})
}
```

### 2️⃣ ✨ Core Features

- ✅ **Fully native compatible**: Seamlessly integrated with standard `go test`, no special test runner required
- ✅ **Automatic dependency injection**: Declare required beans in the test parameter struct, framework injects automatically
- ✅ **Automatic resource cleanup**: Automatically calls destruction methods after tests, graceful shutdown

## 11. 📚 Comparison with Other Frameworks

Below is a feature comparison between Go-Spring and other mainstream Go dependency injection frameworks:

| Feature Point | Go-Spring | Wire | fx | dig |
|:-------|:---------:|:----:|:--:|:---:|
| Runtime IoC Container | ✓ | ✗ | ✓ | ✓ |
| No runtime scanning (pre-registration based on init()) | ✓ | ✓ | ✗ | ✗ |
| Zero reflection runtime (no reflection after initialization) | ✓ | ✓ | ✗ | ✗ |
| Compile-time type checking | Partial | ✓ | ✗ | ✗ |
| Conditional Beans support | ✓ | ✗ | ✗ | ✗ |
| Modular auto-wiring (Starter mechanism) | ✓ | ✗ | ✗ | ✗ |
| Dynamic configuration hot reloading | ✓ | ✗ | ✗ | ✗ |
| Lifecycle management | ✓ | ✗ | ✓ | ✗ |
| Configuration property auto-binding | ✓ | ✗ | ✗ | ✗ |
| Non-intrusive design (no modification to original struct) | ✓ | ✓ | ✗ | ✓ |

## 12. 🤝 Relationship with Other Go Ecosystems

Go-Spring **does not intend to replace any existing Go framework**,
but acts as a "glue" to help you integrate the entire Go ecosystem.

### 1️⃣ Design Philosophy

Go-Spring deeply respects Go's native ecosystem,
the framework is fully compatible with the standard library and various third-party frameworks:

- ✅ **Can be used with any web framework like Gin/Echo/Chi**,
  the framework doesn't force you to replace your routing syntax
- ✅ **Can be used with gRPC/protobuf ecosystem**, auto-wire services
- ✅ **Can be used with sql/database or ORM frameworks**, configuration-driven multiple data sources
- ✅ **Fully compatible with Go standard library `net/http`, `context`, etc.**, no intrusive modifications

### 2️⃣ Positioning and Division of Labor

| Component | What Go-Spring does | You can choose |
|:---:|----------|---------|
| **Dependency Injection** | ✅ Full responsibility | - |
| **Configuration Management** | ✅ Full responsibility (multi-source, hot reloading) | - |
| **Web Routing** | Optional integration | Gin, Echo, Chi, standard library `net/http` |
| **ORM/Database** | Optional integration | GORM, XORM, sqlx, standard library `database/sql` |
| **Logging** | Provides unified interface | Zap, Logrus, slog, etc. |
| **Service Discovery/Registration** | Integratable via Starter | etcd, Consul, Nacos |

In one sentence: **Go-Spring helps you manage dependencies and configuration well,
leaving the rest to the tools you're familiar with**.

## 13. 📖 Further Learning

Want to get started quickly? Check out these resources:

- 📖 **Complete Documentation**: [go-spring/go-spring](https://github.com/go-spring/go-spring)
- 💡 **Example Projects**: [go-spring/examples](https://github.com/go-spring/go-spring/tree/master/docs/4.examples)
- 📦 **Ecosystem Starters**: The [Go-Spring organization](https://github.com/go-spring)
  maintains many out-of-the-box modules

## 14. 🏢 Who's Using Go-Spring?

Many companies are using Go-Spring to build microservices applications in production:

- ...

> If your company or project is also using Go-Spring,
> feel free to submit a PR to showcase your project here!

## 15. 💬 Feedback and Communication

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/go-spring/spring-core/issues)
- 💡 **Feature Suggestions**: Welcome to submit an Issue to join the discussion
- ⭐ **Star Support**: If you like this project, feel free to give a star to encourage us!

## 16. 🤝 Contributing

Go-Spring is an open source community-driven project, we welcome **all forms of contributions**:

- Fix documentation errors
- Fix bugs
- Submit feature suggestions
- Contribute new features
- Share your usage experience

Please check [CONTRIBUTING.md](CONTRIBUTING.md) for how to participate.

### 💬 QQ Group

Welcome to join the QQ group for discussion:

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="140" alt="qq-group"/>

### 📱 WeChat Official Account

Follow the WeChat official account for the latest updates:

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="140" alt="wechat-public"/>

### 🎉 Acknowledgments

Thanks to JetBrains for the [IntelliJ IDEA](https://www.jetbrains.com/idea/) open source license,
which greatly facilitates project development.

### 🛡️ License

Apache License 2.0, see [LICENSE](LICENSE) for details.
