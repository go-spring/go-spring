// Package echosvr wires an Echo HTTP server into the Go-Spring lifecycle via
// starter-echo. It is intentionally IDL-free and minimal: it only provides a
// RouterRegister that mounts a health-check route. starter-echo owns the
// *echo.Echo and its HTTP server (address under ${spring.echo.server}, see
// conf/app.properties) and drives startup and graceful shutdown. Add routes,
// middleware, and groups on the engine handed to the register as your service
// grows.
package echosvr

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go-spring.org/spring/gs"

	StarterEcho "go-spring.org/starter-echo"
)

func init() {
	// Provide a RouterRegister bean; starter-echo registers the gs.Server
	// wrapper only when this bean is present, then calls it with the engine
	// it owns.
	gs.Provide(func() StarterEcho.RouterRegister {
		return func(e *echo.Echo) {
			// Health-check endpoint; replace or extend with real routes.
			e.GET("/ping", func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
			})
		}
	})
}
