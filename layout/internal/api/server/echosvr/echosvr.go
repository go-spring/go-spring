// Package echosvr wires an Echo HTTP server into the Go-Spring lifecycle via
// starter-echo. It is intentionally IDL-free and minimal: it only builds an
// *echo.Echo with a health-check route and hands it to the starter, which
// drives startup and graceful shutdown. Add routes, middleware, and groups on
// the engine as your service grows. The listen address is configured under
// ${spring.echo.server} (see conf/app.properties).
package echosvr

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide an *echo.Echo bean; starter-echo registers the gs.Server
	// wrapper only when this bean is present.
	gs.Provide(func() *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())

		// Health-check endpoint; replace or extend with real routes.
		e.GET("/ping", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
		})

		return e
	})
}
