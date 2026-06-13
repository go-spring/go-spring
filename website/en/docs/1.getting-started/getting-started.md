# Getting Started

This document helps you quickly get started with Go-Spring and create your first application.

## Requirements

- Go 1.26+ (using the latest Go version is always recommended)
- Go modules enabled

## Creation Methods

There are two ways to create a Go-Spring project:

1. **Create manually from scratch**
    * Suitable for beginners who want to understand how the framework works.

2. **Use `gs init` to create a template project**
    * Generates a complete project skeleton with directory structure, configuration files, and dependency management in one command. Suitable for starting development quickly.

## Create Manually from Scratch

### Installation

Install the Go-Spring core framework:

```bash
go get go-spring.org/spring@latest
```

### Initialize the Project

```bash
mkdir hello-go-spring
cd hello-go-spring
go mod init hello
```

### First Example: Compatible with Standard `net/http`

Go-Spring is fully compatible with Go's standard library. You do not need to change your existing coding habits.

Create `main.go`:

```go
package main

import (
	"net/http"

	"go-spring.org/spring/gs"
)

func main() {
	// Register an HTTP handler using the standard library
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello Go-Spring!"))
	})

	// Start the Go-Spring application
	// Compared with `http.ListenAndServe(":8080", nil)`, `gs.Run()` additionally provides:
	//   ✅ automatic configuration file loading (YAML/properties/ENV)
	//   ✅ an out-of-the-box dependency injection container
	//   ✅ complete Bean lifecycle management
	//   ✅ graceful shutdown support
	gs.Run()
}
```

By default, Go-Spring starts an HTTP server listening on port `9090`. You can change this port through a configuration file.

Run the application:

```bash
go run main.go
```

Test the endpoint:

```bash
curl http://127.0.0.1:9090/echo
# Output: Hello Go-Spring!
```

As you can see, with just one `gs.Run()`, you get a fully functional Go-Spring application.

### Second Example: Using Dependency Injection

Dependency injection is a core feature of Go-Spring. Let's see how to use it to organize code.

```go
package main

import (
	"net/http"

	"go-spring.org/spring/gs"
)

func init() {
	// Register a HelloService object in the IoC container
	gs.Provide(&HelloService{})

	// Register a HelloHandler object in the IoC container
	gs.Provide(&HelloHandler{})

	// Register a gs.HttpServeMux object in the IoC container, while also receiving a HelloHandler object
	gs.Provide(func(h *HelloHandler) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", h.ServeHTTP)
		// Wrapping http.Handler with gs.HttpServeMux enables middleware support
		return &gs.HttpServeMux{Handler: mux}
	})
}

type HelloService struct{}

func (s *HelloService) SayHello(name string) string {
	return "Hello, " + name + "!"
}

type HelloHandler struct {
	HelloService *HelloService `autowire:""` // Inject the HelloService object through autowire
}

func (h *HelloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	msg := h.HelloService.SayHello("Go-Spring")
	_, _ = w.Write([]byte(msg))
}

func main() {
	gs.Run()
}
```

After running, visit:

```bash
curl http://127.0.0.1:9090/hello
# Output: Hello, Go-Spring!
```

## Use `gs init` to Create a Project

<!-- To be added -->
