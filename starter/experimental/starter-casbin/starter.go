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
	"context"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Casbin enforcers under ${spring.casbin}, one bean per map
	// key, so an application can hold several enforcers (e.g. one per domain)
	// side by side and inject the one it needs by bean name.
	//
	// Unlike gs.Group, we hand-wire each enforcer in a gs.Module so its ctor can
	// receive the policy adapter / watcher as named beans: the config keys
	// `adapter` / `watcher` name a bean the application provides, and the
	// enforcer injects it by that name (nil when the key is empty — the
	// file-backed policy path). No package-level registry: adapters/watchers are
	// ordinary beans the app owns.
	gs.Module(gs.OnProperty("spring.casbin"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.casbin}", func(name string, c Config) error {
			return registerEnforcer(r, name, c)
		})
	})
}

// registerEnforcer registers one named enforcer whose ctor binds the policy
// adapter and watcher the config selects, by bean name.
func registerEnforcer(r gs.BeanProvider, name string, c Config) error {
	b := r.Provide(func(ctx *gs.ContextProvider, a persist.Adapter, w persist.Watcher) (*Enforcer, error) {
		return newEnforcer(ctx.Context, c, a, w)
	}, gs.IndexArg(1, adapterArg(c)), gs.IndexArg(2, watcherArg(c)))
	b.Name(name).Destroy(destroyEnforcer)
	return nil
}

// adapterArg resolves the adapter ctor arg: the bean named by c.Adapter when the
// key is set, otherwise nil (file-backed policy path). When c.Adapter names a
// bean the application forgot to provide, injection fails loudly at startup —
// same fail-fast as the previous "adapter not registered" error.
func adapterArg(c Config) gs.Arg {
	if c.Adapter == "" {
		return gs.ValueArg((persist.Adapter)(nil))
	}
	return gs.TagArg(c.Adapter)
}

// watcherArg resolves the watcher ctor arg the same way: the bean named by
// c.Watcher when set, otherwise nil (no watcher).
func watcherArg(c Config) gs.Arg {
	if c.Watcher == "" {
		return gs.ValueArg((persist.Watcher)(nil))
	}
	return gs.TagArg(c.Watcher)
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
// (the default, dependency-free path, adapter == nil) or the supplied
// persist.Adapter for DB/other storage. When watcher is non-nil, policy changes
// signaled by other instances trigger an automatic LoadPolicy, giving hot reload
// and multi-instance synchronization. adapter and watcher are resolved by the
// starter's Module from the beans the config names — never nil here unless the
// corresponding config key is empty.
func newEnforcer(ctx context.Context, c Config, adapter persist.Adapter, watcher persist.Watcher) (*Enforcer, error) {
	var (
		e   *casbin.Enforcer
		err error
	)

	log.Debugf(ctx, log.TagAppDef, "creating casbin enforcer model=%s adapter=%s watcher=%s", c.Model, c.Adapter, c.Watcher)

	// `policy` and `adapter` are mutually exclusive storage selections: the
	// enforcer must load from exactly one place. Configuring both is almost
	// certainly a mistake (one of them would be silently ignored), so fail fast
	// instead of picking a winner.
	if c.Adapter != "" && c.Policy != "" {
		return nil, errutil.Explain(nil, "casbin: `policy` and `adapter` are mutually exclusive, configure only one (policy=%q adapter=%q)", c.Policy, c.Adapter)
	}

	if c.Adapter != "" {
		// c.Adapter names an adapter bean; a nil here means the application set
		// the key but provided no such bean.
		if adapter == nil {
			return nil, errutil.Explain(nil, "casbin: no adapter bean named %q", c.Adapter)
		}
		e, err = casbin.NewEnforcer(c.Model, adapter)
	} else {
		e, err = casbin.NewEnforcer(c.Model, c.Policy)
	}
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "create casbin enforcer failed: %v", err)
		return nil, errutil.Explain(err, "failed to create casbin enforcer")
	}
	e.EnableAutoSave(c.AutoSave)

	enforcer := &Enforcer{Enforcer: e}
	if watcher != nil {
		if err := e.SetWatcher(watcher); err != nil {
			return nil, errutil.Explain(err, "casbin: set watcher %q", c.Watcher)
		}
		// A classic callback reloads the policy so this instance picks up
		// changes made by peers.
		if err := watcher.SetUpdateCallback(func(string) { _ = e.LoadPolicy() }); err != nil {
			return nil, errutil.Explain(err, "casbin: set watcher callback")
		}
		enforcer.watcher = watcher
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
