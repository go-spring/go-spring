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

package StarterRegistryK8s

import (
	"context"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var (
	// starterTag identifies logs emitted by the k8s registry starter. It is a
	// user-facing config key — USAGE documents tuning this starter's logging via
	// `logger.<name>.tag=_app_registry_k8s`.
	starterTag = log.RegisterAppTag("registry_k8s", "")
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute (starter/DESIGN.md §3), so one dashboard can compare
// registry backends. This backend reports the READ side only: it has no
// registrar, so it emits no RegisterAttempt / DeregisterAttempt / WeightChange.
const obsSystem = "k8s"

func init() {
	// One NAMED bean per block under ${spring.registry.k8s.<name>}: the bean
	// name "k8s.<name>" is the label a client starter cites to pick this
	// backend (the name carries the backend type, so blocks across backends
	// never collide). The container is the discovery directory — a backend is
	// constructed at injection time (only when something cites it) and its
	// background resources (client-go informers) are released by the bean
	// destructor on shutdown. A duplicate name within this backend fails loudly
	// in the container.
	gs.Module(gs.OnProperty("spring.registry.k8s"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.registry.k8s}", func(name string, c Config) error {
			r.Provide(newBackendBean,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name("k8s." + name).Destroy(destroyBackendBean).Caller(1)
			log.Debugf(context.Background(), starterTag, "declared k8s discovery backend bean name=%s mode=%s namespace=%s", "k8s."+name, c.Mode, c.Namespace)
			return nil
		})
	})
}

// newBackendBean builds the discovery backend for c. It runs at injection
// time, so an invalid mode or an unreachable API server fails startup.
func newBackendBean(ctx *gs.ContextProvider, c Config) (discovery.Discovery, error) {
	log.Debugf(ctx.Context, starterTag, "creating k8s discovery backend mode=%s namespace=%s", c.Mode, c.Namespace)
	b, err := newBackend(ctx, c)
	if err != nil {
		return nil, errutil.Explain(err, "registry-k8s: build backend")
	}
	return b, nil
}

// destroyBackendBean releases the backend's background resources on shutdown.
// dns-mode backends hold nothing to close; endpointslice-mode backends expose
// Close.
func destroyBackendBean(d discovery.Discovery) error {
	if c, ok := d.(interface{ Close() error }); ok {
		return errutil.Explain(c.Close(), "registry-k8s: close backend")
	}
	return nil
}
