# Application Startup, Shutdown, and Runtime Mechanisms

When a Go-Spring application starts, it completes configuration loading, log initialization, IoC container startup, Runner execution, and Server startup in sequence.
When the application shuts down, it cancels the root context, stops services, and releases container resources.

Understanding these phases helps us choose the right startup method and makes it easier to troubleshoot startup failures, services that are not ready, or blocked shutdowns.

## Startup Methods

Go-Spring provides two commonly used startup methods:

- `gs.Run()`: standard blocking startup, suitable for standalone services and also the recommended entry point for new projects.
- `gs.RunAsync()`: asynchronous startup, suitable for integration into an existing program where the caller controls when to exit.

### Blocking Startup

`gs.Run()` is the most commonly used startup method.
After it is called, the framework completes application startup and blocks the current goroutine until it receives an exit signal.

```go
package main

import (
    "go-spring.org/spring/gs"
)

func main() {
    // gs.Run() completes the following work:
    //   1. Loads configuration files such as ./conf/app.* and ./conf/app-{profile}.*
    //   2. Initializes the logging system
    //   3. Starts the IoC container and completes Bean creation and dependency injection
    //   4. Starts the built-in HTTP Server (default port 9090)
    //   5. Listens for Ctrl+C and SIGTERM to trigger graceful shutdown
    gs.Run()
}
```

The complete flow of `gs.Run()` is as follows:

```
Print Banner
  -> Load configuration
  -> Initialize logging system
  -> Start IoC container
  -> Execute all Runners
  -> Start all Servers
  -> Listen for exit signals and wait for shutdown
```

This startup method requires little code, provides complete default behavior, and includes signal listening and graceful shutdown.
For most server-side applications, `gs.Run()` is recommended first.

### Non-blocking Startup

When Go-Spring needs to be integrated into an existing system, the blocking `gs.Run()` may not be applicable.
In this case, use `gs.RunAsync()` to start the application in a non-blocking way.

After `gs.RunAsync()` starts successfully, it returns a `stop` function. Calling this function triggers application shutdown.

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "go-spring.org/spring/gs"
)

func main() {
    // Start the application asynchronously without blocking the current goroutine.
    stop, err := gs.RunAsync()
    if err != nil {
        log.Fatal("startup failed:", err)
    }
    defer stop()

    // Existing system logic can continue here, such as starting other services or waiting for external lifecycle events.
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
}
```

`gs.RunAsync()` does not automatically listen for operating system signals.
Call `stop()` before the program exits to ensure Server and container resources are released according to the Go-Spring lifecycle.

## Startup Configuration

Go-Spring provides several startup customization capabilities,
such as disabling the built-in HTTP Server, setting default configuration, registering Root Beans, or customizing the Banner.
These settings need to be completed before the application starts.

### Disable the Built-in HTTP Server

`gs.Web()` is used to control whether the built-in HTTP Server starts.

```go
// Disable the built-in HTTP Server
gs.Web(false).Run()
```

In fact, `gs.Web(false)` is equivalent to setting the following configuration item:

```go
app.Property("spring.http.server.enabled", "false")
```

Using `gs.Web(false)` avoids writing the concrete configuration key directly.

### Set Default Configuration

We can set the application's default configuration through the `app.Property()` method provided by `gs.Configure()`:

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":8080")
    app.Property("env", "production")
})
```

This type of configuration has the lowest priority, only higher than default values in `value` tags.
For actual environments, it is still recommended to put configuration in configuration files, environment variables, or startup arguments.

### Register Container Beans

We can register Beans with the container through the `app.Provide()` method provided by `gs.Configure()`:

```go
gs.Configure(func(app gs.App) {
    app.Provide(&MyService{})
})
```

This writing style is usually used for testing. For ordinary business Beans, it is recommended to register them through the global `gs.Provide()`, with the module declaring them itself.

### Register Root Beans

We can mark a Bean as a Root Bean through the `app.Root()` method provided by `gs.Configure()`:

```go
gs.Configure(func(app gs.App) {
    app.Root(&AppEntry{})
})
```

Go-Spring's IoC container recursively creates the dependency graph starting from Root Beans.
When integrating with legacy projects, if the built-in HTTP Server is disabled and no other `gs.Runner` or `gs.Server` is provided,
then no Bean may be proactively created.
At this point, we can specify an entry Bean through the `app.Root()` method to ensure related dependencies are initialized.

### Custom Banner

`gs.Banner()` is used to customize the Banner printed at startup.

```go
func init() {
    gs.Banner(`
   _____ __  __  _____
  / ____|  \/  |/ ____|
 | |  __| \  / | |  __
 | | |_ | |\/| | | |_ |
 | |__| | |  | | |__| |
  \_____|_|  |_|\_____|
  My Application v1.0
`)
}
```

If a Banner is not needed, set it to an empty string:

```go
gs.Banner("")
```

## Startup Flow

The complete Go-Spring startup flow is as follows:

```
Call Run() / RunAsync()
      |
      v
  Print Banner
      |
      v
  Load configuration
  - Command line > Environment variables > Profile configuration > Base configuration > Property
      |
      v
  Initialize logging system
      |
      v
  Start IoC container
  - Recursively create and inject dependencies starting from Root Beans
  - Collect all Runners and Servers
      |
      v
  Execute all Runners sequentially
  - Any Runner that returns an error terminates startup
      |
      v
  Start all Servers in parallel
  - Wait for all Servers to send Ready signals
      |
      v
  Startup complete
  - Run() listens for signals / RunAsync() returns control
```

### Configuration Loading

Go-Spring supports multiple layers of configuration sources, with priority from high to low as follows:

1. Command-line arguments
2. Environment variables
3. Profile configuration files
4. Base configuration files
5. Configuration items set by `Property()`

Configuration items with the same name are overridden by higher-priority sources, while different configuration items are automatically merged.

#### Command-line Arguments

We can override configuration items at startup through `-Dkey=value`.
When no value is explicitly assigned, the configuration item is treated as `true`.

```bash
go run main.go -Dspring.http.server.addr=:8080
go run main.go -Denv=production -Dlogging.level=error
go run main.go -Ddebug
```

#### Environment Variables

We can also override configuration items through environment variables. This method is often used for containerized deployment.
Environment variables use the `GS_KEY=value` format.

```bash
export GS_SPRING_HTTP_SERVER_ADDR=:8080
export GS_ENV=production

docker run -e GS_ENV=production my-app
```

Environment variables with the `GS_` prefix first have the prefix removed, then underscores `_` are converted to dots `.`, and the result is converted to lowercase.
Therefore, `GS_SPRING_HTTP_SERVER_ADDR` corresponds to the configuration key `spring.http.server.addr`.

Environment variables without the `GS_` prefix keep their original key names and are generally not recommended for Go-Spring application configuration.

#### Configuration Files

Go-Spring's default configuration directory is `./conf`, which can be changed through the `spring.app.config.dir` configuration item.
Go-Spring loads configuration files named in the form `app.*` and `app-{profile}.*` from that directory.
Go-Spring supports configuration files in `.properties`, `.yaml`, `.toml`, and `.json` formats by default.

Because the framework needs to determine the configuration directory before loading configuration files, `spring.app.config.dir` should usually be set through command-line arguments,
environment variables, or `gs.Configure()`.

```yaml
# ./conf/app.yaml
spring:
  http:
    server:
      addr: :9090
env: development
logging:
  level: info
```

Configuration files support Profile overrides. After we specify the active Profile through `spring.profiles.active`,
the framework loads the corresponding `app-{profile}.*` configuration files.

```bash
go run main.go -Dspring.profiles.active=prod
```

We can specify multiple Profiles in `spring.profiles.active`, separated by commas,
for example, `-Dspring.profiles.active=dev,local`.
The framework loads the configuration files corresponding to Profiles in declaration order, and later-loaded Profile configuration can override configuration items with the same name that were loaded earlier.

Configuration files support importing other configuration files with `spring.app.imports`:

```yaml
spring:
  app:
    imports:
      - ./conf/database.yaml
      - ./conf/redis.yaml
env: development
```

Configuration files imported through `spring.app.imports` are at the same priority as the configuration file that declares them.
When imported configuration files contain the same key, the configuration loaded later takes precedence.
In addition, imports currently only support one level; that is, imported files cannot declare new imports, and such declarations will not take effect even if present.

Besides local files, we can also import other forms of configuration through `Provider`.
For details, see [Configuration Sources](/en/docs/2.guides/01-configuration.md#supported-configuration-sources).

#### Code Configuration

We can also set configuration items through the `app.Property()` method provided by `gs.Configure`.

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":9090")
    app.Property("env", "development")
})
```

For more configuration-related content, see [Configuration Management](/en/docs/2.guides/01-configuration.md).

### Initialize Logging

The logging system is initialized before the IoC container. Because the container startup process itself also needs to output logs,
logging cannot depend on Bean creation being completed before becoming ready.

Go-Spring's logging system is designed separately.
For details, see [Logging](04-logging.md).

The logging system uses a k-v format for configuration, so configuration can be read directly from the configuration system
without requiring a separate configuration file.

### Initialize the IoC Container

Before the container starts, Go-Spring first registers some built-in Beans:

- `ContextProvider`: used to obtain the application's root context.
- `PropertiesRefresher`: used to trigger dynamic configuration refresh.

Then the container recursively traverses the dependency graph starting from Root Beans, creates Beans on demand, and completes dependency injection.
After injection is complete, the container collects all Beans that implement the `Runner` and `Server` interfaces for execution in subsequent phases.

If the application does not enable any `gs.Dync[T]` dynamic configuration fields, the container cleans up the configuration cache to save memory.

### Execute Runners

`Runner` is used to execute one-time initialization tasks, such as database migrations, cache warm-up, or base data initialization.
It executes after container initialization and before Server startup, so it can safely use Beans whose injection has already been completed.

All `Runner`s execute synchronously in collection order. Any `Runner` that returns an error causes application startup to fail.
The sequential execution design avoids races between initialization tasks and also makes it easy to express ordering dependencies through collection order.

`Runner` execution should return quickly and is not suitable for long-running background tasks.
If continuous running is required, implement it as a `Server`; otherwise, the application will remain in the startup phase.

### Start Servers

`Server` is used to host long-running services, such as HTTP services, gRPC services, MQ consumers, or task schedulers.
All `Server`s start in parallel in independent goroutines.

Pay attention to the Ready signal mechanism when starting a `Server`.
Each `Server` should trigger the Ready signal only after it has completed listener binding or is capable of serving requests.
The framework waits until all `Server`s have sent Ready signals before continuing the startup completion flow.

This mechanism avoids the situation where the health check interface is already available and traffic has already entered, but another Server later fails to start,
ultimately causing request processing to fail.

If any `Server` panics or returns an error during runtime, the framework triggers the graceful shutdown flow.

## Listen for Exit Signals

After `gs.Run()` starts successfully, it listens for two common signals:

- `SIGINT`: usually triggered by Ctrl+C.
- `SIGTERM`: sent when environments such as Docker and Kubernetes stop containers.

After receiving a signal, Go-Spring records a log and calls `ShutDown()` to enter the graceful shutdown flow.

## Graceful Shutdown

```
                Trigger ShutDown()
                     |
                     v
              Cancel root context
      All logic listening to ctx.Done() receives notification
                     |
    -----------------+-----------------
    |                |                |
    v                v                v
Server 1 Stop()  Server 2 Stop()  ...
    |                |                |
    -----------------+-----------------
                     |
                     v
          Wait for all Server goroutines to exit
                     |
                     v
              Close IoC container
          Call Destroy methods of related Beans
                     |
                     v
            Flush logs and end process
```

Note that Go-Spring does not set a global forced shutdown timeout. The framework waits until all resources are cleaned up before exiting.

This is an explicit design trade-off:
different businesses have widely varying requirements for shutdown wait times, and the framework cannot provide a default value suitable for all scenarios.

## Implement Runner

`Runner` is suitable for one-time initialization tasks. It executes sequentially during startup and ends after execution is complete.

The `Runner` interface is as follows:

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

Example: automatically create tables at startup.

```go
type DBMigrator struct {
    DB *sql.DB `autowire:""`
}

func (m *DBMigrator) Run(ctx context.Context) error {
    _, err := m.DB.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            name TEXT NOT NULL
        );
    `)
    return err
}

func init() {
	gs.Provide(&DBMigrator{}).Export(gs.As[gs.Runner]())
}
```

The code above registers the `DBMigrator` instance as a `Runner`.
During startup, the framework automatically discovers and executes this `Runner`.

## Implement Server

`Server` is suitable for long-running services. It continues running after Runner execution is complete until the application shuts down.

The `Server` interface is as follows:

```go
type Server interface {
    // Run must block until ctx is canceled or the service exits.
	Run(ctx context.Context, sig ReadySignal) error

    // Stop is used to stop the service and release resources.
	Stop() error
}
```

The following is a simplified example of a custom HTTP Server:

```go
type MyServer struct {
	Addr string `value:"${server.addr:=:8080}"`
	srv  *http.Server
}

func (s *MyServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	s.srv = &http.Server{Addr: s.Addr}

	// 1. Complete listener binding first.
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

	// 2. Then trigger Ready and wait until all Servers are Ready before releasing them together.
	<-sig.TriggerAndWait()

	// 3. Start serving. Serve blocks until the service exits.
	err = s.srv.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *MyServer) Stop() error {
	return s.srv.Shutdown(context.Background())
}
```

Note that the Ready signal should be triggered after `Listen` succeeds.
If Ready is triggered before listening on the port, the application may declare readiness early while the actual port is not yet open, causing external requests to fail.

## Inject Root Context

We strongly recommend binding the lifecycle of the entire application to a root context.
Business code should preferably derive contexts from the root context instead of directly using `context.Background()`.
When the application shuts down, this context is canceled and triggers all logic listening to `ctx.Done()`.

The `ContextProvider` object can be injected through a struct field or constructor.

```go
type MyService struct {
    CtxProvider *gs.ContextProvider `autowire:""`
}

func (s *MyService) DoWork() {
    ctx := s.CtxProvider.Context

    select {
    case <-ctx.Done():
        // The application is shutting down; stop accepting new tasks.
        return
    default:
        // Process normally.
    }
}
```

## Refresh Dynamic Configuration

We can declare dynamic configuration fields on struct fields by using `gs.Dync[T]`.

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

Then the `PropertiesRefresher` object can be used to trigger configuration refresh at runtime.
Dynamic refresh only applies to configuration fields declared with `gs.Dync[T]`.

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

`gs.Dync[T]` is concurrency-safe and suitable for reading the latest configuration value during runtime.
For details on configuration binding and dynamic refresh, see [Configuration Management](/en/docs/2.guides/01-configuration.md).
