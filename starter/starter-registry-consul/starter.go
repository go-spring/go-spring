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

// Package StarterRegistryConsul adapts Consul as a service registry. Each
// ${spring.registry.consul.<name>} block becomes ONE backend bean named
// "consul.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registrar collected by the starter-registry core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name).
// Blank-import the package and configure one block per Consul agent:
//
//	spring.registry.consul.main.address=127.0.0.1:8500
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-registry-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to Consul.
//
// Importing this package imports the starter-registry registration core
// transitively: the single registryServer bean that publishes into every
// configured center (across backends) exists exactly once per process.
package StarterRegistryConsul

import (
	"go-spring.org/log"

	// The registration core: provides the single registryServer that collects
	// this backend's registrar beans. Go runs its package init exactly once no
	// matter how many backend starters import it.
	_ "go-spring.org/starter-registry"
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute, so one dashboard can compare registry centers.
const obsSystem = "consul"

var (
	// starterTag identifies logs emitted by the consul registry starter.
	starterTag = log.RegisterAppTag("registry_consul", "")
)
