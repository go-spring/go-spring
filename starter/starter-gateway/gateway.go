/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterGateway

import (
	"go-spring.org/spring/gs"
	"go-spring.org/spring/endpoint"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/spring/health"
)

func init() {
	// The gateway serves as soon as it is enabled (default on): its "routes" are
	// config, not application-registered functions, so — unlike the grpc starter —
	// it does not additionally require an app-provided registration bean.
	enable := gs.OnProperty("spring.gateway.server.enabled").HavingValue("true").MatchIfMissing()

	gs.Module(enable, func(r gs.BeanProvider, p flatten.Storage) error {
		// Shared metrics collector for the route table and the /metrics endpoint.
		r.Provide(newMetrics)

		// The compiled, hot-reloadable route table. Its ${spring.gateway} config
		// and optional FilterWrapper beans (jwt-auth, lua) are populated by field
		// injection; route compilation is deferred to server startup (warmup).
		r.Provide(newRouteTable)

		// The listen-port server, wired into graceful drain as a gs.Server. Named
		// so it coexists with the application's main HTTP server (which also
		// exports gs.Server) without a duplicate-bean clash.
		r.Provide(newGatewayServer).
			Name("gatewayServer").
			Export(gs.As[gs.Server]())

		// Contribute /gateway/metrics to the actuator and report gateway health.
		r.Provide(newMetricsEndpoint).Export(gs.As[endpoint.Endpoint]())
		r.Provide(newGatewayHealth).Export(gs.As[health.Indicator]())
		return nil
	})
}
