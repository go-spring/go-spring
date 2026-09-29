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

package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Cassandra clients as a group, one per entry under
	// "${spring.cassandra}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.cassandra.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.cassandra.instances}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so the ctor arms
			// it (ArmGovernance, with the injected governance beans) and
			// Destroy tears it down. The Driver bean is
			// selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when one is provided,
			// nil otherwise, and the ctor falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.cassandra.instances."+name+".driver:=${spring.cassandra.default.driver:=?}}")),
				// The governance beans are NULLABLE injections: they exist
				// whenever starter-governance is in the container, which is the
				// normal case, and are absent from a container without it. Without
				// the "?" gs would treat an absent bean as a wiring error and the
				// app would not boot — turning "governance is off" into "governance
				// must be imported", which is not the contract. ArmGovernance
				// treats a nil bean as an unarmed authority, i.e. a transparent
				// pass-through.
				gs.IndexArg(3, gs.TagArg("?")), // mgr *resilience.Manager
				gs.IndexArg(4, gs.TagArg("?")), // inj *fault.Injector
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) *health.Indicator {
				return NewClientHealth(name, w.Session)
			}, gs.TagArg(name)).Name("cassandra:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new Cassandra client based on the provided
// configuration. The cluster is probed once at startup so that
// misconfiguration or an unreachable cluster fails fast rather than on first
// use.
//
// mgr and inj are the governance beans starter-governance provides. The wiring
// injects them NULLABLY, so both are nil in a container without
// starter-governance as well as in a standalone (non-gs) call;
// [Client.ArmGovernance] treats a nil bean as "governance off".
func newClient(ctx *gs.ContextProvider, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating cassandra client, hosts=%v keyspace=%s", c.Hosts, c.Keyspace)

	if (c.Username == "") != (c.Password == "") {
		return nil, errutil.Explain(nil, "cassandra username and password must be set together")
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	session, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	if err = HealthCheck(ctx.Context, session); err != nil {
		session.Close()
		return nil, errutil.Explain(err, "failed to reach cassandra cluster %v", c.Hosts)
	}
	w := &Client{Session: session, cfg: c}
	// Arm governance on the wrapper. It runs here, not inside the driver, so a
	// custom Driver's client is governed too — without the Driver interface
	// carrying a dependency on cloud/governance.
	if err := w.ArmGovernance(mgr, inj); err != nil {
		session.Close()
		return nil, err
	}
	return w, nil
}

// HealthCheck reports whether the Cassandra cluster answers a trivial query.
// It is a thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, session *gocql.Session) error {
	var release string
	return session.Query("SELECT release_version FROM system.local").WithContext(ctx).Scan(&release)
}
