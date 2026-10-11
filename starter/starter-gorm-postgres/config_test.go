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

package StarterGormPostgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestDSN pins the space-separated key=value PostgreSQL DSN: base fields in a
// fixed order, then the optional SSL material, timezone and connect timeout.
func TestDSN(t *testing.T) {
	c := Config{
		Host: "127.0.0.1", Port: "5432", User: "postgres",
		Password: "secret", DB: "app", SSLMode: "disable",
	}
	got := c.DSN()
	want := "host=127.0.0.1 port=5432 user=postgres password=secret dbname=app sslmode=disable"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	c.SSLRootCert, c.SSLCert, c.SSLKey = "/ca.pem", "/cert.pem", "/key.pem"
	c.TimeZone = "Asia/Shanghai"
	c.ConnectTimeout = 3 * time.Second
	got = c.DSN()
	for _, part := range []string{
		"sslrootcert=/ca.pem", "sslcert=/cert.pem", "sslkey=/key.pem",
		"TimeZone=Asia/Shanghai", "connect_timeout=3",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("dsn %q must contain %q", got, part)
		}
	}
}

// TestBuildValidation proves the required-address rule: with neither host nor
// service-name the build fails instead of producing an empty-DSN dialector.
func TestBuildValidation(t *testing.T) {
	if _, err := build(context.Background(), Config{User: "u", Password: "p", DB: "test"}, nil); err == nil {
		t.Fatal("build must require host or service-name")
	}
}

// TestDSNConnectTimeoutSubSecond proves sub-second connectTimeout rounds up to
// 1s instead of truncating to 0 (which pgx reads as "no timeout").
func TestDSNConnectTimeoutSubSecond(t *testing.T) {
	c := Config{Host: "h", Port: "5432", User: "u", Password: "p", DB: "d", SSLMode: "disable"}
	c.ConnectTimeout = 500 * time.Millisecond
	if got := c.DSN(); !strings.Contains(got, "connect_timeout=1") {
		t.Fatalf("dsn %q must round 500ms up to connect_timeout=1", got)
	}
	c.ConnectTimeout = 2500 * time.Millisecond
	if got := c.DSN(); !strings.Contains(got, "connect_timeout=3") {
		t.Fatalf("dsn %q must round 2500ms up to connect_timeout=3", got)
	}
}

// TestBuildTLSConflict proves tls.enabled with sslmode=disable fails loudly at
// build time instead of silently dialing plaintext.
func TestBuildTLSConflict(t *testing.T) {
	c := Config{Host: "h", Port: "5432", User: "u", Password: "p", DB: "d", SSLMode: "disable"}
	c.TLS.Enabled = true
	if _, err := build(context.Background(), c, nil); err == nil || !strings.Contains(err.Error(), "sslmode=disable") {
		t.Fatalf("tls.enabled + sslmode=disable must fail loudly, got %v", err)
	}
}
