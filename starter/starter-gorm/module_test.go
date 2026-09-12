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
	"testing"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
)

// fakeCfg is the config of a make-believe dialect used to drive Module's
// multi-instance assembly through a real gs container without needing a real
// database server.
type fakeCfg struct {
	File string `value:"${file}"`
}

func init() {
	Module(Dialect[fakeCfg]{
		Prefix:       "spring.gorm.fake",
		Engine:       "fake",
		HealthPrefix: "gorm:fake:",
		Build: func(ctx context.Context, c fakeCfg, _ discovery.Discovery) (Spec, error) {
			return Spec{
				Dialector:      fakeDialector{},
				Pool:           PoolConfig{PingTimeout: time.Second},
				Resource:       "gorm:fake:" + c.File,
				ObserveEnabled: false,
			}, nil
		},
	})

	// fake2Cfg drives a second dialect that reuses the instance names of
	// [fakeCfg], proving the dialect qualifier keeps their beans apart.
	Module(Dialect[fakeCfg]{
		Prefix:       "spring.gorm.fake2",
		BeanPrefix:   "fake2",
		Engine:       "fake2",
		HealthPrefix: "gorm:fake2:",
		Build: func(ctx context.Context, c fakeCfg, _ discovery.Discovery) (Spec, error) {
			return Spec{
				Dialector:      fakeDialector{},
				Pool:           PoolConfig{PingTimeout: time.Second},
				Resource:       "gorm:fake2:" + c.File,
				ObserveEnabled: false,
			}, nil
		},
	})
}

// TestRegisterMultiInstance pins the BindEach assembly: one *DB bean plus one
// health.Indicator per entry under the prefix, each opened through the shared
// chain and torn down on shutdown.
func TestRegisterMultiInstance(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.gorm.fake.instances.orders.file", "orders.db")
		app.Property("spring.gorm.fake.instances.audit.file", "audit.db")
	}).RunTest(t, func(s *struct {
		DBs  []*DB               `autowire:""`
		Inds []*health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 2 {
			t.Fatalf("want 2 DB beans, got %d", len(s.DBs))
		}
		for _, db := range s.DBs {
			if err := Ping(context.Background(), db.DB); err != nil {
				t.Fatalf("instance %s must be open and pingable: %v", db.Name(), err)
			}
		}

		names := map[string]bool{}
		for _, ind := range s.Inds {
			names[ind.Name] = true
			if err := ind.Probe(context.Background()); err != nil {
				t.Fatalf("indicator %s must check UP: %v", ind.Name, err)
			}
		}
		if !names["gorm:fake:orders"] || !names["gorm:fake:audit"] {
			t.Fatalf("want gorm:fake:orders and gorm:fake:audit indicators, got %v", names)
		}
	})
}

// TestDialectQualifiedBeanNames pins the bean-naming contract: every DB bean is
// named "<dialect>.<instance>", taken from BeanPrefix (here explicit for the
// second dialect, derived from Prefix for the first). Two dialects sharing an
// instance name therefore register distinct beans instead of colliding.
func TestDialectQualifiedBeanNames(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.gorm.fake.instances.orders.file", "orders.db")
		app.Property("spring.gorm.fake2.instances.orders.file", "orders2.db")
	}).RunTest(t, func(s *struct {
		Fake  *DB `autowire:"fake.orders"`
		Fake2 *DB `autowire:"fake2.orders"`
	}) {
		if s.Fake == nil || s.Fake2 == nil {
			t.Fatalf("both dialects' \"orders\" beans must resolve by qualified name, got %v, %v", s.Fake, s.Fake2)
		}
		if s.Fake == s.Fake2 {
			t.Fatal("qualified names must yield two distinct DB beans")
		}
	})
}

// TestRegisterNotTriggered proves the OnProperty guard: with no
// spring.gorm.fake.instances.* entries configured, no DB or indicator beans register (the
// container starts fine and the injections resolve empty).
func TestRegisterNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*DB               `autowire:""`
		Inds []*health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 {
			t.Fatalf("no DB beans should register without config, got %d", len(s.DBs))
		}
		if len(s.Inds) != 0 {
			t.Fatalf("no indicators should register without config, got %d", len(s.Inds))
		}
	})
}
