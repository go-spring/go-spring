// Package httpsvr bootstraps the HTTP server, registers routes and middleware chain.
package httpsvr

import (
	"net/http"

	"GS_PROJECT_MODULE/idl/http/proto"
	"GS_PROJECT_MODULE/internal/api/server/httpsvr/middleware"

	"go-spring.org/spring/gs"
)

// ServerConfig defines the configuration for an HTTP server.
// It contains sub-configs for recovery, tracing, and metrics.
type ServerConfig struct {
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

func init() {
	// Register HTTP handler provider function
	gs.Provide(func(config ServerConfig, server *GS_PROJECT_NAMEController) *gs.HttpServeMux {
		mux := http.NewServeMux()

		// Register all API endpoints defined in the proto to the router
		proto.InitRouter(mux, server)

		// Serve static files under "/static/" from "./public"
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./public"))))

		// Apply middleware chain to the mux and return the final handler
		return &gs.HttpServeMux{
			Handler: middleware.Chain(mux,
				middleware.Recovery(config.RecoveryConfig),
				middleware.Trace(config.TraceConfig),
				middleware.Metric(config.MetricConfig),
			),
		}
	})
}
