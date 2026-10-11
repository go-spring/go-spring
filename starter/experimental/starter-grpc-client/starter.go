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

// starter.go is the wiring half of this starter, and it is one line long: this
// package contributes a [Client] bean and nothing else. That is the point of the
// design in client.go and balancer.go — everything a connection needs (the
// discovery directory, the endpoint-selection authority, the per-connection
// policy) is either a field of that bean or created by the [Client.Dial] that
// asked for it. There is no config to bind, because a gsdiscovery connection is
// addressed by the Dial arguments rather than by properties, and no install hook
// to run, because governance reaches each connection through its own
// subscription.
package StarterGrpcClient

import "go-spring.org/spring/gs"

func init() {
	// The dialer an application injects. It is not exported to a collected type:
	// a bean the application autowires is reachable by definition.
	gs.Provide(NewClient)
}
