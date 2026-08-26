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
	if _, err := build(context.Background(), Config{User: "u", Password: "p", DB: "test"}); err == nil {
		t.Fatal("build must require host or service-name")
	}
}
