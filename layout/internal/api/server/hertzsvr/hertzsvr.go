// Package hertzsvr wires a Hertz HTTP server into the Go-Spring lifecycle via
// starter-hertz. It is intentionally IDL-free and minimal: it only provides a
// RouterRegister that mounts a health-check route. starter-hertz owns the
// *server.Hertz and its listener (address under ${spring.hertz.server}, see
// conf/app.properties) and drives startup and graceful shutdown. Add routes and
// middleware on the engine handed to the register as your service grows.
package hertzsvr

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go-spring.org/spring/gs"

	StarterHertz "go-spring.org/starter-hertz"
)

func init() {
	// Provide a RouterRegister bean; starter-hertz registers the gs.Server
	// wrapper only when this bean is present, then calls it with the engine
	// it owns.
	gs.Provide(func() StarterHertz.RouterRegister {
		return func(h *server.Hertz) {
			// Health-check endpoint; replace or extend with real routes.
			h.GET("/ping", func(ctx context.Context, r *app.RequestContext) {
				r.JSON(consts.StatusOK, map[string]string{"message": "pong"})
			})
		}
	})
}
