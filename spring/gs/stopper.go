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

package gs

import (
	"context"
	"sync"

	"go-spring.org/log"
	"go-spring.org/spring/gs/internal/gs_bean"
)

// Stopper is a shutdown function for a process-global resource whose cleanup
// must run at exit independently of the bean dependency graph. The application
// runtime invokes every registered Stopper after all servers have stopped and
// the IoC container has closed, so a global resource can flush buffered data
// (e.g. OTel spans/metrics) after in-flight requests finish.
//
// This is an intentionally narrow escape hatch, NOT a general-purpose shutdown
// callback. Reserve it for process-level singletons a starter installs as a side
// effect of import (OTel providers, a profiler, ...) whose lifecycle is not -
// and cannot be - modelled by bean autowiring: such a resource is live the
// moment it is installed (e.g. otel.SetTracerProvider) regardless of whether any
// bean autowires it, so the bean destroyer (gated by reachability) would never
// fire for it. Ordinary resources (connection pools, clients, ...) belong on a
// bean's Destroy hook, which respects dependency order and reachability as
// intended; misusing RegisterStopper for them bypasses those guarantees.
//
// It is a plain function (not an interface) so a resource's existing
// Shutdown(context.Context) error method can be registered directly, e.g.
// gs.RegisterStopper("otel-trace", tp.Shutdown), with no adapter.
//
// When a stopper can be registered depends on when its resource is constructed:
// one with no configuration dependency may register at Run entry; one built from
// ${spring.*} config (e.g. OTel providers) must register from a module's setup,
// where configuration is already loaded.
//
// Stoppers must be independent: like servers, they run in no defined order and
// must not rely on one another's cleanup having run (the logging system, for
// instance, is handled separately - not as a stopper - precisely because others
// may want to log during flush). If an ordering requirement emerges, the model
// will need redesigning.
type Stopper func(context.Context) error

var (
	stopperMu sync.RWMutex
	stoppers  = map[string]Stopper{} // name -> stopper
)

// RegisterStopper makes a process-global resource stopper available under name.
// It panics on empty name or nil stopper. A repeated registration under the same
// name replaces the previous one (last-wins) - this mirrors how a module's setup
// installs other process globals (otel.SetTracerProvider, discovery.SetTraceInjector)
// and lets a setup that re-runs across app instances in one process (e.g. sequential
// gs.RunTest runs) rebind the stopper to the current resource instead of panicking.
func RegisterStopper(name string, s Stopper) {
	if name == "" {
		panic("gs: register stopper with empty name")
	}
	if s == nil {
		panic("gs: register nil stopper for " + name)
	}
	stopperMu.Lock()
	defer stopperMu.Unlock()
	stoppers[name] = s
	log.Debugf(context.Background(), gs_bean.TagBeanLifecycle, "global stopper registered: %s", name)
}

// runStoppers invokes every registered stopper. Stoppers are independent and run
// in no defined order (map iteration); each must not rely on another having run.
// A failing stopper is logged but does not block the rest. It is idempotent: it
// drains the registry, so a second call is a no-op.
func runStoppers(ctx context.Context) {
	stopperMu.Lock()
	reg := stoppers
	stoppers = map[string]Stopper{}
	stopperMu.Unlock()

	ctx = context.WithoutCancel(ctx)
	for name, s := range reg {
		if err := s(ctx); err != nil {
			log.Errorf(ctx, gs_bean.TagBeanLifecycle, "stopper %q failed: %v", name, err)
			continue
		}
		log.Debugf(ctx, gs_bean.TagBeanLifecycle, "stopper %q done", name)
	}
}

// resetStoppersForTest clears the stopper registry. Test-only.
func resetStoppersForTest() {
	stopperMu.Lock()
	defer stopperMu.Unlock()
	stoppers = map[string]Stopper{}
}
