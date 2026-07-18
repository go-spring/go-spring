// Package ginsvr wires a Gin HTTP server into the Go-Spring lifecycle via
// starter-gin. It is intentionally IDL-free and minimal: it only provides a
// RouterRegister that mounts a health-check route. starter-gin owns the
// *gin.Engine and its HTTP server (address under ${spring.gin.server}, see
// conf/app.properties) and drives startup and graceful shutdown. Add routes,
// middleware, and groups on the engine handed to the register as your service
// grows.
package ginsvr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"

	StarterGin "go-spring.org/starter-gin"
)

func init() {
	// Provide a RouterRegister bean; starter-gin registers the gs.Server
	// wrapper only when this bean is present, then calls it with the engine
	// it owns.
	gs.Provide(func() StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			// Health-check endpoint; replace or extend with real routes.
			e.GET("/ping", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "pong"})
			})
		}
	})
}
