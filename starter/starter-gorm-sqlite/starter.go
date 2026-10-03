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

// Package StarterGormSqlite is the gorm+sqlite dialect starter. It registers one
// gorm client per entry under "spring.gorm.sqlite" — the gs.Module block below
// is that registration, written out here so a reader sees the beans this
// starter contributes — with the shared open/pool/observe/resilience scaffolding
// provided by go-spring.org/starter-gorm. The SQLite-specific piece — the
// Config + DSN — lives here; SQLite is an in-process database, so there is no
// TLS or service discovery to wire.
package StarterGormSqlite

import (
	"context"

	gormsqlite "github.com/glebarez/sqlite"
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-gorm"
	"go-spring.org/stdlib/flatten"
)

// The registration below is written out per dialect starter rather than shared
// through a helper, so a reader of this file sees exactly which beans it
// contributes: one *DB named "sqlite.<entry>" plus a paired health.Indicator, per
// entry under spring.gorm.sqlite.instances. The construction those beans run —
// build → [gormcore.NewDB] (open, observe, governance, startup ping) — is shared
// in starter-gorm.
func init() {
	gs.Module(gs.OnProperty("spring.gorm.sqlite.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an entry does not define falls back to the family-wide
		// "default" bucket: spring.gorm.sqlite.default.<k> is the value every entry
		// inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.gorm.sqlite.instances", "spring.gorm.sqlite.default")
		return conf.BindEach(p, "${spring.gorm.sqlite.instances}", func(name string, c Config) error {
			// The dialect qualifier keeps sqlite's instances in their own bean-name
			// space, so two dialects may carry an instance of the same name.
			beanName := "sqlite." + name
			r.Provide(func(ctx *gs.ContextProvider, discoveryLabel string, center *governance.Center) (*gormcore.DB, error) {
				// SQLite has no network transport, so the discovery label resolves
				// to nothing and the dialector dials the configured file. The center
				// is still the family's sole injection point: it hands out the
				// resilience/fault/loadbalance authorities, bundled into the one
				// ClientParams the dialect and NewDB both read.
				disc, _ := center.Discovery().Get(discoveryLabel)
				params := cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault(), Loadbalance: center.Loadbalance(), Discovery: disc}
				spec, err := build(ctx.Context, c, params)
				if err != nil {
					return nil, err
				}
				return gormcore.NewDB(ctx.Context, "sqlite", spec, params, c.Ping)
			},
				gs.IndexArg(1, gs.TagArg("${spring.gorm.sqlite.instances."+name+".discovery:=${spring.gorm.sqlite.default.discovery:=none}}")),
			).Name(beanName).Destroy((*gormcore.DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the bean just registered above
			// by name.
			if c.Health {
				r.Provide(func(w *gormcore.DB) *health.Indicator {
					return gormcore.NewClientHealth("gorm:sqlite:", name, w)
				}, gs.TagArg(beanName)).Name("gorm:sqlite:" + name).Caller(1)
			}
			return nil
		})
	})
}

// build constructs the driver-specific dialector for a Config. SQLite needs no
// TLS registration (no transport) and no discovery dialer (the "server" is a
// file path), so the Spec carries only the dialector and pool settings; the
// params argument — shared by every dialect's build signature so the wiring can
// hand them all the same bundle — is ignored, since there is no candidate set to
// select from.
func build(ctx context.Context, c Config, _ cloud.ClientParams) (gormcore.Spec, error) {
	return gormcore.Spec{
		Dialector:      gormsqlite.Open(c.DSN()),
		Pool:           c.Pool(),
		Service:        resilience.ServiceLabel("gorm:sqlite", c.File),
		ObserveEnabled: c.ObserveEnabled,
	}, nil
}
