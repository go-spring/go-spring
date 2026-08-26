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

package StarterGormMySql

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

// No MySQL server is reachable from unit tests, so these tests pin the driver
// assembly: DSN construction, the build validation/TLS paths, fail-fast open
// against a closed port (connection error, not a hang or panic), and the
// conditional bean registration.

func TestDSN(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		c := Config{User: "u", Password: "p", Addr: "127.0.0.1:3306", DB: "test"}
		got := c.DSN()
		want := "u:p@tcp(127.0.0.1:3306)/test"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("explicit network", func(t *testing.T) {
		c := Config{User: "u", Password: "p", Network: "unix", Addr: "/tmp/mysql.sock", DB: "test"}
		if got := c.DSN(); !strings.HasPrefix(got, "u:p@unix(/tmp/mysql.sock)/test") {
			t.Fatalf("unix network must be honored: %q", got)
		}
	})

	t.Run("all options", func(t *testing.T) {
		c := Config{
			User: "u", Password: "p", Addr: "h:3306", DB: "test",
			Charset: "utf8mb4", ParseTime: true, Location: "Asia/Shanghai",
			Timeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second,
		}
		c.tlsParam = "gstls_1"
		for _, want := range []string{
			"charset=utf8mb4", "parseTime=true", "loc=Asia%2FShanghai",
			"timeout=1s", "readTimeout=2s", "writeTimeout=3s", "tls=gstls_1",
		} {
			if !strings.Contains(c.DSN(), want) {
				t.Fatalf("dsn %q must contain %q", c.DSN(), want)
			}
		}
	})
}

func TestBuildValidation(t *testing.T) {
	// Neither addr nor service-name: must be rejected before anything registers.
	if _, err := build(context.Background(), Config{User: "u", Password: "p", DB: "test"}); err == nil {
		t.Fatal("build must require addr or service-name")
	}

	// A broken TLS config must fail the build, not leak into a bad DSN.
	c := Config{User: "u", Password: "p", Addr: "127.0.0.1:3306", DB: "test"}
	c.TLS.Enabled = true
	c.TLS.CAFile = "/nonexistent/ca.pem"
	if _, err := build(context.Background(), c); err == nil {
		t.Fatal("build must fail on an unreadable CA file")
	}
}

func TestBuildPlainSpec(t *testing.T) {
	c := Config{User: "u", Password: "p", Addr: "127.0.0.1:3306", DB: "test"}
	c.PingTimeout = 500 * time.Millisecond

	spec, err := build(context.Background(), c)
	assert.Error(t, err).Nil("build")
	if spec.Dialector == nil {
		t.Fatal("plain-addr build must return a dialector")
	}
	if spec.Pool.PingTimeout != 500*time.Millisecond {
		t.Fatalf("pool settings must flow from Config to Spec: %+v", spec.Pool)
	}

	// Port 1 on loopback is closed: the open must fail fast with a connection
	// error from the shared open path, returning a nil client.
	client, err := gormcore.Open(spec.Dialector, spec.Pool, gormcore.Options{Engine: "mysql", Resource: spec.Resource})
	if err == nil {
		_ = client.Destroy()
		t.Fatal("expected a connection error for a closed port")
	}
	if client != nil {
		t.Fatalf("expected nil client on failure, got %v", client)
	}
}

// TestMysqlNotTriggered proves the conditional wiring: with no
// spring.gorm.mysql.* entries the starter registers nothing and the app starts.
func TestMysqlNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*DB              `autowire:""`
		Inds []health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 || len(s.Inds) != 0 {
			t.Fatalf("starter must stay dormant without config, got %d DB / %d indicators", len(s.DBs), len(s.Inds))
		}
	})
}
