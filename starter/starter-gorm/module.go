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
	"strings"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
	"gorm.io/gorm"
)

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
	BeanPrefix   string // bean-name qualifier, e.g. "mysql"; defaults to the Prefix tail
	Engine       string // db.system + observe + resource label, e.g. "mysql"
	HealthPrefix string // e.g. "gorm:mysql:"
	// Build assembles the dialector for one entry. backend is the discovery
	// backend bean the entry's ${discovery} label resolved to (nil when the
	// label named no bean), handed in as an argument rather than carried on
	// Config so Config stays a pure bound value; dialects with no network
	// transport (e.g. sqlite) ignore it.
	Build func(ctx context.Context, c C, backend discovery.Discovery) (Spec, error)
}

// Module declares a dialect starter as a gs module: it wires one *DB bean (plus
// a paired health.Indicator) per entry under Prefix, with the open/pool/customize
// sequence and the observe/resilience lifecycle shared from this package. Call
// from a dialect starter's init function.
//
// Every *DB bean is named "<BeanPrefix>.<entry>". The dialect qualifier keeps
// each dialect's instances in their own bean-name space, so two dialects may
// carry an instance of the same name — without it both would register the same
// (name, *DB) pair and the container would refuse to start.
func Module[C any](d Dialect[C]) {
	beanPrefix := d.BeanPrefix
	if beanPrefix == "" {
		beanPrefix = d.Prefix[strings.LastIndex(d.Prefix, ".")+1:]
	}
	gs.Module(gs.OnProperty(d.Prefix+".instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${"+d.Prefix+".instances}", func(name string, c C) error {
			beanName := beanPrefix + "." + name
			// disc is the discovery backend bean cited by this entry's
			// ${discovery} label (nil when the key is unset or the entry dials
			// a static address); it is handed to the dialect Build, which is the
			// only consumer, and never carried on the Config value.
			r.Provide(func(ctx *gs.ContextProvider, disc discovery.Discovery) (*DB, error) {
				spec, err := d.Build(ctx.Context, c, disc)
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
				gs.IndexArg(1, gs.TagArg("${"+d.Prefix+".instances."+name+".discovery:=none}?")),
			).Name(beanName).Init((*DB).Init).Destroy((*DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance, injecting the
			// wrapper just registered above by name (the embedded *gorm.DB is
			// passed through to the indicator).
			r.Provide(func(w *DB) *health.Indicator {
				return NewGormHealth(d.HealthPrefix, name, w.DB)
			}, gs.TagArg(beanName)).Name(d.HealthPrefix + name).Caller(1)
			return nil
		})
	})
}
