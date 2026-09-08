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

package gormcore

import (
	"context"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"gorm.io/gorm"
)

// discoveryBearer is implemented by every dialect Config that embeds
// [Common]: it lets the generic [Register] stamp the injected discovery
// backend bean into the config without knowing the dialect's Config type.
// Dialects with no network transport (e.g. sqlite, embedding only
// [PoolSettings]) do not implement it and skip discovery wiring entirely.
type discoveryBearer interface {
	setDiscoveryBackend(discovery.Discovery) error
}

// setDiscoveryBackend stamps the backend bean cited by the entry's
// ${discovery} label into the config and fail-louds when the entry routes
// through service discovery but the label names no backend bean — the
// container is the discovery directory.
func (c *Common) setDiscoveryBackend(d discovery.Discovery) error {
	c.backend = d
	if c.ServiceName != "" && d == nil {
		return errutil.Explain(nil, "gorm: instance cites discovery backend %q but no such bean exists (register a discovery backend bean under that name)", c.Discovery)
	}
	return nil
}

// Spec is the per-instance result of a dialect's [Dialect.Build]: everything
// [Open] needs beyond the shared label strings. Closers are run on teardown (and
// on open failure) to release driver-scoped state such as discovery watches and
// registered TLS configs.
type Spec struct {
	Dialector      gorm.Dialector
	Pool           PoolConfig
	Resource       string
	ObserveEnabled bool
	Closers        []func()
}

// Dialect describes one gorm dialect starter: its configuration prefix, its
// observability labels, and how to build the driver-specific dialector for a
// Config value.
type Dialect[C any] struct {
	Prefix       string // e.g. "spring.gorm.mysql"
	Engine       string // db.system + observe + resource label, e.g. "mysql"
	HealthPrefix string // e.g. "gorm:mysql:"
	Build        func(ctx context.Context, c C) (Spec, error)
}

// Register wires a dialect starter into the container: one *DB bean (plus a
// paired health.Indicator) per entry under Prefix, with the open/pool/customize
// sequence and the observe/resilience lifecycle shared from this package. Call
// from a dialect starter's init function.
func Register[C any](d Dialect[C]) {
	gs.Module(gs.OnProperty(d.Prefix), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${"+d.Prefix+"}", func(name string, c C) error {
			// disc is the discovery backend bean cited by this entry's
			// ${discovery} label (nil when the key is unset or the entry dials
			// a static address); it is stamped into the config inside the
			// constructor, before the dialect Build consumes it.
			r.Provide(func(ctx *gs.ContextProvider, disc discovery.Discovery) (*DB, error) {
				// No-op for dialects without [Common] (e.g. sqlite).
				if s, ok := any(&c).(discoveryBearer); ok {
					if err := s.setDiscoveryBackend(disc); err != nil {
						return nil, err
					}
				}
				spec, err := d.Build(ctx.Context, c)
				if err != nil {
					return nil, err
				}
				db, err := Open(spec.Dialector, spec.Pool, Options{
					Engine:         d.Engine,
					Resource:       spec.Resource,
					ObserveEnabled: spec.ObserveEnabled,
					Closers:        spec.Closers,
				})
				if err != nil {
					for _, closer := range spec.Closers {
						if closer != nil {
							closer()
						}
					}
					return nil, err
				}
				return db, nil
			},
				// Bind the constructor's disc parameter: the backend bean named
				// by this entry's ${discovery} key ("none" is a never-existing
				// bean name, so an unset key yields the optional nil).
				gs.IndexArg(1, gs.TagArg("${"+d.Prefix+"."+name+".discovery:=none}?")),
			).Name(name).Init((*DB).Init).Destroy((*DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance, injecting the
			// wrapper just registered above by name (the embedded *gorm.DB is
			// passed through to the indicator).
			r.Provide(func(w *DB) *health.Indicator {
				return NewGormHealth(d.HealthPrefix, name, w.DB)
			}, gs.TagArg(name)).Name(d.HealthPrefix + name).Caller(1)
			return nil
		})
	})
}
