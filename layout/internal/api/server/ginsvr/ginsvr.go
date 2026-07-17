// Package ginsvr wires a Gin HTTP server into the Go-Spring lifecycle via
// starter-gin. It is intentionally IDL-free and minimal: it only builds a
// *gin.Engine with a health-check route and hands it to the starter, which
// drives startup and graceful shutdown. Add routes, middleware, and groups on
// the engine as your service grows. The listen address is configured under
// ${spring.gin.server} (see conf/app.properties).
package ginsvr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide a *gin.Engine bean; starter-gin registers the gs.Server
	// wrapper only when this bean is present.
	gs.Provide(func() *gin.Engine {
		gin.SetMode(gin.ReleaseMode)
		e := gin.New()
		e.Use(gin.Recovery())

		// Health-check endpoint; replace or extend with real routes.
		e.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		return e
	})
}
