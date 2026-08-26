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

package StarterGormClickhouse

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

// No ClickHouse server is reachable from unit tests, so these tests pin the
// driver assembly: the URL-style DSN, the build validation and TLS path, the
// native-driver branch (TLS enabled forces clickhouse.New over the plain DSN),
// fail-fast open against a closed port, and the conditional bean registration.

func TestDSN(t *testing.T) {
	c := Config{User: "default", Password: "", Addr: "127.0.0.1:9000", DB: "default"}
	if got, want := c.DSN(), "clickhouse://default:@127.0.0.1:9000/default"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	c.DialTimeout = 2 * time.Second
	c.ReadTimeout = 30 * time.Second
	got := c.DSN()
	if !strings.Contains(got, "dial_timeout=2s") || !strings.Contains(got, "read_timeout=30s") {
		t.Fatalf("dsn %q must carry the timeouts", got)
	}
}

func TestBuild(t *testing.T) {
	// Neither addr nor service-name: rejected up front.
	if _, err := build(context.Background(), Config{}); err == nil {
		t.Fatal("build must require addr or service-name")
	}

	c := Config{Addr: "127.0.0.1:9000", User: "default", DB: "default"}
	c.PingTimeout = 500 * time.Millisecond

	// Plain path: DSN dialector, pool settings flow through.
	spec, err := build(context.Background(), c)
	assert.Error(t, err).Nil("build")
	if spec.Dialector == nil || spec.Dialector.Name() != "clickhouse" {
		t.Fatalf("plain build must return a clickhouse dialector: %v", spec.Dialector)
	}
	if spec.Pool.PingTimeout != 500*time.Millisecond {
		t.Fatalf("pool settings must flow from Config to Spec: %+v", spec.Pool)
	}

	// TLS path: a broken CA must fail the build before any dial.
	c.TLS.Enabled = true
	c.TLS.CAFile = "/nonexistent/ca.pem"
	if _, err := build(context.Background(), c); err == nil {
		t.Fatal("build must fail on an unreadable CA file")
	}
	c.TLS.CAFile = ""

	// Closed port: the open must fail fast, returning a nil client.
	spec, err = build(context.Background(), c)
	assert.Error(t, err).Nil("build")
	client, err := gormcore.Open(spec.Dialector, spec.Pool, gormcore.Options{Engine: "clickhouse"})
	if err == nil {
		_ = client.Destroy()
		t.Fatal("expected a connection error for a closed port")
	}
	if client != nil {
		t.Fatalf("expected nil client on failure, got %v", client)
	}
}

// TestClickhouseNotTriggered proves the conditional wiring: with no
// spring.gorm.clickhouse.* entries the starter registers nothing and the app starts.
func TestClickhouseNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*DB              `autowire:""`
		Inds []health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 || len(s.Inds) != 0 {
			t.Fatalf("starter must stay dormant without config, got %d DB / %d indicators", len(s.DBs), len(s.Inds))
		}
	})
}
