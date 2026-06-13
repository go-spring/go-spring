# Built-in HTTP Server

Go-Spring includes a built-in HTTP Server, suitable for scenarios where an application directly exposes HTTP APIs.
It is built on the standard library `net/http`, starts with the application by default, and is managed uniformly as part of the Go-Spring lifecycle.

- **Enabled by default**: Automatically registered and initialized through the Starter mechanism, with no extra configuration required.
- **Standard compatible**: Fully compatible with `net/http`, so you can continue using standard library routing and handler patterns.
- **Flexible extension**: Can integrate any routing framework that implements the `http.Handler` interface.
- **Graceful shutdown**: Startup and shutdown are managed uniformly by the application lifecycle, making it suitable for production environments.

## Quick Start

```go
package main

import (
	"net/http"

	"go-spring.org/spring/gs"
)

func init() {
	// Use the HTTP standard library routing and handler style; Go-Spring will automatically take over and make it effective.
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Go-Spring!"))
	})
}

func main() {
	gs.Run()
}
```

After the application starts, visit `http://localhost:9090/hello` in a browser to see the response.

## Configuration Items

The built-in HTTP Server supports the following configuration items:

| Configuration item | Description | Default value |
|--------|------|--------|
| `spring.http.server.addr` | Listen address | `:9090` |
| `spring.http.server.readTimeout` | Timeout for reading requests | `5s` |
| `spring.http.server.headerTimeout` | Timeout for reading request headers | `1s` |
| `spring.http.server.writeTimeout` | Timeout for writing responses | `5s` |
| `spring.http.server.idleTimeout` | Timeout for idle connections | `60s` |
| `spring.http.server.enabled` | Whether to enable HTTP Server | `true` |

To change the listen port, set it in the configuration file:

```properties
spring.http.server.addr=:8080
```

To disable the built-in HTTP Server, set:

```properties
spring.http.server.enabled=false
```

You can also disable the built-in HTTP Server in code with `gs.Web(false)`:

```go
func main() {
	gs.Web(false).Run()
}
```

## Routing Mechanism

Go-Spring wraps the standard library `http.Handler` with `gs.HttpServeMux`,
so HTTP routing can be integrated into the IoC container and application lifecycle management.
At the same time, it can reuse the standard library handler model and preserve the composition style of HTTP middleware.

```go
type HttpServeMux struct {
	http.Handler
}
```

By default, Go-Spring wraps `http.DefaultServeMux` and registers it in the container as a Bean.
Therefore, routes can be registered directly through the global `http.HandleFunc` and `http.Handle`.

If you need to replace the default router, you can provide a custom `*gs.HttpServeMux`.
When creating `*gs.HttpServeMux`, you can also use other Beans through dependency injection.

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"go-spring.org/spring/gs"
)

type UserController struct{}

func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.FormValue("user"))
}

// logging is standard HTTP middleware used to record request logs.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func init() {
	// UserController is registered as a regular Bean and can be injected into route definitions.
	gs.Provide(new(UserController))

	// Redefine the routing entry point for the HTTP Server.
	// Go-Spring uses http.DefaultServeMux by default;
	// but when a custom *gs.HttpServeMux exists in the container,
	// the built-in HTTP Server will replace the default router with the *gs.HttpServeMux returned here.
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)

		// A custom router can also wrap middleware before being passed to HttpServeMux.
		return &gs.HttpServeMux{Handler: logging(mux)}
	})
}

func main() {
	gs.Run()
}
```

## Routing Integration

Common HTTP routing frameworks such as Gin, chi, and gorilla/mux all implement the `http.Handler` interface,
so they can all be integrated by providing a custom `*gs.HttpServeMux`.

This integration method does not change the usage of the third-party framework itself. Routing, middleware, parameter parsing, and other capabilities are still handled by the corresponding framework.
Go-Spring is only responsible for using the final `http.Handler` as the routing entry point of the HTTP Server.

### Integrating Gin

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
)

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// Create a Gin engine; routes can be defined with Gin's native API.
		g := gin.Default()

		// Register Gin middleware.
		g.Use(func(c *gin.Context) {
			log.Printf("%s %s", c.Request.Method, c.Request.URL.Path)
			c.Next()
		})

		// Register Gin routes.
		g.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "pong"})
		})

		// Gin Engine implements http.Handler, so it can be used as Go-Spring's routing entry point.
		return &gs.HttpServeMux{Handler: g}
	})

	gs.Run()
}
```

### Integrating gorilla/mux

```go
package main

import (
	"log"
	"net/http"

	"go-spring.org/spring/gs"
	"github.com/gorilla/mux"
)

// logging is standard HTTP middleware used to record request logs.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// Create a gorilla/mux router; routes can be defined with the gorilla/mux native API.
		m := mux.NewRouter()

		// Register gorilla/mux middleware.
		m.Use(logging)

		// Register gorilla/mux routes.
		m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})

		// gorilla/mux Router implements http.Handler, so it can be used as Go-Spring's routing entry point.
		return &gs.HttpServeMux{Handler: m}
	})

	gs.Run()
}
```

### Integrating chi

```go
package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go-spring.org/spring/gs"
)

// logging is standard HTTP middleware used to record request logs.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// Create a chi router; routes and middleware can be defined with chi's native API.
		c := chi.NewRouter()

		// Register chi middleware.
		c.Use(logging)

		// Register chi routes.
		c.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})

		// chi Router implements http.Handler, so it can be used as Go-Spring's routing entry point.
		return &gs.HttpServeMux{Handler: c}
	})

	gs.Run()
}
```

## Lifecycle

Go-Spring's built-in HTTP Server is implemented by `gs.SimpleHttpServer`.
It implements the `gs.Server` interface and integrates into the application's startup and shutdown processes through `Run` and `Stop`.

```go
func (s *SimpleHttpServer) Run(ctx context.Context, sig ReadySignal) error {
	// Listen on the port first to detect startup errors such as port conflicts as early as possible.
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.svr.Addr)
	}

	// Wait for ReadySignal to trigger to ensure other Servers are ready.
	<-sig.TriggerAndWait()

	// Start accepting HTTP requests after the application is ready.
	err = s.svr.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}
```

A key point during startup is that after the port is successfully listened on, the HTTP Server waits for other Servers to finish preparation
before it starts accepting requests. This avoids the situation where the health check endpoint is already available and traffic has entered, but another Server subsequently fails to start,
ultimately causing request processing failures.

```go
// Stop gracefully stops the HTTP Server and allows in-flight requests to complete.
func (s *SimpleHttpServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
```

When the application receives a stop signal (such as `SIGINT` or `SIGTERM`), Go-Spring automatically calls the `Stop` method
and performs the standard graceful shutdown process for HTTP Server: stop accepting new connections, wait for in-flight requests to complete, close idle connections, and exit.
With this mechanism, the service will not actively interrupt business requests that are being processed during shutdown, making it suitable for online service deployment, restart, and offline scenarios.
