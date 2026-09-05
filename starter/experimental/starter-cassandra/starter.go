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
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-cassandra/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Cassandra clients as a group, one per entry under
	// "${spring.cassandra}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.cassandra"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.cassandra}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so Init arms it
			// (InitMethod) and Destroy tears it down.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) *health.Indicator {
				return health2.NewClientHealth(name, w.Session)
			}, gs.TagArg(name)).Name("cassandra:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new Cassandra client based on the provided
// configuration. The cluster is probed once at startup so that
// misconfiguration or an unreachable cluster fails fast rather than on first
// use.
func newClient(ctx *gs.ContextProvider, c Config) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating cassandra client, hosts=%v keyspace=%s", c.Hosts, c.Keyspace)

	if (c.Username == "") != (c.Password == "") {
		return nil, errutil.Explain(nil, "cassandra username and password must be set together")
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "cassandra driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "cassandra driver not found: %s", c.Driver)
	}
	session, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	if err = HealthCheck(ctx.Context, session); err != nil {
		session.Close()
		return nil, errutil.Explain(err, "failed to reach cassandra cluster %v", c.Hosts)
	}
	return &Client{Session: session, cfg: c}, nil
}

// HealthCheck reports whether the Cassandra cluster answers a trivial query.
// It is a thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, session *gocql.Session) error {
	var release string
	return session.Query("SELECT release_version FROM system.local").WithContext(ctx).Scan(&release)
}
