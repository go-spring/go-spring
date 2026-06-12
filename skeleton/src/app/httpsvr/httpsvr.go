package httpsvr

import (
	"net/http"

	"GS_PROJECT_MODULE/idl/http/proto"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Register HTTP handler provider function
	gs.Provide(func(config *ServerConfig, server *GS_PROJECT_NAMEController) *gs.HttpServeMux {
		mux := http.NewServeMux()

		// Register all API endpoints defined in the proto to the router
		proto.InitRouter(mux, server)

		// Serve static files under "/static/" from "./public"
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./public"))))

		// Apply middleware chain to the mux and return the final handler
		return &gs.HttpServeMux{
			Handler: Chain(mux,
				Recovery(config.RecoveryConfig),
				Trace(config.TraceConfig),
				Metric(config.MetricConfig),
			),
		}
	})
}
