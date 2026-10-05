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

// Package luohua is a company umbrella starter (the "aggregator/profile"
// archetype) that re-bases go-spring onto a hypothetical company's conventions
// by wiring its standard components, not by re-implementing them.
//
// A company blanket-imports this package and sets spring.luohua.*; each
// capability below composes an existing go-spring starter through its public
// seam — never a private path. Concretely, this edition re-bases:
//
//   - wire vocabulary (propagate.go): luohua's own load-test header and its
//     named business headers, carried by a propagator registered into
//     starter-otel/trace (the G1/G2 seams);
//   - log fields (observability.go): the context fields luohua prints on every
//     log event.
//
// Standard components + company customization point is the governing rule:
// for every default luohua provides there is an explicit seam (a config key, a
// Register* driver registry, or an OnMissingBean default) through which a user
// who adopts the standard component but wants to customise here can do so.
package luohua

import (
	"context"

	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/security"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-otel/trace"
	StarterRedigo "go-spring.org/starter-redigo"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/i18n"
)

func init() {
	// Armed by any spring.luohua.* key (OnProperty is a prefix check); Enabled
	// (default true) is the master switch inside.
	gs.Module(gs.OnProperty("spring.luohua"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c Config
		if err := conf.Bind(p, &c, "${spring.luohua:=}"); err != nil {
			return err
		}
		if !c.Enabled {
			return nil
		}
		log.Infof(context.Background(), tagAppLuohua, "luohua baseline armed")
		return apply(r, c)
	})
}

// tagAppLuohua is the static log tag luohua stamps on its own startup lines. A
// company adds its own semantic tags once, here — this is the log-tag
// vocabulary extension point.
var tagAppLuohua = log.RegisterAppTag("luohua", "")

// disabled reports whether the app explicitly switched off the whole luohua
// baseline (spring.luohua.enabled=false); absent means enabled (Config.Enabled
// defaults true). The keyed bean capabilities (identity/i18n/redis) each arm on
// their own spring.luohua.<cap> prefix, so they consult this here — otherwise
// spring.luohua.enabled=false would silence only the apply() re-basing while the
// capability beans still assembled, contradicting the master switch's documented
// "apply none of the luohua baseline" contract.
func disabled(p flatten.Storage) (bool, error) {
	var e struct {
		Enabled bool `value:"${enabled:=true}"`
	}
	if err := conf.Bind(p, &e, "${spring.luohua:=}"); err != nil {
		return false, err
	}
	return !e.Enabled, nil
}

// apply runs each enabled capability through its public seam. Order matters
// only where one capability feeds another (propagation before observability,
// which reads the same context fields).
func apply(r gs.BeanProvider, c Config) error {
	// The load-test marker is luohua's convention when the config re-bases it:
	// it is contributed as the process's traffic.Propagator bean, which the
	// server / messaging / client starters inject. Registered unconditionally
	// when the key is set — that key IS the application's declaration — so an
	// application that also provides its own bean gets a duplicate-bean wiring
	// error instead of one of the two silently winning.
	p, err := loadTestPropagator(c.Propagate)
	if err != nil {
		return err
	}
	if p != nil {
		r.Provide(func() traffic.Propagator { return p }).Caller(1)
	}
	if err := applyPropagate(c.Propagate); err != nil {
		return err
	}
	return applyObservability(c.Observability)
}

// The capability registrations below are the umbrella's per-capability seams.
// Each lives in its own file for the implementation, but its registration is
// collected here so this file is the single place that shows what the starter
// contributes to the container.

// governance registers the company resilience driver as a bean named "luohua"
// (selectable by spring.governance.driver=luohua).
func init() {
	// Contributing the backend as a bean named "luohua" makes it selectable by
	// spring.governance.driver=luohua: the resilience manager is built over every
	// bean exported as resilience.Driver, keyed by bean name. Like the bundled
	// "default" (and sentinel's blank-import contribution) this is an init-time
	// availability registration, not a config-gated activation — the driver only
	// governs when the process actually selects it.
	gs.Provide(func(c resilience.Counters) *luohuaResilienceDriver {
		return &luohuaResilienceDriver{counters: c}
	}, gs.TagArg("?")).Name(LuohuaResilienceDriverName).
		Export(gs.As[resilience.Driver]()).
		Caller(1)
}

// identity registers the SSO token validator default (step-aside via OnMissingBean).
func init() {
	// Armed by any spring.luohua.identity.* key; the secret is required, so a
	// half-configured identity fails startup loudly instead of silently issuing
	// nothing.
	gs.Module(gs.OnProperty("spring.luohua.identity"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		var c IdentityConfig
		if err := conf.Bind(p, &c, "${spring.luohua.identity:=}"); err != nil {
			return err
		}
		sso := NewLuohuaSSO(c.Secret, c.Issuer)
		// This is a default: when an application provides its own
		// security.TokenValidator (e.g. a JWT/OIDC verifier), OnMissingBean steps
		// luohua's LuohuaSSO aside rather than competing — the same step-aside the
		// i18n default performs. Without it two TokenValidator beans make the
		// container's single-value autowire ambiguous and startup fails.
		r.Provide(func() *LuohuaSSO { return sso }).
			Condition(gs.OnMissingBean[security.TokenValidator]()).
			Export(gs.As[security.TokenValidator]()).Caller(1)
		return nil
	})
}

// i18n registers the message catalog default (step-aside via OnMissingBean).
func init() {
	// Armed by any spring.luohua.i18n.* key. The catalog is a default: if the
	// application registers its own i18n.MessageSource, OnMissingBean steps
	// luohua's aside rather than competing.
	gs.Module(gs.OnProperty("spring.luohua.i18n"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		var c I18nConfig
		if err := conf.Bind(p, &c, "${spring.luohua.i18n:=}"); err != nil {
			return err
		}
		r.Provide(func() *i18n.MapSource { return luohuaCatalog(c.DefaultLocale) }).
			Condition(gs.OnMissingBean[i18n.MessageSource]()).
			Export(gs.As[i18n.MessageSource]()).Caller(1)
		return nil
	})
}

// loadbalance registers the company balancer as a named Factory bean.
func init() {
	// Contributing the company Balancer as a NAMED Factory bean is the whole
	// extension — no change to cloud/loadbalance. The bean name IS the strategy
	// name a selection rule cites, and the Export is what makes the bean visible
	// to the directory the manager is built over.
	gs.Provide(func() loadbalance.Factory { return luohuaFactory{} }).
		Name(luohuaStrategy).
		Export(gs.As[loadbalance.Factory]()).Caller(1)
}

// lock registers the locker default (step-aside via OnMissingBean).
func init() {
	// Armed by spring.luohua.lock=true (OnProperty is a prefix check), gated by
	// the master Enabled. The default rides the same OnMissingBean step-aside as
	// identity/i18n: consumers autowire lock.Locker and get luohua's baseline
	// only while no backend starter provides its own bean.
	gs.Module(gs.OnProperty("spring.luohua.lock"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		r.Provide(lockDefault).
			Condition(gs.OnMissingBean[lock.Locker]()).
			Export(gs.As[lock.Locker]()).Caller(1)
		return nil
	})
}

// memory_cache registers the in-process cache under the luohua driver name.
func init() {
	gs.Provide(func() *cache.Cache { return NewLuohuaCache() }).Name("luohua")
}

// propagate registers luohua's wire vocabulary into the otel trace propagator.
func init() {
	trace.RegisterPropagator(propagatorName, luohuaPropagator{})
}

// redigo registers the luohua pool-assembly driver for the redis starter.
func init() {
	gs.Provide(func(c RedisConfig) StarterRedigo.Driver {
		return RedisDriver{tag: c.Tag}
	}, gs.TagArg("${spring.luohua.redis}")).
		Condition(gs.OnProperty("spring.luohua.redis"), gs.Not(gs.OnProperty("spring.luohua.enabled").HavingValue("false"))).
		Caller(1)
}
