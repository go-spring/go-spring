# IoC Container

> Inversion of Control (IoC) and Dependency Injection (DI) are core foundations of the Java Spring framework.
> Go-Spring inherits the design philosophy of Java Spring while preserving Go's native style, providing Go developers with a concise and efficient dependency injection framework.

## What Is Dependency Injection?

> If you are familiar with Java Spring, you can skip this section.

**Dependency injection** is a design pattern that helps us write more elegant and maintainable code:

1. **Decouple component dependencies**: Components do not need to create dependency objects by themselves; the container provides them centrally.
2. **Centralize lifecycle management**: Object creation, initialization, and destruction are all managed by the container.
3. **Make unit testing easier**: Dependencies can be easily replaced with Mock objects during testing.
4. **Improve maintainability**: Dependency relationships are clear and centrally managed instead of being scattered throughout the codebase.

Let's look at the difference through code. Without dependency injection, we usually create dependencies ourselves in the constructor:

```go
type UserController struct {
	service *UserService
}

// No DI: UserController must create UserService by itself, which is tight coupling.
func NewUserController() *UserController {
	return &UserController{
		// UserService is hard-coded here and cannot be flexibly replaced at creation time.
		service: NewUserService(),
	}
}
```

With dependency injection, we only need to **declare** what dependencies are needed in the constructor; we do not need to create them ourselves:

```go
type UserController struct {
	service UserService
}

// With DI: UserService is passed in from outside, and its implementation can be flexibly replaced. This is loose coupling.
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

### Why Do We Need an IoC Container?

The approach above already decouples components, but there is still one problem to solve: **who is responsible for creating these dependencies and passing them in?**

When an application has only two or three objects, we can assemble them manually:

```go
service := NewUserService()
controller := NewUserController(service)
```

However, when an application has dozens or hundreds of components with complex dependencies between them, continuing to assemble everything manually becomes very cumbersome.
**This is where the IoC container comes in**: it can automatically help us do the following:

1. Register all components into the container (this step is called **registering Beans**).
2. Automatically analyze dependencies between components.
3. Create all objects in the correct order.
4. Automatically inject dependencies where they are needed.
5. Manage the full lifecycle of components from creation to destruction.

In short: all component assembly and management can be handed over to the IoC container, allowing us to focus more on writing business logic.
This **is Go-Spring's mission**!

## Quick Start

Let's quickly understand how to use Go-Spring through a concise example:

```go
package main

import (
	"fmt"
	"net/http"

	// Import the Go-Spring core package.
	"go-spring.org/spring/gs"
)

// UserService is a business service. The whole application only needs one instance.
type UserService struct{}

// GetUser gets the user name.
func (s *UserService) GetUser() string {
	return "Alice"
}

// UserController is an HTTP controller that depends on UserService.
type UserController struct {
	service *UserService
}

// NewUserController is a constructor. Its parameters are dependency declarations.
// The container can automatically analyze parameter types and inject matching Beans.
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

// Hello handles /hello requests.
func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	user := c.service.GetUser()
	fmt.Fprintf(w, "Hello, %s!", user)
	fmt.Println("Hello,", user)
}

// init runs before program startup and registers all Beans with the container.
func init() {
	// Register UserService so it can be dependency-injected into other Beans.
	gs.Provide(new(UserService))

	// Register UserController. The container automatically resolves constructor parameters and injects dependencies.
	gs.Provide(NewUserController)

	// Register HTTP route configuration. Function-style Bean definitions are supported.
	// The returned HttpServeMux is recognized by the container and automatically starts an HTTP service.
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	// Start the Go-Spring application.
	// It automatically creates all Beans, injects dependencies, and starts the HTTP service.
	gs.Run()
}
```

Run the application with `go run main.go`, then visit `http://localhost:9090/hello`. You will see:

```
Hello, Alice!
```

The console will also output:

```
Hello, Alice
```

As you can see, the whole process is very concise: register services, controllers, and routes in `init()`, declare dependencies through constructors,
and then call `gs.Run()` to start the application. Dependency injection, object management, and HTTP service startup are all completed automatically by Go-Spring.

## Bean Definition

A **Bean** is a component managed centrally by the container. Its lifecycle, including creation, initialization, and destruction, is handled by the container.
This concept comes from Java Spring, but Go-Spring reinterprets it to better fit Go's design characteristics.

From the perspective of dependency injection, a Bean can be understood as **the smallest functional unit that can be managed and composed by the container**.
This understanding is not a simple migration; it is a natural extension of how Go code is organized.

In Go, packages are mainly used for code organization and namespace division, and their granularity is relatively coarse.
A package often contains multiple functional units that have different runtime dependencies,
while the language itself does not provide a unified management mechanism for these units.

Therefore, Go-Spring introduces Beans as runtime organizational units that complement packages:
it abstracts finer-grained functions into manageable objects, making it easier and more accurate to describe dependency relationships and lifecycles.

## Bean Injection

Go-Spring's dependency injection system can be understood from two dimensions:
first, the injection method, which means declaring dependencies through different syntax forms;
second, the injection target, which means the final shape in which dependencies are received and used.

### Injection Methods

Go-Spring supports **two injection methods**: struct field injection and constructor parameter injection.

#### Struct Field Injection

Struct field injection is the simplest and most intuitive injection method:
you only need to declare a dependency on a struct field with the `autowire` (or `inject`) tag,
and the container automatically injects the matching Bean into that field.

Example:

```go
// UserController depends on UserService.
type UserController struct {
	Service UserService `autowire:""`
}
```

This approach is concise and does not require manually writing constructors. It is suitable for business scenarios with simple dependencies.

#### Constructor Parameter Injection

Constructor parameter injection means that dependencies are passed in through constructor parameters.
When creating a Bean, the container automatically resolves these parameters and provides the corresponding dependencies.

**Example:**

```go
// UserController depends on UserService.
type UserController struct {
	service UserService
}

// Dependencies are passed in through constructor parameters.
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// Register the constructor. The container automatically analyzes parameters and injects corresponding dependencies.
	gs.Provide(NewUserController)
}
```

Go-Spring uses standard Go constructors for dependency injection, without additional framework-level conventions or special syntax markers.

#### Which Injection Method Should You Choose?

From the perspective of decoupling, constructor injection is recommended in most scenarios,
because it explicitly declares dependencies at object creation time, makes component boundaries clearer,
and better ensures dependency immutability and testability.

For scenarios where dependencies are simple or convenience is prioritized, struct field injection can also be used
to balance clarity and development efficiency without overly restricting its use.

### Injection Targets

Whether you use struct field injection or constructor parameter injection, Go-Spring supports injecting Beans into many kinds of targets.  
The most common approach is injecting a single Bean, which means matching a unique Bean instance by type or name.  
It also supports injecting multiple Beans. In this case, the container automatically collects all qualifying Beans and injects them into a slice or map.

#### Injecting a Single Bean

This is the most basic and common usage: inject the **unique** Bean that meets the criteria.
Most business dependencies belong to this scenario.

**Inject by type**

You can add the `autowire` or `inject` tag to a struct field,
and the container automatically injects the Bean matched by type into that field.

```go
type Service struct {
	// Automatically match by type and inject the unique UserRepository interface.
	Repo UserRepository `autowire:""`
}
```

When registering a constructor Bean, you can also explicitly specify the Bean used for injection through `TagArg`.
The container matches according to the content of `TagArg` and injects the corresponding Bean into the constructor parameter.

```go
// UserController needs a UserService injected.
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// Automatically match by type. The content of TagArg can be omitted here.
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

**Inject by name**

When there is only one candidate Bean, you can inject directly by type without specifying a name.
But when there are multiple candidate Beans, you need to distinguish them by name so the specific injection target is clear.

```go
func init() {
	// Register two named Beans: master and slave.
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	// Inject the Bean named "slave" into a struct field.
	ds DataSource `autowire:"slave"`
}
```

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	// Inject the Bean named "slave" into a constructor parameter.
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

**Nullable injection**

By default, when the container cannot find a matching Bean, it reports an error and terminates startup.
If you want to avoid errors when no Bean is found and instead inject the corresponding zero value, you can use the `?` marker to declare the dependency as nullable.

```go
type Service struct {
	// Nullable injection. If no matching Bean is found, no error is reported and the zero value is kept.
	OptionalDep Dep `autowire:"?"`

	// Specify both a name and the nullable option.
	NamedOptional Dep `autowire:"my-name?"`
}
```

```go
// UserController needs a UserService injected.
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// Use nullable injection for the constructor parameter. If the Bean does not exist, inject the zero value.
	gs.Provide(NewUserController, gs.TagArg("?"))

	// Specify both a name and the nullable option.
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
}
```

#### Injecting Multiple Beans

When you need to obtain **multiple qualifying Beans**,
you can declare the dependency as `[]T` (slice) or `map[string]T` (map).
The container then automatically collects all same-type Beans that meet the criteria and injects them into the corresponding collection.

##### Slice Collection `[]T`

You can use a slice in a struct field to collect multiple Beans,
or use a slice in a constructor parameter to collect multiple Beans.

```go
type Application struct {
	// Collect all Plugin implementations into a slice.
	plugins []Plugin `autowire:""`
}

func init() {
	// Register multiple implementations as Beans of type Plugin.
	gs.Provide(NewPluginA).Export(gs.As[Plugin]())
	gs.Provide(NewPluginB).Export(gs.As[Plugin]())
	gs.Provide(NewPluginC).Export(gs.As[Plugin]())
}
```

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{plugins: plugins}
}

func init() {
	// Inject all Plugins into the slice. The TagArg parameter is omitted here.
	gs.Provide(NewApplication)
}
```

When no tag content is specified, the container sorts the elements in the slice by **lexicographic order of Bean names**,
ensuring that the collected result is deterministic every time and therefore behavior is consistent.

If you need to **control the order precisely**, you can explicitly specify the ordering rule in the tag through a list of Bean names.

```go
type Chain struct {
	// Order: auth(nullable) -> tracing -> recovery
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

```go
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}

func init() {
	// Order: auth(nullable) -> tracing -> recovery
	gs.Provide(NewChain, gs.TagArg("auth?,tracing,recovery"))
}
```

You can use nullable syntax `name?` for each Bean in the list, meaning that the corresponding Bean is automatically skipped if it does not exist.

In addition, you can use the wildcard `*` to include **all remaining Beans that are not explicitly listed**.
Note that the wildcard `*` can appear only once in the same expression.

When the wildcard `*` is used, Beans are collected in this order:

1. **Explicit Beans before `*`** are ordered as declared in the tag.
2. **Remaining Beans matched by `*`** are ordered lexicographically by name.
3. **Explicit Beans after `*`** are ordered as declared in the tag.

For example, `autowire:"a,*,c?,b"` means:
collect `a` first, then collect all other Beans except `c` and `b` (sorted by name),
then `c` (if it exists), and finally `b`. `c` is nullable and is automatically skipped if it does not exist.

##### Map Collection `map[string]T`

Besides using slices to collect multiple Beans, you can also use `map[string]T`.
The tag syntax is basically the same, but the result of `map[string]T` does not guarantee order because maps are unordered.

In this form, the key of `map[string]T` is the Bean name, and the value is the corresponding Bean instance.

```go
type Router struct {
	// name -> Handler mapping. The key is the Bean name.
	Handlers map[string]Handler `autowire:""`
}
```

```go
// Inject all Handlers into a map.
func NewRouter(handlers map[string]Handler) *Router {
	return &Router{handlers: handlers}
}

func init() {
	gs.Provide(NewRouter)
}
```

For map collection, you can also use nullable syntax `name?`, meaning that the corresponding Bean is automatically skipped if it does not exist.
However, although wildcard `*` is syntactically available, it is relatively less meaningful in map scenarios because maps themselves do not guarantee order,
so using `*` cannot reflect control over collection order.

```go
type Service struct {
	// Include only handlers with specified names.
	Handlers map[string]Handler `autowire:"auth,user?,order"`
}
```

```go
func NewService(handlers map[string]Handler) *Service {
	return &Service{Handlers: handlers}
}

func init() {
	gs.Provide(NewService, gs.TagArg("auth,user?,order"))
}
```

#### Injecting Through Configuration Items

Usually, the Bean name to use can be determined at injection time.
But in some scenarios, you may need to dynamically decide which Bean or Beans to inject according to configuration items.
Go-Spring supports this usage: write the Bean name as a `${...}` configuration item expression,
and the container parses the corresponding Bean name or list from configuration at runtime and completes injection.

This approach applies to all injection forms:
whether struct field injection or constructor parameter injection,
whether injecting a single Bean or collecting a slice, map, or other collection,
you can dynamically specify Beans through `${...}` expressions.

```go
type Service struct {
	// Read the Bean name from the configuration item "storage.provider".
	// This allows implementations to be switched dynamically through configuration without changing code.
	Storage Storage `autowire:"${storage.provider}"`
}
```

```go
func NewService(storage Storage) *MyService {
	return &MyService{storage: storage}
}

func init() {
	// Read the Bean name from the configuration item "storage.provider".
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

```go
type Chain struct {
	// Read the filter list from the configuration item "http.filters".
	// This allows implementations to be switched dynamically through configuration without changing code.
	Filters []Filter `autowire:"${http.filters}"`
}
```

```go
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}

func init() {
	// Read the filter list from the configuration item "http.filters".
	gs.Provide(NewChain, gs.TagArg("${http.filters}"))
}
```

The `${...}` syntax itself also supports specifying default values. Because this is rarely used in Bean name selection, it is not expanded here.

### Lazy Injection

Lazy injection is mainly used to solve circular dependency problems in certain scenarios, and it only applies to struct field injection.
To use it, add the `,lazy` marker in the tag.

```go
type Service struct {
	// Force this field to be injected lazily, after all non-lazy injections are completed.
	Dep Dependency `autowire:",lazy"`
}
```

Note that fields marked as `lazy` are processed together after all non-lazy injections are complete.
Because this is a separate phase, these fields remain empty during injection in the previous phase. Remember not to use them then.

## Bean Types

When registering Beans, Go-Spring supports three forms of parameters:

* **Struct pointer**: Hand a pre-created object directly to the container for management. This is the simplest usage.
* **Constructor function**: The container calls the constructor at startup to create the Bean. This is the recommended approach.
* **Function pointer**: Register the function itself as a Bean, supporting function-style use cases.

### Struct Pointer

This is the simplest registration method: directly pass in an already-created struct pointer.
The object can be a temporarily created instance or a globally created object reused elsewhere.

```go
// MyService is your business struct.
type MyService struct {
	// ...
}

func init() {
	// Create an object pointer directly with new and register it with the container.
	gs.Provide(new(MyService))
}
```

When a globally created and reused object is registered with the container, the container still manages it,
including calling initialization and destruction methods at appropriate lifecycle stages.
This capability is especially useful when Go-Spring integrates with other frameworks or existing systems.

### Constructor Function

Creating Beans through constructor functions is the approach recommended by Go-Spring.
In this mode, all dependencies are passed in through constructor parameters, and the container is responsible for resolving and providing them.

```go
type MyService struct {
	dep Dep
}

// Constructor parameters receive dependencies and return an instance.
func NewMyService(dep Dep) *MyService {
	return &MyService{dep: dep}
}

func init() {
	gs.Provide(NewMyService)
}
```

The constructor form above is `func(...) T`, which directly returns a Bean and is suitable when creation is certain not to fail.

But when creating complex objects, initialization may fail, such as configuration validation failure, file open failure, or database connection failure.
In this case, the constructor can return an `error` to express failure, using the `func(...) (T, error)` form.
The container automatically recognizes this pattern and terminates startup if creation fails.

```go
// The constructor returns error to indicate whether Bean creation succeeded.
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}

func init() {
	gs.Provide(NewMyService)
}
```

#### Argument Binding

In the previous examples, we have shown some uses of `TagArg`. This section further explains the implementation principle behind it.

Because Go only supports tags on struct fields and cannot directly declare tags on constructor parameters,
Go-Spring adopts an alternative solution:
during registration, it explicitly provides binding information for constructor parameters that need special handling through `Arg` parameters.

This mechanism, which establishes a mapping between constructor parameters and specific dependencies during registration, is called **argument binding (Arg binding)**.

Go-Spring provides several `Arg` implementations for different parameter binding scenarios:

* `TagArg`: binds Bean dependencies or configuration properties.
* `ValueArg`: binds fixed values.
* `BindArg`: binds constructor parameters in the Option pattern.
* `IndexArg`: binds by parameter index position.

##### Injecting Beans

When a constructor Bean depends on other Beans, you can explicitly declare the dependency relationship through `gs.TagArg`.

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// An empty string parameter for TagArg means matching only by type, without a name qualifier.
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

The string parameter of `TagArg` is semantically equivalent to the `autowire` tag on struct fields.

In addition, in the example above, because there is only one parameter and matching by type is sufficient without a name qualifier,
you can omit the `TagArg` parameter.

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// Automatically match by type. TagArg can be omitted here.
	gs.Provide(NewUserController)
}
```

Because the previous sections have already shown many uses of `TagArg`,
such as injecting Beans by name, dynamically specifying Bean names through configuration items, and collecting slices and maps,
this section does not repeat them. For details, see [**Bean Injection**](#bean-injection).

##### Injecting Configuration Items

`TagArg` can be used not only to bind Bean dependency injection, but also to bind configuration properties.
Go-Spring supports reading values from the configuration system, automatically converting them to the corresponding type, and injecting them into constructor parameters.

```go
type RedisClient struct {
	host string
	port int
}

// Constructor parameters directly inject configuration values.
func NewRedisClient(host string, port int) *RedisClient {
	return &RedisClient{host: host, port: port}
}

func init() {
	// Use TagArg to specify configuration item paths.
	gs.Provide(NewRedisClient,
		gs.TagArg("${redis.host:=localhost}"), // Read host address from configuration item "redis.host".
		gs.TagArg("${redis.port:=6379}"), // Read port from configuration item "redis.port".
	)
}
```

You can complete injection of `RedisClient` with the following YAML configuration:

```yaml
redis:
  host: localhost
  port: 6379
```

You can also let the container bind an entire configuration object directly to a struct parameter.

```go
// RedisConfig defines the Redis configuration struct.
type RedisConfig struct {
	Host     string        `value:"${host:=localhost}"`
	Port     int           `value:"${port:=6379}"`
	Password string        `value:"${password:=}"`
	DB       int           `value:"${db:=0}"`
	Timeout  time.Duration `value:"${timeout:=5s}"`
}

// Directly inject the bound RedisConfig object.
func NewRedisClient(cfg RedisConfig) *RedisClient {
	return &RedisClient{
		host: cfg.Host,
		port: cfg.Port,
		// ...
	}
}

func init() {
	// The prefix "redis." is automatically added to all fields.
	// So cfg.Host corresponds to configuration key "redis.host".
	gs.Provide(NewRedisClient, gs.TagArg("redis"))
}
```

This approach gathers a group of related configuration items for centralized management, which is clearer and cleaner than injecting many scattered parameters.

`TagArg` also supports richer configuration binding capabilities, such as map and list injection, configuration references, type converters, and nested configuration.
For more details, see [01-configuration.md](/en/docs/2.guides/01-configuration.md).

##### Injecting Fixed Values

If the value of a parameter is already determined when registering the Bean and does not need to be obtained from the container or read from the configuration system,
you can use `ValueArg` to bind a fixed value.

```go
type RedisClient struct {
	db int
}

func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	// Bind the db parameter to the fixed value 0.
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

##### Option Binding

**Functional Options** is a very popular programming pattern in Go,
mainly used to handle **optional parameters** in constructors.

For example, a service may contain multiple configuration items, but most of them have reasonable default values, and callers usually only need to modify a few configuration items.
In this case, the Option pattern is clearer and more elegant than defining multiple constructors or passing a long list of zero-value parameters.

Here is a typical Option pattern definition:

```go
// Option defines the function type used to modify Server configuration.
type Option func(*Server)

// WithPort returns an Option for setting the port.
func WithPort(port int) Option {
	return func(s *Server) {
		s.port = port
	}
}

// WithTimeout returns an Option for setting the timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.timeout = timeout
	}
}

type Server struct {
	port    int
	timeout time.Duration
}

// NewServer accepts a variable number of Options to configure Server.
func NewServer(opts ...Option) *Server {
	s := &Server{
		port:    8080,             // Default port.
		timeout: 30 * time.Second, // Default timeout.
	}

	// Apply all Options in order.
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

The code above shows the standard Functional Options pattern:

1. Define the `Option` function type, which receives `*Server` and modifies its configuration.
2. Create specific Options through `WithXXX` functions and set corresponding configuration items.
3. `NewServer` accepts the variadic parameter `opts ...Option` and applies these Options in order after creating the instance.

The question is: **if creating the Options themselves depends on configuration or other Beans, how should this be handled?**
For example, the port needed by `WithPort` comes from a configuration file instead of being hard-coded.

In this scenario, Go-Spring's `BindArg` solves the problem well:
it allows the container to manage the creation process of each Option, automatically injecting required dependencies or configuration when creating an Option,
and finally passing the generated Options into the `NewServer` constructor.

```go
func init() {
	// Use BindArg to provide argument bindings for each Option.
	// WithPort needs to read the port from configuration and is bound with TagArg.
	// WithTimeout uses the fixed timeout 60 seconds and is bound with ValueArg.
	gs.Provide(NewServer,
		gs.BindArg(WithPort, gs.TagArg("${server.port:=8080}")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

In addition, `BindArg` supports conditional control:
when a condition is met, the corresponding Option is created and passed to the constructor;
if the condition is not met, that position is skipped directly and no argument is passed.

```go
func init() {
	gs.Provide(
		NewServer,
		// Add this Option only when server.port is configured.
		gs.BindArg(WithPort, gs.TagArg("${server.port}")).
			Condition(gs.OnProperty("server.port")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

In the example above, only when the configuration system explicitly sets `server.port` will the `WithPort` Option be created and injected into the constructor;
if it is not configured, that Option is skipped and the default value defined in `NewServer` is used directly.

This mechanism is very useful in real applications. It can selectively enable or disable certain features according to configuration conditions, enabling more flexible assembly.

#### Parameter Order

By default, argument bindings in `gs.Provide()` are **matched to constructor parameters in order**:
the first binding corresponds to the first parameter, the second binding corresponds to the second parameter, and so on.

However, when a constructor has many parameters and only a few need explicit binding while the rest can be inferred by type by the container,
you can use `IndexArg` to explicitly specify binding positions without passing bindings one by one in parameter order.

```go
// The constructor has three parameters: a, b, c.
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	// Bind only the third parameter (index=2, counted from 0) to a fixed value.
	// a and b will be inferred and injected by the container by type.
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

In this way, without changing the constructor parameter order, you can precisely bind key parameters
and let the container infer the remaining parameters automatically, making the overall configuration more flexible and easier to maintain.

### Function Pointer

Besides struct pointers and constructors, Go-Spring also supports directly registering **function pointers as Beans**,
which is suitable for scenarios where the function itself is an injectable component.

Because Go's type system cannot clearly distinguish function types from constructors,
and the container needs to explicitly identify whether the function is being registered as the Bean itself,
you need to wrap the function with `reflect.ValueOf` when registering the Bean.

```go
import "reflect"

// PasswordChecker defines the type of a password validation function.
type PasswordChecker func(username, password string) bool

// Authenticator needs a password validation function injected.
type Authenticator struct {
	checker PasswordChecker `autowire:""`
}

func NewAuthenticator(checker PasswordChecker) *Authenticator {
	return &Authenticator{checker: checker}
}

// Provide a concrete password validation function.
func BcryptPasswordChecker(username, password string) bool {
	// ... Concrete validation logic.
	return true
}

func init() {
	// Wrap the function pointer with reflect.ValueOf() and register it directly as a Bean.
	// This allows any component that needs PasswordChecker to inject this function.
	gs.Provide(reflect.ValueOf(BcryptPasswordChecker))

	// Register Authenticator, which automatically injects the password validation function above.
	gs.Provide(NewAuthenticator)
}
```

## Bean Configuration

When registering Beans, we usually also need some additional configuration,
such as customizing names, specifying initialization methods, exporting interfaces, attaching conditions, or declaring dependencies.

Go-Spring provides chain calls so these Bean configurations can be completed more conveniently.

### Setting the Bean Name

To uniquely identify each Bean in the container, Go-Spring uses the combination of **type + name** to generate a unique identifier.

If no name is explicitly specified when registering a Bean, Go-Spring automatically uses the short name of the type as the default name. For example:

* For the struct pointer `*UserService`, the default name is `"UserService"`.
* For the interface `UserService`, the default name is `"UserService"`.

When the same type needs to register multiple different instances, such as master and slave data sources,
you can explicitly specify a name for each Bean through the `.Name()` method to distinguish them.

```go
func init() {
	// Register two Beans of the same DataSource type with different names.
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

After setting names, you can explicitly specify which instance to use by that name.

```go
type UserRepo struct {
	// Struct field injection specifies the Bean name in the autowire tag.
	ds *DataSource `autowire:"slave"`
}

func NewUserRepo(ds *DataSource) *UserRepo {
	return &UserRepo{ds: ds}
}

func init() {
	// Constructor parameter injection uses TagArg to specify the Bean name.
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

### Setting Lifecycle Callbacks

Sometimes, after a Bean is created and all dependency injection is completed, we need to execute custom initialization logic.
Similarly, when the container exits, resources need to be released gracefully.

To solve such problems, Go-Spring provides a **lifecycle callback mechanism**.

An **initialization callback** occurs after Bean creation and dependency injection are complete. It is used to execute custom initialization logic,
such as establishing database connections, loading cache data into memory, or validating configuration correctness.

A **destruction callback** occurs during container exit and is used for graceful resource release,
such as closing database connections, persisting in-memory state, or stopping background tasks.

Go-Spring supports two ways to configure lifecycle callbacks: explicitly specifying a function pointer, or declaring a method name.

#### Method 1: Set Through Function Pointers

You can directly pass an independent function pointer as a lifecycle callback. The function needs to receive the Bean instance as a parameter.
It can have no return value, or it can return only `error`.

The function signature rules for `init` and `destroy` are exactly the same.

```go
type MyService struct {
	client *redis.Client
}

func NewMyService() *MyService {
	return &MyService{}
}

// Initialization function that receives the bean as a parameter.
func InitMyService(s *MyService) error {
	s.client = redis.NewClient( /* ... */ )
	// Test the connection.
	if err := s.client.Ping().Err(); err != nil {
		return err // Initialization failed; the container terminates startup.
	}
	return nil
}

// Destruction function that receives the bean as a parameter.
func DestroyMyService(s *MyService) error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		Init(InitMyService).      // Set the initialization function.
		Destroy(DestroyMyService) // Set the destruction function.
}
```

If the `init` callback returns an error, the container terminates startup, indicating initialization failure.
If the `destroy` callback returns an error, the container logs the error but does not block container exit.

#### Method 2: Specify by Method Name

If the struct itself already defines initialization and destruction methods, and their signatures meet the requirements,
you can configure lifecycle callbacks by specifying method names.

```go
type MyService struct {
	client *redis.Client
}

func NewMyService() *MyService {
	return &MyService{}
}

// Init is an initialization method called after dependency injection is complete.
func (s *MyService) Init() error {
	s.client = redis.NewClient( /* ... */ )

	// Test the connection.
	if err := s.client.Ping().Err(); err != nil {
		return err // Initialization failed; the container terminates startup.
	}
	return nil
}

// Destroy is a destruction method called when the container exits.
func (s *MyService) Destroy() error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		InitMethod("Init").      // Set the initialization method.
		DestroyMethod("Destroy") // Set the destruction method.
}
```

### Exporting as an Interface

In Go programs, interfaces are used frequently. So in Go-Spring, how do you register and use interface types?

Go-Spring's approach is: when registering a Bean, you need to explicitly specify the interface type that the Bean should export.
This is because in Go, a struct may implement multiple interfaces, or may even unintentionally implement certain interfaces.
Therefore, to avoid uncertainty caused by automatic inference, the externally exposed interface types must be explicitly declared.

```go
// Define the interface.
type UserService interface {
	Get(id int) (*User, error)
}

// Concrete implementation.
type UserServiceImpl struct {
	// ...
}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func (s *UserServiceImpl) Get(id int) (*User, error) {
	return &User{ID: id}, nil
}

func init() {
	// Export UserServiceImpl as the UserService interface for dependents to inject by interface.
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

In the example above, two Beans exist in the container at the same time: one Bean of the original type and another Bean exported as an interface.
Both forms can be injected. Which one is used depends on the type declaration at the injection point.

Of course, you can also return the interface type directly from the constructor, so explicit interface export is not needed.

```go
// Define the interface.
type UserService interface {
	Get(id int) (*User, error)
}

// Concrete implementation.
type userServiceImpl struct {
	// ...
}

func NewUserService() UserService {
	return &userServiceImpl{}
}

func (s *userServiceImpl) Get(id int) (*User, error) {
	return &User{ID: id}, nil
}

func init() {
	// The constructor directly returns an interface type, so the container registers by interface type and explicit export is unnecessary.
	gs.Provide(NewUserService)
}
```

### Attaching Activation Conditions

Sometimes, you need to register a Bean only under specific conditions, such as environment variables or configuration files.
In this case, you can add conditions to a Bean through the `.Condition()` method so that it takes effect only when the conditions are met.

```go
func init() {
	// Register this Bean only under the dev profile.
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active"). // Listen to the spring.profiles.active configuration.
			HavingValue("expr:contains($, 'dev')"). // The property value must contain dev.
			MatchIfMissing(),                    // Match by default when the property does not exist.
	)
}
```

Because conditional registration involves many topics, this article introduces it later in the [**Conditional Registration**](#conditional-registration) section.

### Explicit Dependency Declaration

In most cases, Go-Spring automatically infers dependency order through **injection relationships**:
whichever Bean you inject, the container ensures that Bean is initialized first.

But in some cases, two Beans may not have a direct dependency injection relationship while their initialization order still needs to be controlled.
In this case, you can use the `.DependsOn()` method to explicitly declare a dependency and ensure that the depended-on Bean is initialized first.

The purpose of `.DependsOn()` is to tell the container:
**although the current Bean does not directly inject this dependency, it must be created first in the initialization order**.

```go
type A struct {
	// ...
}

type B struct {
	// ...
}

func init() {
	// Declare that B depends on A, ensuring A is initialized before B.
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

In the example above, although `B` does not directly depend on `A`, the `.DependsOn()` method ensures that `A` is initialized before `B`.

If both indirectly related Beans define destruction methods, the container also executes destruction in the reverse order when exiting:
it destroys the dependent Bean first, then the depended-on Bean.

### Marking as a Root Bean

Go-Spring uses an **on-demand creation** mechanism. By default, only Beans marked as root are triggered as dependency injection entry points.

However, when integrating with other frameworks, sometimes you do not need to use Go-Spring's built-in Runner or Server (which are built-in root Beans),
and instead only want to use it as an independent Bean container.

In this case, if no root Bean is explicitly specified, the container lacks an initialization entry point and cannot automatically trigger instantiation and dependency injection for user-registered Beans.

In this situation, you can explicitly mark a Bean as a root Bean through the `app.Root()` method.
Then the Go-Spring container uses that Bean as the starting point and recursively completes initialization and injection for the Bean itself and its dependency Beans along dependency relationships.

```go
func main() {
	bootstrap := &Bootstrap{}

	// ...
	// Use the bootstrap object in the code in between.
	// ...

	gs.Configure(func(app gs.App) {
		// Explicitly register the existing Bootstrap instance as a Root Bean.
		// The container uses it as the entry point, ensuring this object is definitely created and included in the dependency system.
		app.Root(bootstrap)
	}).Run()
}
```

In the example above, `bootstrap` is marked as a root Bean.
Even if it is created outside the IoC container, it is included in container management and used as the initialization entry point.

## Bean Registration

Go-Spring provides multiple Bean registration APIs to meet different use cases.

### Registering Through `gs.Provide()`

You can call `gs.Provide()` in a package's `init()` function to register Beans. This is the most basic and commonly used approach.
However, note that this method must be called before the application starts; otherwise, it directly `panic`s.

```go
func init() {
	gs.Provide(NewUserService)
}
```

`gs.Provide()` records Beans in the global registry and merges them uniformly when the application starts.

For most business components, this is the recommended registration method.

### Registering Through `gs.Module()`

`gs.Provide()` can register only one Bean at a time, but in real scenarios, you often need to register Beans in batches.
For this, Go-Spring provides `gs.Module()` to organize and register multiple Beans uniformly.

`gs.Module()` also supports **conditional registration**, allowing the whole module to be enabled or disabled according to configuration item conditions.

Essentially, a Module is a group of Bean registration units with conditional control capabilities. It is very suitable for enabling feature modules on demand,
so it is a perfect abstraction for the Starter mechanism. Third-party integration packages usually expose capabilities through Modules.

In implementation, `gs.Module()` provides `gs.BeanProvider` through a callback function.
Developers can register Beans through its `Provide(...)` method, whose usage is exactly the same as `gs.Provide()`.

For example, in the following module, Redis-related Beans are registered only when the configuration item `enable.redis=true`;
otherwise, the whole Redis module does not take effect.

```go
func RedisModule(r gs.BeanProvider, p flatten.Storage) error {
	var m map[string]RedisConfig

	// Bind Redis instance configurations (name -> config) from configuration.
	if err := conf.Bind(p, &m); err != nil {
		return err
	}

	// Register an independent Redis Client Bean for each Redis configuration.
	for name, config := range m {
		r.Provide(NewRedisClient, gs.ValueArg(config)).Name(name)
	}
	return nil
}

func init() {
	gs.Module(
		// Enable this module when enable.redis=true; otherwise the module does not take effect.
		gs.OnProperty("enable.redis").HavingValue("true"),
		RedisModule,
	)
}
```

### Registering Through `gs.Group()`

`gs.Group()` is a special wrapper around `gs.Module()` used to create Beans of the same type in batches.
It uses configuration keys as Bean names and is very suitable for generating multiple instances from a configuration dictionary.

When you need to create multiple Beans of the same type based on configuration,
such as multiple HTTP clients, multiple data sources, or multiple cache clients where each instance has independent configuration parameters,
using `gs.Group()` can significantly reduce boilerplate code.

```go
// Define the HTTP client configuration struct.
type HTTPClientConfig struct {
	BaseURL string        `value:"${baseURL}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}

// Create an HTTP client based on configuration.
func NewHTTPClient(c HTTPClientConfig) (*http.Client, error) {
	return &http.Client{Timeout: c.Timeout}, nil
}

func init() {
	// Read a map from configuration "http.clients".
	// The key is used as the Bean name, and the value is used as configuration parameters.
	// Each entry corresponds to calling NewHTTPClient to create an independent Bean.
	gs.Group("${http.clients}", NewHTTPClient, nil)
}
```

The corresponding YAML configuration is as follows:

```yaml
http:
  clients:
    serviceA:  # Used as the Bean name.
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:  # Used as the Bean name.
      baseURL: "http://b.example.com"
      timeout: 60s
```

In the example above, after `gs.Group()` is processed, the container generates two `*http.Client` Beans,
named `serviceA` and `serviceB`, which can be injected by name in services.

```go
type MyService struct {
	ClientA *http.Client `autowire:"serviceA"`
	ClientB *http.Client `autowire:"serviceB"`
}
```

If resources need to be released, you can also provide a destruction function, which takes effect for each Bean instance separately.

```go
func init() {
	gs.Group("${http.clients}",
		NewHTTPClient,
		// During the destruction phase, the container calls the destruction function for each instance separately.
		func(c *http.Client) error { return c.Close() },
	)
}
```

### Registering Through `Configuration`

The `Configuration` pattern allows a **configuration class (parent Bean)** to export multiple **child Beans**,
which is used to centrally organize and manage Beans in the same functional domain.
Exported child Beans can directly reuse configuration parameters already injected into the configuration class.

For example, a database configuration class can uniformly export related Beans such as data sources, Repositories, and Mappers.

```go
// DatabaseConfiguration is a configuration class (parent Bean) and itself supports dependency injection.
type DatabaseConfiguration struct {
	MaxOpenConns int `value:"${db.max-open-conns:=10}"`
}

// Export a DataSource Bean.
// It can use parameters already injected into the configuration class (parent Bean).
func (c *DatabaseConfiguration) NewDataSource() *DataSource {
	return NewDataSource(c.MaxOpenConns)
}

// Export a UserRepository Bean.
// Method parameters participate in dependency injection (type-based matching only).
func (c *DatabaseConfiguration) NewUserRepository(ds *DataSource) *UserRepository {
	return NewUserRepository(ds)
}

// Export an OrderRepository Bean.
// Method parameters participate in dependency injection (type-based matching only).
func (c *DatabaseConfiguration) NewOrderRepository(ds *DataSource) *OrderRepository {
	return NewOrderRepository(ds)
}

func init() {
	// Register the configuration class.
	// After Configuration is enabled, the container automatically scans methods on this object
	// and registers return values of methods that meet the rules as Beans.
	gs.Provide(new(DatabaseConfiguration)).Configuration()
}
```

In the example above, a `DatabaseConfiguration` is defined as a configuration class (parent Bean)
for centralized management of database-related configuration parameters.
Then methods separately export Beans such as `DataSource`, `UserRepository`, and `OrderRepository`.
These Beans can use parameters already injected into the configuration class, and can also receive other Beans as dependencies (only type-based matching is supported).

You may wonder how the `Configuration` pattern works:

1. First, register the configuration class as a normal Bean in the container, and mark it as a configuration class through `.Configuration()`.
2. During the parsing phase, the container traverses all registered Beans and filters out Beans for which the `Configuration` pattern is enabled.
3. It then scans all public methods of the configuration class and filters them according to `Includes` and `Excludes` rules.
4. For methods that meet the criteria, the container automatically registers their return values as independent child Beans and adds them to the Bean list.
5. Finally, all Beans, regardless of source, are included in unified lifecycle and rule management.

In practice, these qualifying methods are treated as constructors, so they can return either only one object or an additional `error`.

Here are examples of qualifying methods:

```go
// Valid: returns a pointer, no error.
func (c *Config) NewDataSource() *DataSource

// Valid: returns a pointer + error.
func (c *Config) NewDataSource() (*DataSource, error)

// Invalid: wrong number of return values.
func (c *Config) NewDataSource() (*DataSource, string, error)
```

If you need custom include and exclude rules, you can control them through the `Includes` and `Excludes` parameters.
By default, `Includes` only matches methods whose names match `New.*` (that is, methods starting with `New`),
and `Excludes` does not exclude any methods.

Example of custom include and exclude rules:

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"}, // Include methods matching these regular expressions.
			Excludes: []string{".*Internal$"},       // Exclude methods matching these regular expressions.
		})
}
```

### Registering Through `app.Provide()`

Besides the registration methods above, you can also register Beans through the callback function provided by `gs.Configure()`.
This method provides a `gs.App` object in the callback, and developers can register Beans through its `Provide` method.

```go
func main() {
	gs.Configure(func(app gs.App) {
		// Register Beans or set application-level properties in the callback.
		// These contents take effect only in the current application instance.
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

This Bean registration method is usually used in unit tests to isolate data between tests.

## Conditional Registration

Sometimes you want certain Beans to take effect only when specific conditions are met. This can be implemented through the **Condition** mechanism.

Go-Spring provides rich condition implementations and composition tools out of the box.
You can judge based on configuration properties, judge based on whether Beans exist,
and combine multiple conditions through `And` / `Or` / `Not` / `None` to implement more complex logic.

As mentioned earlier, when registering a Bean, you can bind conditions to the Bean through the `.Condition()` method.

```go
gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
```

### Common Conditions

Go-Spring provides several common condition types.

#### Property Conditions

`OnProperty` is one of the most commonly used condition types.
It can judge whether a configuration property exists,
judge whether a configuration property's value equals a specific value,
and support more flexible condition checks through `expr` expressions.

```go
// The condition is met when the configuration item exists (it can be either a leaf node or a path node).
gs.OnProperty("enable.redis")

// The condition is met only when the configuration item equals the specified value.
gs.OnProperty("env").HavingValue("prod")

// The condition is also met when the configuration item does not exist (MatchIfMissing).
gs.OnProperty("optional.feature").MatchIfMissing()
```

`OnProperty` also supports expression judgment. Add the `expr:` prefix before the value
to implement more complex validation logic beyond existence checks.

```go
// Match cases where the port is greater than 8080.
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

##### Expression Syntax

`OnProperty` uses the [expr-lang/expr](https://github.com/expr-lang/expr) engine for evaluation.
The syntax is concise and intuitive:

* **`$`**: Represents the value of the current configuration property. All expressions compare and judge around `$`.
* **Comparison operators**: Supports common comparison operations such as `>`, `<`, `>=`, `<=`, `==`, and `!=`.
* **Logical operators**: Supports `&&`, `||`, and `!` for logical composition.
* **String operations**: Supports string operation methods such as `contains`, `hasPrefix`, and `hasSuffix`.

Common expression examples:

```go
// The port is greater than 1024 and less than 65535.
gs.OnProperty("server.port").HavingValue("expr: $ > 1024 && $ < 65535")

// The environment is not production.
gs.OnProperty("app.env").HavingValue("expr: $ != 'prod'")

// The configuration item starts with "http://".
gs.OnProperty("app.base-url").HavingValue("expr: startsWith($, 'http://')")

// The configuration item contains the specified keyword.
gs.OnProperty("app.features").HavingValue("expr: contains($, 'debug')")
```

You can register custom functions through `gs.RegisterExpressFunc()` and use them in expressions.

```go
func init() {
	// Register a custom expression function.
	gs.RegisterExpressFunc("isValidPort", func(port int) bool {
		return port > 1024 && port < 65535
	})

	// Use the custom function in a condition expression.
	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").HavingValue("expr:isValidPort($)"),
	)
}
```

#### Bean Existence Conditions

Go-Spring provides several conditions for judging whether specific Beans exist in the container:

```go
// The condition is met when UserService exists in the container (at least one match).
gs.OnBean[*UserService]()

// The condition is met when UserService does not exist in the container.
gs.OnMissingBean[*UserService]()

// The condition is met when exactly one UserService exists in the container.
gs.OnSingleBean[*UserService]()

// Match by name and judge whether the DataSource with the specified name exists.
gs.OnBean[*DataSource]("master")
```

- `OnBean[T]()` means at least one matching Bean exists in the container.
- `OnMissingBean[T]()` means no matching Bean exists in the container.
- `OnSingleBean[T]()` means exactly one matching Bean exists in the container.

When using these three conditions, you can choose to pass a Bean name or not.
If no Bean name is passed, matching is by type only; if a Bean name is passed, matching is by both type and name.

#### Custom Function Conditions

In simple cases, you can use `OnFunc` to wrap a custom function to implement condition judgment.

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	// Custom arbitrary condition judgment logic.
	return myCustomCheck(ctx)
})
```

#### Composite Conditions

Go-Spring provides four condition logic compositions: `And` / `Or` / `Not` / `None`.

```go
// Use gs.And when all conditions must be met.
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))

// Use gs.Or when any condition may be met.
gs.Provide(NewService).Condition(gs.Or(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))

// Use gs.Not when a condition needs to be negated.
gs.Provide(NewFallbackService).Condition(gs.Not(
	gs.OnBean[RealService](),
))

// Use gs.None when none of the conditions should be met.
gs.Provide(NewService).Condition(gs.None(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))
```

- **`gs.And`**: True only when all child conditions are met.
- **`gs.Or`**: True when any child condition is met.
- **`gs.Not`**: Negates the result of the child condition.
- **`gs.None`**: True only when none of the child conditions are met.

These composite conditions can also be nested with each other to implement more complex condition logic.

```go
// Production environment and (A is enabled or B is enabled).
gs.And(
	gs.OnProperty("env").HavingValue("prod"),
	gs.Or(
		gs.OnProperty("enable.a"),
		gs.OnProperty("enable.b"),
	),
)
```

Note that although composite conditions can implement complex logic, overly complex condition expressions are generally not recommended.
If the logic is too complex, first consider whether it can be simplified or refactored.

#### Caching Condition Results with `OnOnce`

Sometimes condition calculation may be complex and needs to be reused in multiple places.
To avoid repeated calculation, you can use `OnOnce` to cache the result, and subsequent judgments directly reuse the cached result.

```go
// The condition is calculated only once; subsequent uses directly reuse the cached result.
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

In most cases, simple conditions do not need result caching.
Use the caching mechanism only when conditions are relatively complex and need to be reused in multiple places.

### Profile Conditions

For scenarios where Beans are selectively enabled by environment (development/test/production),
Go-Spring provides the chained `.OnProfiles()` method, which is more concise and intuitive than explicit condition settings.

```go
func init() {
	// Enable this Bean only in the dev environment.
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

Essentially, Profile conditions are judged based on the `spring.profiles.active` configuration item.
When any of the configured profiles matches one of the currently active profiles, the condition is met.

> `spring.profiles.active` represents the currently active profiles, such as `"dev"`, `"test"`, or `"prod"`.

## Container Internals

This chapter introduces some implementation details of Go-Spring.
Understanding how the container works helps us better understand and use Go-Spring.

### Runtime Flow

The core of Go-Spring is the runtime flow of the IoC container.
The following describes the complete process from container startup to shutdown.

#### Registration Phase

Bean registration is divided into two independent phases: `global registration` and `container registration`.
The fundamental purpose is to support data isolation in unit tests.

##### Global Registration

You call `gs.Provide()` and `gs.Module()` in `init()` to complete Bean registration.
These registration records are saved in the global registry and serve as the primary registration source for all containers.

When creating a container, Go-Spring copies registration records from the global registry to generate an independent container instance,
so containers do not affect each other.

This is also the foundation for data isolation in unit tests:
each test case creates an independent container and builds its own Bean collection based on a copy of the global registry.

##### Container Registration

Container registration is mainly used to supplement Beans in unit testing scenarios.
Go-Spring supports running unit tests without starting the full application.
At this time, you can register required Beans directly into the current container through `app.Provide()`
to complete dependencies in the test environment. These Beans take effect only in the current container.

#### Parsing Phase

After the container starts, the first step is to merge and parse all Beans. This phase executes the following steps in order:

##### Bean Merging

The first step is to merge Beans from all sources together, including:

* Beans registered through `app.Provide()`.
* Beans registered through `gs.Provide()`.
* Beans registered through `gs.Module()` or `gs.Group()`.
* Beans exported through the `Configuration` pattern.

After merging, the container obtains a complete list of Beans to be processed.

##### Condition Pruning

This step traverses all Beans and executes their Condition judgments in order.
Ultimately, Beans that meet conditions are kept, and Beans that do not meet conditions are pruned.
Only retained Beans participate in subsequent creation.

This process implements the ability to "dynamically decide which Beans take effect according to conditions" and is also the basis for multi-environment configuration.

##### Conflict Detection

In Go-Spring, Beans with exactly the same type and name are considered conflicting.
Go-Spring strictly follows the principle of **no overriding** and performs conflict detection at the end of the parsing phase.
Once a conflict is detected, the container directly fails startup and reports an error.

#### Injection Phase

After parsing is complete, the container starts from root beans, recursively creates Bean instances according to dependency relationships, and completes dependency injection.
The core steps of this phase are:

1. First build indexes for all Beans by type and name, for fast dependency matching later.
2. Starting from root beans, recursively create Beans and complete injection in dependency order.
   **During this process, each Bean is handled as follows:**
   - Perform dependency injection immediately after creating the Bean instance.
   - Detect circular dependencies during injection and report an error directly if any exist.
   - Record fields that need lazy injection for later centralized processing.
   - Record dependency relationships between Beans for destroy execution order.
   - Call the Bean's `init` method after injection is complete to finish initialization.
3. After all Beans are processed, centrally process lazy injection fields and complete final dependency binding.
4. Topologically sort dependencies between destroy methods to ensure depended-on Beans are destroyed later.
5. Clean up temporary metadata from the parsing phase and release unnecessary memory resources.

After the injection phase is complete, all Beans have been created, dependency-injected, and initialized, and the container enters the runtime phase.

#### Runtime Phase

Go-Spring does not participate in business logic processing during the runtime phase, and it does not support dynamically obtaining Bean instances at runtime.

This design is intentional.
On one hand, judging from Java Spring's implementation, supporting runtime Bean retrieval significantly increases framework implementation complexity.
On the other hand, Go emphasizes statically explicit dependency relationships. This design avoids uncertainty caused by runtime reflection.
Therefore, Go-Spring deliberately avoids reflective Bean access at runtime in its design.

In other words, Go-Spring adopts a **startup-time model**:
all dependency relationships are injected only during startup, and after injection is complete, the container no longer participates in the runtime process.

This design also brings additional advantages, such as:

* All dependency errors are exposed during startup, avoiding discovery only at runtime.
* It is more suitable for resource-constrained environments, such as embedded systems or lightweight services.
* The container does not need to participate at runtime, making overall execution lighter and more predictable.

#### Shutdown Phase

When the application receives an exit signal such as SIGINT, the container is notified to enter the shutdown phase.
At this time, the container destroys all Beans in reverse dependency order to ensure dependencies are released safely.

During destruction, each Bean's `Destroy` callback method is called in turn to release resources.
The container exits completely only after all callbacks finish.

### Core Design

During implementation, Go-Spring makes a series of key design decisions. These decisions fundamentally shape the overall runtime mechanism of the container.
Deeply understanding these design ideas helps us use Go-Spring more efficiently and flexibly.

#### Interface Separation

No matter how many interfaces a struct implements, Go-Spring treats **interface Beans** and **original Beans** as independent entities.
If you want to inject through an interface, you must explicitly export the corresponding interface by using the `Export` method.

This design is adopted because Go does not have an `implements` keyword. If dependencies were inferred automatically, unexpected behavior could easily occur.
For example, a struct may "happen to" implement an interface, but that may not be our intention, resulting in the wrong dependency being injected.

```go
type Service interface {
	Do()
}

type ServiceImpl struct{}

func (s *ServiceImpl) Do() {
	println("ServiceImpl.Do")
}

func NewServiceImpl() *ServiceImpl {
	return &ServiceImpl{}
}

func init() {
	// Register only *ServiceImpl and do not export the Service interface.
	// Other components cannot inject through the Service interface and can only depend on the concrete type *ServiceImpl.
	gs.Provide(NewServiceImpl)
}

func init() {
	// Correct approach: explicitly export the interface.
	// This allows injection by interface while retaining support for the concrete type.
	gs.Provide(NewServiceImpl).Export(gs.As[Service]())
}
```

#### On-Demand Creation

In large projects, dependency relationships and organization structures are often very complex, making it easy to introduce Beans that are not actually used.
To solve this problem, Go-Spring uses an **on-demand creation (lazy creation)** strategy.

That is, in Go-Spring, only depended-on Beans are created, while Beans that are not depended on are not instantiated,
thereby avoiding unnecessary resource overhead.

But this leads to a key question: from which Beans should the container start analyzing dependency relationships to ensure that truly needed Beans are not missed?

For this purpose, Go-Spring introduces the concept of **root bean**, the starting point of the dependency tree.
The container starts from these root beans and recursively resolves and creates all Beans they depend on.

So which Beans are considered root beans?

- First, Beans that implement the `gs.Runner` or `gs.Server` interfaces are automatically recognized as root beans.
  These Beans play special roles during application startup and are automatically collected by the container.
- Second, Beans explicitly marked through `app.Root()` are also considered root beans.
  These Beans are usually used outside the IoC container, but use the container to complete dependency injection.

Through this mechanism, Go-Spring can ensure dependency completeness while implementing a precise and controllable Bean creation strategy.

#### Circular Dependencies

Go-Spring can provide **limited support** for circular dependencies, depending on the dependency injection method:

- If A depends on B through **field injection**, and B also depends on A through **field injection**, this case is supported.
  The container can first create object instances and then fill properties, completing the dependency loop.

- If A depends on B through **field injection**, and B depends on A through **constructor injection**, this case is **limitedly supported**.
  During dependency injection, A or B may be in a "not fully initialized" state, so use it with caution.

- If A and B **both depend on each other through constructor injection**, this case is **not supported**.
  Because constructors require dependencies to be complete at creation time, A and B cannot both be initialized before each other, making resolution impossible.

#### Destruction Order

Go-Spring strictly follows the **reverse dependency order** principle to manage Bean destruction order: **depended-on Beans are initialized first and destroyed last**.

For example, if A depends on B:

* During initialization: B must be created first before it can be injected into A.
* During destruction: A is destroyed first, releasing its dependency on B, and then B is destroyed.

This "create before use, stop before teardown" order ensures that dependencies are always available throughout destruction,
thereby avoiding problems such as "accessing a destroyed Bean".

## FAQ

### 1. Does it support dynamically obtaining Beans at runtime? Why is there no `getBean()`?

Go-Spring **does not support** dynamically obtaining Beans at runtime, meaning it does not provide APIs such as `getBean()`.
This is an intentional design choice, mainly based on the following considerations:

1. **Little practical need**: In most server-side applications, component dependencies are already determined at startup, and dynamic Bean retrieval is rarely needed at runtime.
2. **Avoid complexity**: All dependencies are resolved during startup, so runtime code does not need to know about the container, making call relationships more direct and clear.
3. **Zero runtime overhead**: After application startup is complete, the container no longer participates in runtime logic, related metadata is cleaned up, and performance is basically the same as handwritten code.
4. **Expose problems early**: Dependency errors are exposed immediately during startup instead of being delayed until runtime.
5. **Explicit dependency declaration**: All dependency relationships are explicitly reflected in code, making them easier to understand and debug than runtime dynamic retrieval.

If there is indeed a need to create instances at runtime, the **factory pattern** is recommended:
register a factory singleton, and let the factory create and manage the required instances.

### 2. Why must interfaces be explicitly exported? Can't they be inferred automatically?

In Go, there is no `implements` keyword; interface implementation is **implicit**.
This means that if dependencies were inferred automatically, a struct that "happens to" implement an interface could be matched incorrectly, introducing hard-to-detect problems.

In addition, in most scenarios, you may not necessarily need to decouple through interfaces.
Go more strongly recommends using concrete types directly within an appropriate scope to keep code concise.

Therefore, Go-Spring requires `Export` to explicitly declare interface implementation relationships, making dependencies more **explicit, controllable, and readable**.

### 3. Are circular dependencies supported?

Go-Spring provides **limited support** for circular dependencies and can automatically handle some common scenarios, such as circular dependencies based on field injection.

However, for some cases, such as circular dependencies between constructors, objects must be complete when created, so the container cannot resolve them and they are not supported.

### 4. How should unit tests be written?

See the independent document [07-testing.md](/en/docs/2.guides/07-testing.md), which systematically explains testing-related content, including:

* Pure unit testing that does not depend on IoC (recommended; the simplest and most direct approach).
* IoC container-based integration testing.
* How to override globally registered default implementations with Mock Beans in tests.
* Usage of Go-Spring's built-in assertion tools and Mock framework.

### 5. Is prototype mode (multiple instances) supported?

Go-Spring core **only supports singleton mode** and does not directly provide prototype scope.
This is a design choice aligned with Go language characteristics:

1. **Little practical need**: Prototype mode has some use cases in the Java ecosystem, but in Go server-side development,
   most components are naturally stateless or reusable, and singletons already cover the vast majority of needs.

2. **Go provides a more natural alternative**: If multiple instances are truly needed, the more Go-like approach is to use the factory pattern
   instead of relying on container scope mechanisms.

A typical approach is to register the factory itself as a singleton and let the factory create new instances:

```go
// The factory itself is a singleton.
type MyServiceFactory struct {
	// The factory can depend on other components in the container.
	config Config `autowire:""`
}

// Create a new instance through a factory method.
func (f *MyServiceFactory) NewMyService(...) *MyService {
	return &MyService{
		// The factory is responsible for initialization.
		config: f.config,
		// ...
	}
}

func init() {
	gs.Provide(new(MyServiceFactory))
}
```

In short: if multiple instances are needed, use the factory pattern directly.
The core responsibility of the IoC container does not need to introduce extra scope complexity.

### 6. How is the performance? Is there runtime reflection overhead?

Go-Spring ensures **zero runtime reflection overhead** by design:

- **Dependency injection runs only once during startup**: All Bean parsing and injection occur during startup.
  After completion, the container releases most metadata and no longer participates in runtime logic.
- **Runtime code calls directly**: After application startup is complete, calls between components are no different from ordinary Go code.
  There is no runtime reflection lookup or dynamic resolution.
- **Reflection is used only during initialization**: The only place reflection is used is during startup for dependency resolution and field injection.
  This process executes only once.

Therefore, after application startup is complete, Go-Spring's runtime performance is basically the same as handwritten Go code, with no need to worry about additional performance loss.

### 7. Why use `init()` for manual registration instead of automatic package scanning?

First, from the perspective of Go itself, there is **no true "automatic package scanning" capability**:

- Go's compilation model determines that only imported packages are compiled into the binary; packages that are not imported will not be loaded even if their code exists.
- Even to implement "automatic registration", it is still necessary in essence to trigger `init()` execution through `import _ xxx`.

Since imports are ultimately required to introduce packages, Go-Spring chooses to explicitly register Beans in `init()`
instead of introducing an additional scanning mechanism. This implementation also has several benefits:

1. **Fully explicit**: Which Beans are registered is clearly visible, avoiding uncertainty caused by "implicit scanning".
2. **Clear and controllable**: No annotations, configuration files, or runtime filesystem scanning are required, matching Go's philosophy that explicit is better than implicit.
3. **Simpler startup**: All registration logic is completed automatically through `init()` during program startup, without extra scanning or parsing phases.

Of course, if you want to reduce manually written registration code, you can generate `Provide` calls at build time using code generation tools, but this does not change the core model.

### 8. How is it different from compile-time DI tools such as Wire?

| Feature | Go-Spring | Wire |
| ------ | ----------- | ------- |
| Processing time | Runtime (startup-time reflection) | Compile-time code generation |
| Conditional registration | Native support | Not supported |
| Dynamic configuration | Native support | Not supported |
| Lifecycle management | Full support | Not supported |
| Startup speed | Fast enough | Faster |

Essentially, Wire is a **compile-time dependency injection tool**. It generates dependency relationships during build time,
so it has stronger static constraints and advantages in startup speed and binary size. It is suitable for scenarios with high requirements for extreme performance and compile-time safety.

Go-Spring is a **runtime (startup-time) dependency injection framework**. Although it uses reflection for dependency resolution,
this happens only during application startup. Once startup is complete, runtime no longer depends on reflection, so there is no continuous performance overhead.

Its advantages are mainly:

* Supports conditional registration, so dependencies can be flexibly enabled or replaced according to configuration.
* Supports dynamic configuration, making it more suitable for complex business environments.
* Provides complete lifecycle management capabilities, including initialization, destruction, and dependency order.
* Better matches the engineering needs of large server-side applications.

Therefore, it can be simply understood as:

* Wire leans toward "**determine everything at compile time**".
* Go-Spring leans toward "**flexibly manage system structure at runtime**".

The two are not directly better or worse; they are design trade-offs for different engineering constraints.

### 9. What is a root bean? When should it be manually marked?

A **root bean** is the starting point of the dependency tree. The container starts from root beans to recursively resolve their dependencies
and ensures that these Beans are definitely created, even if they are not explicitly depended on by other Beans.

The following cases automatically become root beans and require no manual configuration:

* Beans that implement the `gs.Runner` interface.
* Beans that implement the `gs.Server` interface, such as HTTP Server.

These components usually represent application entry points or core runtime units, and the container automatically collects and processes them during startup.

If a component needs to be actively created when the container starts, but it is neither a `Runner` nor a `Server`,
you can **manually mark it as a root bean** through `app.Root()`:

```go
func main() {
	c := &MyComponent{}
	// ...
	gs.Configure(func(app gs.App) {
		app.Root(c)
	}).Run()
}
```
