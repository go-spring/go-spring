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

// Package StarterMesh wires the process-global service-mesh switch.
//
// When the application runs inside a mesh (Istio/Envoy, Linkerd, ...) the
// injected sidecar already performs service discovery and load balancing.
// Leaving the application's own client-side discovery (stdlib/discovery) and
// load balancing (stdlib/loadbalance) enabled on top of that balances traffic
// twice and lets locality/outlier logic fight the mesh. Importing this starter
// and setting ${spring.mesh.enabled}=true degrades both layers to a
// pass-through: names resolve to the stable Service address the sidecar
// intercepts, and the balancer stops selecting.
//
// The switch is read once here, at startup, and applied centrally in the
// discovery/loadbalance factory points — no client starter special-cases it.
package StarterMesh

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// A gs.Module, not a bean: its body runs during applyModules in the
	// RefreshPrepare phase, before any bean is instantiated. Setting the mesh
	// switch here therefore guarantees it is live before the first client
	// constructor builds a dialer. Doing it in a bean constructor would race with
	// clients constructed earlier.
	gs.Module(nil, setup)
}

// setup binds ${spring.mesh}, resolves the mode to a boolean and applies the
// switch to stdlib/discovery, which stdlib/loadbalance also reads. A false value
// (the default) is applied too, so an imported-but-disabled starter leaves the
// client-side stack fully active.
func setup(_ gs.BeanProvider, p flatten.Storage) error {
	var cfg Config
	if err := conf.Bind(p, &cfg, "${spring.mesh}"); err != nil {
		return err
	}
	enabled, err := resolveMeshMode(cfg.Enabled)
	if err != nil {
		return err
	}
	discovery.SetMeshMode(enabled)
	if enabled {
		log.Infof(context.Background(), log.TagAppDef,
			"service-mesh mode enabled: client-side discovery and load balancing degrade to pass-through; the sidecar owns them")
	}
	return nil
}

// resolveMeshMode turns the configured ${spring.mesh.enabled} value into the
// boolean the discovery layer consumes. "auto" infers from the environment via
// discovery.DetectMesh; any other value is parsed as a boolean, keeping the
// explicit true/false as the single source of truth.
func resolveMeshMode(mode string) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(mode), "auto") {
		detected := discovery.DetectMesh()
		log.Infof(context.Background(), log.TagAppDef,
			"service-mesh mode=auto: sidecar %s", map[bool]string{true: "detected", false: "not detected"}[detected])
		return detected, nil
	}
	enabled, err := strconv.ParseBool(mode)
	if err != nil {
		return false, fmt.Errorf("invalid spring.mesh.enabled %q: want true, false, or auto", mode)
	}
	return enabled, nil
}
