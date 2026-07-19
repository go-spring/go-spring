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

package StarterSwagger

import (
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/endpoint"
)

func init() {
	// Contribute the Swagger UI as an endpoint.Endpoint. The actuator autowires
	// every bean exported as endpoint.Endpoint and mounts it on the management
	// port, so enabling this starter surfaces interactive API docs with zero
	// wiring — no HTTP server or listening port of its own.
	//
	// The bean is also a plain *UI (http.Handler), so an app that does not run
	// the actuator can inject it and mount UI.Path() on its own HTTP server.
	//
	// Gated on spring.swagger.enabled (default on) so it can be disabled in
	// production without removing the import.
	gs.Provide(NewUI, gs.TagArg("${spring.swagger}")).
		Export(gs.As[endpoint.Endpoint]()).
		Condition(gs.OnProperty("spring.swagger.enabled").HavingValue("true").MatchIfMissing())
}
