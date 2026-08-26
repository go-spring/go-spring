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

package StarterGormSqlserver

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/gs"
	gormcore "go-spring.org/starter-gorm"
)

// No SQL Server is reachable from unit tests, so these tests pin the driver
// assembly: the URL-style DSN (including the TLS parameter mapping), the build
// validation, fail-fast open against a closed port, and the conditional bean
// registration.

func TestDSN(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		c := Config{User: "sa", Password: "p@ss", Host: "127.0.0.1", Port: "1433", DB: "master"}
		got := c.DSN()
		want := "sqlserver://sa:p%40ss@127.0.0.1:1433?database=master"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("timeouts and tls", func(t *testing.T) {
		c := Config{User: "sa", Password: "p", Host: "h", Port: "1433", DB: "master",
			DialTimeout: 2 * time.Second, ConnectTimeout: 5 * time.Second}
		c.TLS.Enabled = true
		c.TLS.InsecureSkipVerify = true
		c.TLS.CAFile = "/ca.pem"
		got := c.DSN()
		for _, want := range []string{
			"dial+timeout=2", "connection+timeout=5",
			"encrypt=true", "TrustServerCertificate=true", "certificate=%2Fca.pem",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("dsn %q must contain %q", got, want)
			}
		}
	})
}

func TestBuild(t *testing.T) {
	// Neither host nor service-name: rejected up front.
	if _, err := build(context.Background(), Config{User: "sa", Password: "p", DB: "master"}); err == nil {
		t.Fatal("build must require host or service-name")
	}

	c := Config{User: "sa", Password: "p", Host: "127.0.0.1", Port: "1", DB: "master"}
	c.PingTimeout = 500 * time.Millisecond
	spec, err := build(context.Background(), c)
	assert.Error(t, err).Nil("build")
	if spec.Dialector == nil {
		t.Fatal("plain-host build must return a dialector")
	}
	if spec.Pool.PingTimeout != 500*time.Millisecond {
		t.Fatalf("pool settings must flow from Config to Spec: %+v", spec.Pool)
	}

	// Port 1 on loopback is closed: the open must fail fast (dial timeout
	// bounded by PingTimeout), returning a nil client rather than hanging.
	client, err := gormcore.Open(spec.Dialector, spec.Pool, gormcore.Options{Engine: "microsoft.sql_server"})
	if err == nil {
		_ = client.Destroy()
		t.Fatal("expected a connection error for a closed port")
	}
	if client != nil {
		t.Fatalf("expected nil client on failure, got %v", client)
	}
}

// TestSqlserverNotTriggered proves the conditional wiring: with no
// spring.gorm.sqlserver.* entries the starter registers nothing and the app starts.
func TestSqlserverNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*DB              `autowire:""`
		Inds []health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 || len(s.Inds) != 0 {
			t.Fatalf("starter must stay dormant without config, got %d DB / %d indicators", len(s.DBs), len(s.Inds))
		}
	})
}
