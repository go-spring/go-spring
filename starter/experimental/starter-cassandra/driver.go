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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + the bundled DefaultDriver, which owns full session assembly
// (ClusterConfig, authenticator, consistency, TLS, timeouts).
package StarterCassandra

import (
	"context"
	"crypto/tls"

	"github.com/gocql/gocql"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Cassandra session (a *gocql.
// Session). It is an OPTIONAL CONTAINER BEAN: a company or umbrella starter
// may provide its own Driver bean (its constructor returns
// StarterCassandra.Driver); when none is present, starter-cassandra falls back
// to the bundled [DefaultDriver] inside client assembly. A custom driver is a
// bean, so it may inject the configuration/beans it needs — e.g. company
// config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.cassandra} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*gocql.Session, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new gocql.Session from the provided configuration.
// It owns full session assembly — hosts, PasswordAuthenticator, consistency
// level, timeouts, TLS — but not the startup probe or the resilience wiring,
// which are the starter's lifecycle concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*gocql.Session, error) {
	consistency, err := parseConsistency(c.Consistency)
	if err != nil {
		return nil, err
	}
	cfg := gocql.NewCluster(c.Hosts...)
	cfg.Keyspace = c.Keyspace
	cfg.Consistency = consistency
	cfg.Timeout = c.Timeout
	cfg.ConnectTimeout = c.ConnectTimeout
	cfg.CQLVersion = c.CQLVersion
	if c.Username != "" {
		cfg.Authenticator = gocql.PasswordAuthenticator{Username: c.Username, Password: c.Password}
	}
	if c.TLS.Enabled {
		cfg.SslOpts = &gocql.SslOptions{
			Config: &tls.Config{
				ServerName:         c.TLS.ServerName,
				InsecureSkipVerify: c.TLS.InsecureSkipVerify,
			},
			CertPath:               c.TLS.CertFile,
			KeyPath:                c.TLS.KeyFile,
			CaPath:                 c.TLS.CAFile,
			EnableHostVerification: !c.TLS.InsecureSkipVerify,
		}
	}
	session, err := cfg.CreateSession()
	if err != nil {
		return nil, errutil.Explain(err, "cassandra: create session failed for %v", c.Hosts)
	}
	return session, nil
}

// parseConsistency maps the config string onto gocql.Consistency.
func parseConsistency(s string) (gocql.Consistency, error) {
	switch s {
	case "", "local-quorum":
		return gocql.LocalQuorum, nil
	case "any":
		return gocql.Any, nil
	case "one":
		return gocql.One, nil
	case "two":
		return gocql.Two, nil
	case "three":
		return gocql.Three, nil
	case "quorum":
		return gocql.Quorum, nil
	case "all":
		return gocql.All, nil
	case "each-quorum":
		return gocql.EachQuorum, nil
	case "local-one":
		return gocql.LocalOne, nil
	default:
		return 0, errutil.Explain(nil, "cassandra: unknown consistency %q (want any|one|two|three|quorum|all|local-quorum|each-quorum|local-one)", s)
	}
}
