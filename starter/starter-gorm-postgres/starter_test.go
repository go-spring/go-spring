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
	"testing"
	"time"

	gormcore "go-spring.org/starter-gorm"
)

// TestBuildPlainDSN is the regression test for the plain-DSN (non-discovery)
// open path. Previously newClient only opened the gorm.DB inside the discovery
// branch, leaving db nil in plain-DSN mode and panicking when applying the pool
// to a nil receiver. With the fix the plain-DSN branch builds its own dialector,
// so an unreachable postgres must surface as a normal connection error from the
// shared open rather than a nil-pointer panic.
func TestBuildPlainDSN(t *testing.T) {
	c := Config{
		// Port 1 on loopback is closed, so the open + startup ping fails fast.
		Host:     "127.0.0.1",
		Port:     "1",
		User:     "nobody",
		Password: "nope",
		DB:       "nodb",
		SSLMode:  "disable",
	}
	c.PingTimeout = 500 * time.Millisecond

	spec, err := build(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if spec.Dialector == nil {
		t.Fatal("expected a non-nil dialector in plain-DSN mode")
	}

	client, err := gormcore.Open(spec.Dialector, spec.Pool, gormcore.Options{
		Engine:         "postgresql",
		Resource:       spec.Resource,
		ObserveEnabled: spec.ObserveEnabled,
		Closers:        spec.Closers,
	})
	if err == nil {
		_ = client.Destroy()
		t.Fatal("expected a connection error for unreachable postgres, got nil (plain-DSN must not panic)")
	}
	if client != nil {
		t.Fatalf("expected nil client on failure, got %v", client)
	}
}
