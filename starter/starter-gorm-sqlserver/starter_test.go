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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/go-mssqldb/msdsn"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/gs"
	gormcore "go-spring.org/starter-gorm"
	"go-spring.org/stdlib/testing/assert"
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
		c.TLS.ServerName = "db.internal"
		got := c.DSN()
		for _, want := range []string{
			"dial+timeout=2", "connection+timeout=5",
			"encrypt=true", "TrustServerCertificate=true", "certificate=%2Fca.pem",
			"hostNameInCertificate=db.internal",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("dsn %q must contain %q", got, want)
			}
		}
	})

	// Every TLS key is nested under enabled: with TLS off no TLS parameter may
	// leak into the DSN, server-name included.
	t.Run("tls keys stay under enabled", func(t *testing.T) {
		c := Config{User: "sa", Password: "p", Host: "h", Port: "1433", DB: "master"}
		c.TLS.CAFile = "/ca.pem"
		c.TLS.ServerName = "db.internal"
		got := c.DSN()
		for _, unwanted := range []string{"encrypt", "certificate", "hostNameInCertificate"} {
			if strings.Contains(got, unwanted) {
				t.Fatalf("dsn %q must not contain %q while tls.enabled=false", got, unwanted)
			}
		}
	})
}

// TestDSNTLSLandsOnTheDriver pins the TLS keys against the driver's own parser
// rather than against the DSN string: msdsn ignores parameters it does not
// know, so a misspelled or unbound key is indistinguishable from "the DSN
// cannot express it" unless the parse result is asserted. In particular
// hostNameInCertificate must set HostInCertificateProvided, or the driver
// overwrites ServerName with the (possibly dummy) host at dial time — the
// discovery case, where the resolved address is not the certificate's name.
func TestDSNTLSLandsOnTheDriver(t *testing.T) {
	// Any .pem path reads back fine: the driver only reads the bytes and feeds
	// them to AppendCertsFromPEM, which is content-agnostic here.
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := Config{User: "sa", Password: "p", Host: "0.0.0.0", Port: "0", DB: "master"}
	c.TLS.Enabled = true
	c.TLS.CAFile = caFile
	c.TLS.ServerName = "db.internal"

	cfg, err := msdsn.Parse(c.DSN())
	assert.Error(t, err).Nil("msdsn.Parse")
	if cfg.TLSConfig == nil {
		t.Fatal("tls.enabled=true must yield a TLSConfig from the driver's parser")
	}
	if cfg.TLSConfig.ServerName != "db.internal" {
		t.Fatalf("tls.server-name must reach tls.Config.ServerName, got %q", cfg.TLSConfig.ServerName)
	}
	if !cfg.HostInCertificateProvided {
		t.Fatal("tls.server-name must set HostInCertificateProvided, else the driver rewrites ServerName to the host")
	}
	if cfg.TLSConfig.RootCAs == nil {
		t.Fatal("tls.ca-file must populate RootCAs")
	}
}

func TestBuild(t *testing.T) {
	// Neither host nor service-name: rejected up front.
	if _, err := build(context.Background(), Config{User: "sa", Password: "p", DB: "master"}, nil); err == nil {
		t.Fatal("build must require host or service-name")
	}

	c := Config{User: "sa", Password: "p", Host: "127.0.0.1", Port: "1", DB: "master"}
	c.PingTimeout = 500 * time.Millisecond
	spec, err := build(context.Background(), c, nil)
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
// spring.gorm.sqlserver.instances.* entries the starter registers nothing and the app starts.
func TestSqlserverNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*gormcore.DB      `autowire:""`
		Inds []*health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 || len(s.Inds) != 0 {
			t.Fatalf("starter must stay dormant without config, got %d DB / %d indicators", len(s.DBs), len(s.Inds))
		}
	})
}

// TestDSNTimeoutSubSecond proves sub-second dial/connect timeouts round up to
// 1s instead of truncating to 0 (which the driver reads as "no timeout").
func TestDSNTimeoutSubSecond(t *testing.T) {
	c := Config{User: "u", Password: "p", Host: "h", Port: "1433", DB: "d"}
	c.DialTimeout = 300 * time.Millisecond
	c.ConnectTimeout = 500 * time.Millisecond
	got := c.DSN()
	if !strings.Contains(got, "dial+timeout=1") {
		t.Fatalf("dsn %q must round 300ms up to dial+timeout=1", got)
	}
	if !strings.Contains(got, "connection+timeout=1") {
		t.Fatalf("dsn %q must round 500ms up to connection+timeout=1", got)
	}
}
