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

package StarterActuator

import (
	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/spring/gs"
)

func init() {
	// Mark that a management server collecting endpoint.Endpoint beans is
	// linked in, so contributors (e.g. starter-otel's Prometheus /metrics with
	// metrics.port=0) can WARN at startup when they would otherwise be
	// silently homeless. See endpoint.MarkServing.
	endpoint.MarkServing()

	// Register the actuator as a gs.Server under a distinct name so it coexists
	// with the application's main HTTP server (which also exports gs.Server).
	// The starter convention: spring.actuator.enabled defaults to on (the
	// switch opts OUT), while spring.actuator.addr must still be set — the
	// address opts IN. Both must hold.
	gs.Provide(NewServer,
		gs.IndexArg(0, gs.TagArg("${spring.actuator}")),
		gs.IndexArg(1, gs.TagArg("?")),
		gs.IndexArg(2, gs.TagArg("?")),
	).
		Condition(gs.And(
			gs.OnProperty("spring.actuator.enabled").HavingValue("true").MatchIfMissing(),
			gs.OnProperty("spring.actuator.addr"),
		)).
		Export(gs.As[gs.Server]())
}
