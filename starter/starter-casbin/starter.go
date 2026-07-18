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

package StarterCasbin

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Casbin enforcers as a group.
	// Each instance is created according to the configuration in "${spring.casbin}",
	// so an application can hold several enforcers (e.g. one per domain) side by side
	// and inject the one it needs by bean name.
	gs.Group("${spring.casbin}", newEnforcer, destroyEnforcer)
}

// Enforcer wraps *casbin.Enforcer so the starter can own resources that Casbin
// does not close on its own — notably a policy watcher, whose background work is
// released in destroyEnforcer. The embedded *casbin.Enforcer promotes all the
// usual methods (Enforce, AddPolicy, ...), so callers use the bean exactly like
// a plain enforcer.
type Enforcer struct {
	*casbin.Enforcer
	watcher persist.Watcher
}

// newEnforcer builds an Enforcer from the model plus either a file-backed policy
// (the default, dependency-free path) or a registered persist.Adapter for
// DB/other storage. When a watcher is configured, policy changes signaled by
// other instances trigger an automatic LoadPolicy, giving hot reload and
// multi-instance synchronization. Adapter and watcher are both optional and
// supplied by the application via RegisterAdapter / RegisterWatcher.
func newEnforcer(c Config) (*Enforcer, error) {
	var (
		e   *casbin.Enforcer
		err error
	)
	if c.Adapter != "" {
		a, ok := lookupAdapter(c.Adapter)
		if !ok {
			return nil, errutil.Explain(nil, "casbin: adapter %q not registered", c.Adapter)
		}
		e, err = casbin.NewEnforcer(c.Model, a)
	} else {
		e, err = casbin.NewEnforcer(c.Model, c.Policy)
	}
	if err != nil {
		return nil, errutil.Explain(err, "failed to create casbin enforcer")
	}
	e.EnableAutoSave(c.AutoSave)

	enforcer := &Enforcer{Enforcer: e}
	if c.Watcher != "" {
		w, ok := lookupWatcher(c.Watcher)
		if !ok {
			return nil, errutil.Explain(nil, "casbin: watcher %q not registered", c.Watcher)
		}
		if err := e.SetWatcher(w); err != nil {
			return nil, errutil.Explain(err, "casbin: set watcher %q", c.Watcher)
		}
		// A classic callback reloads the policy so this instance picks up
		// changes made by peers.
		if err := w.SetUpdateCallback(func(string) { _ = e.LoadPolicy() }); err != nil {
			return nil, errutil.Explain(err, "casbin: set watcher callback")
		}
		enforcer.watcher = w
	}
	return enforcer, nil
}

// destroyEnforcer releases the watcher's background resources. The enforcer
// itself holds nothing else that needs closing.
func destroyEnforcer(e *Enforcer) error {
	if e.watcher != nil {
		e.watcher.Close()
	}
	return nil
}
