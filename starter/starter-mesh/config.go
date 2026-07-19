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

package StarterMesh

// Config holds the service-mesh switch, bound from ${spring.mesh}.
type Config struct {
	// Enabled turns service-mesh mode on. Set it to true only when a sidecar is
	// injected (it does discovery and load balancing for you); leave it false on
	// VMs, bare metal, or any deployment without a mesh, where the application's
	// own client-side discovery and load balancing must stay active.
	Enabled bool `value:"${enabled:=false}"`
}
