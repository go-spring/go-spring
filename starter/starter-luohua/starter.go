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

	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

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
