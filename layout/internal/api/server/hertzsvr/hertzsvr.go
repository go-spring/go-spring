// Package hertzsvr wires a Hertz HTTP server into the Go-Spring lifecycle via
// starter-hertz. It is intentionally IDL-free and minimal: it only builds a
// *server.Hertz with a health-check route and hands it to the starter, which
// drives startup and graceful shutdown. Add routes and middleware on the engine
// as your service grows. Unlike gin/echo, Hertz owns its own listener, so the
// listen address is set here via WithHostPorts, not in conf/app.properties.
package hertzsvr

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide a *server.Hertz bean; starter-hertz registers the gs.Server
	// wrapper only when this bean is present.
	gs.Provide(func() *server.Hertz {
		h := server.Default(server.WithHostPorts("127.0.0.1:8003"))

		// Health-check endpoint; replace or extend with real routes.
		h.GET("/ping", func(ctx context.Context, r *app.RequestContext) {
			r.JSON(consts.StatusOK, map[string]string{"message": "pong"})
		})

		return h
	})
}
