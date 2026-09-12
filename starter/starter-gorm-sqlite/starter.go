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

// Package StarterGormSqlite is the gorm+sqlite dialect starter. It registers
// one gorm client per entry under "spring.gorm.sqlite", with the shared
// open/pool/observe/resilience scaffolding provided by
// go-spring.org/starter-gorm. Only the SQLite-specific piece — the Config +
// DSN — lives here; SQLite is an in-process database, so there is no TLS or
// service discovery to wire.
package StarterGormSqlite

import (
	"context"

	gormsqlite "github.com/glebarez/sqlite"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/starter-gorm"
)

func init() {
	gormcore.Module(gormcore.Dialect[Config]{
		Prefix:       "spring.gorm.sqlite",
		BeanPrefix:   "sqlite",
		Engine:       "sqlite",
		HealthPrefix: "gorm:sqlite:",
		Build:        build,
	})
}

// build constructs the driver-specific dialector for a Config. SQLite needs no
// TLS registration (no transport) and no discovery dialer (the "server" is a
// file path), so the Spec carries only the dialector and pool settings and the
// backend argument — required by the shared Dialect.Build shape — is ignored.
func build(ctx context.Context, c Config, _ discovery.Discovery) (gormcore.Spec, error) {
	return gormcore.Spec{
		Dialector:      gormsqlite.Open(c.DSN()),
		Pool:           c.Pool(),
		Resource:       resilience.ResourceLabel("gorm:sqlite", c.File),
		ObserveEnabled: c.ObserveEnabled,
	}, nil
}
