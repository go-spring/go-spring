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
)

func init() {
	// Shared instrument set for the route table. Its metrics ride the OTel
	// pipeline; there is no private endpoint to keep in sync with it.
	gs.Provide(newObserver)

	// The compiled, hot-reloadable route table. Its ${spring.gateway} config
	// and optional FilterWrapper beans (jwt-auth, lua) are populated by field
	// injection; the container's named discovery backend beans are collected by
	// the constructor (lb:// upstreams resolve their label against them), and so
	// are the governance authorities it drives per-route protection and endpoint
	// selection with; route compilation is deferred to server startup (warmup).
	// Discovery runs inside the backend (loaders have no resources), so there is
	// no destroy half to register.
	gs.Provide(newRouteTable,
		gs.IndexArg(2, gs.TagArg("?")),
		// The governance beans are REQUIRED: each is registered by the package that
		// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
		// starter imports — "governance off" is spring.governance.enabled=false, never
		// an absent bean.
		gs.IndexArg(3, gs.TagArg("")),
		gs.IndexArg(4, gs.TagArg("")),
		gs.IndexArg(5, gs.TagArg("?")),
	).Caller(1)

	// The listen-port server, wired into graceful drain as a gs.Server. Named
	// so it coexists with the application's main HTTP server (which also
	// exports gs.Server) without a duplicate-bean clash.
	gs.Provide(newGatewayServer).
		Name("gatewayServer").
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.gateway.server.addr"))

	// Report gateway health. The metrics need no contribution: they are OTel
	// instruments now, so starter-otel's Prometheus exporter exposes them on the
	// same /metrics (actuator management port) as the rest of the application.
	gs.Provide(newGatewayHealth)
}

// The self-contained filter factories (implementations in filter.go).
func init() {
	RegisterFilter("stripPrefix", stripPrefixFilter)
	RegisterFilter("prefixPath", prefixPathFilter)
	RegisterFilter("addRequestHeader", headerFilter(reqHeader, headerAdd))
	RegisterFilter("setRequestHeader", headerFilter(reqHeader, headerSet))
	RegisterFilter("removeRequestHeader", headerFilter(reqHeader, headerRemove))
	RegisterFilter("addResponseHeader", headerFilter(respHeader, headerAdd))
	RegisterFilter("setResponseHeader", headerFilter(respHeader, headerSet))
	RegisterFilter("removeResponseHeader", headerFilter(respHeader, headerRemove))
	RegisterFilter("rewriteHost", rewriteHostFilter)
	RegisterFilter("preserveHostHeader", preserveHostFilter)
	RegisterFilter("requestId", requestIDFilter)
	// rateLimit is deliberately NOT registered here: it needs a limiter backend,
	// which lives in the container, so the route table handles it directly (see
	// RouteTable.buildFilters) rather than through this self-contained factory
	// registry.
}
