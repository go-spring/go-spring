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

// Package gormcore is the shared scaffolding behind every go-spring gorm
// dialect starter (mysql, postgres, clickhouse, sqlserver). Each driver starter
// owns its dialect-specific Config + DSN + TLS/discovery dialing and hands the
// driver-agnostic remainder — connection-pool tuning, the open/ping/customize
// sequence, the DB wrapper + its observe/resilience lifecycle, the health
// indicator, and the post-open extension seam — to this package, so the four
// starters share one implementation instead of copy-pasting it each.
package gormcore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PoolConfig carries the driver-agnostic connection-pool and logging settings
// shared by every gorm dialect starter. Each driver Config holds these same
// fields (via [Common]) and copies them into a PoolConfig before calling [Open].
type PoolConfig struct {
	MaxOpenConns    int           // Max open connections (0 = unlimited)
	MaxIdleConns    int           // Max idle connections (0 = default 2)
	ConnMaxLifetime time.Duration // Max lifetime of a connection (0 = unlimited)
	ConnMaxIdleTime time.Duration // Max idle time of a connection (0 = unlimited)
	PingTimeout     time.Duration // Startup connectivity-check bound (0 = 5s)
	SlowThreshold   time.Duration // GORM slow-query log threshold (0 = off)
}

// PoolSettings is the transport-agnostic half of the shared config block:
// connection-pool tuning, the startup ping bound and the slow-query log
// threshold. Every dialect starter binds these keys, including in-process
// engines like sqlite that have no network transport at all.
type PoolSettings struct {
	// Connection pool tuning. A zero value leaves the database/sql default in
	// place (see sql.DB.SetMaxOpenConns and friends).
	MaxOpenConns    int           `value:"${max-open-conns:=0}"`     // Max open connections (0 = unlimited)
	MaxIdleConns    int           `value:"${max-idle-conns:=0}"`     // Max idle connections (0 = default 2)
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=0}"`  // Max lifetime of a connection (0 = unlimited)
	ConnMaxIdleTime time.Duration `value:"${conn-max-idle-time:=0}"` // Max idle time of a connection (0 = unlimited)

	// PingTimeout bounds the startup connectivity check. The client fails fast
	// during creation if the server cannot be reached within this window.
	PingTimeout time.Duration `value:"${ping-timeout:=5s}"`

	// SlowThreshold enables GORM slow-query logging when > 0: queries slower than
	// this are logged at warn level.
	SlowThreshold time.Duration `value:"${slow-threshold:=0}"`
}

// Common is the config block every network gorm dialect starter shares:
// PoolSettings plus service-discovery routing and the observe kill switch.
// A dialect's Config embeds it (anonymously, so conf binds these fields at the
// same level) alongside its own connection-specific fields. Starters with no
// network transport (e.g. sqlite) embed only [PoolSettings] instead.
type Common struct {
	PoolSettings

	// ServiceName is the service discovery name. When set, the connection dials a
	// live instance resolved from the discovery backend instead of the configured
	// address.
	ServiceName string `value:"${service-name:=}"`
	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`

	// ObserveEnabled is the hard per-instance kill switch for the gorm observe
	// plugin (trace span + metric + access log on every Create/Query/Update/
	// Delete). Defaults to true; when false the plugin is not installed at all,
	// so no per-query callbacks run — for high-throughput instances where the
	// instrumentation overhead is unwanted.
	ObserveEnabled bool `value:"${observe.enabled:=true}"`
}

// Pool extracts the connection-pool and logging settings into a PoolConfig.
func (c PoolSettings) Pool() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
		PingTimeout:     c.PingTimeout,
		SlowThreshold:   c.SlowThreshold,
	}
}

// NewResolver resolves the registered discovery backend for this config into a
// by-name resolver that re-reads the service's live endpoint snapshot. It returns
// (nil, nil) when ServiceName is unset or mesh mode is enabled (a sidecar owns
// discovery+LB), in which case the caller dials the configured address
// directly. Freshness lives inside the backend, so there is nothing to release.
func (c Common) NewResolver(ctx context.Context) (discovery.Resolver, error) {
	return discovery.NewResolver(ctx, c.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
}

// NewPickPool builds the shared per-connection endpoint selector over the
// resolver from [Common.NewResolver]: a round-robin loadbalance.Pool the dialect
// starter's DialContext calls on every new connection. It returns
// (nil, nil) when discovery is not in effect; otherwise the pool (and its
// source resolver, which has no resources to release).
func (c Common) NewPickPool(ctx context.Context) (*loadbalance.Pool, discovery.Resolver, error) {
	resolver, err := c.NewResolver(ctx)
	if err != nil || resolver == nil {
		return nil, resolver, err
	}
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	if err != nil {
		return nil, nil, err
	}
	return loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal), resolver, nil
}

// logWriter adapts GORM's logger onto the repo log core: every message GORM
// writes (slow queries at warn level, errors) is emitted through log.Warnf
// with the default application tag, so slow-query output lands in the
// configured appenders instead of raw stdout.
type logWriter struct{}

// Printf implements gorm logger.Writer. GORM calls it once per message with a
// trailing newline; the newline is stripped before re-logging.
func (logWriter) Printf(format string, args ...any) {
	if msg := strings.TrimRight(fmt.Sprintf(format, args...), "\r\n"); msg != "" {
		log.Warnf(context.Background(), log.TagAppDef, "%s", msg)
	}
}

// GormConfig builds the *gorm.Config for a client. When SlowThreshold is set,
// GORM's logger reports queries slower than the threshold at warn level
// through go-spring.org/log (TagAppDef), not the Go standard library.
func GormConfig(pool PoolConfig) *gorm.Config {
	cfg := &gorm.Config{}
	if pool.SlowThreshold > 0 {
		cfg.Logger = logger.New(
			logWriter{},
			logger.Config{
				SlowThreshold: pool.SlowThreshold,
				LogLevel:      logger.Warn,
				Colorful:      false,
			},
		)
	}
	return cfg
}

// ApplyPool applies connection-pool settings and performs a startup ping so
// misconfigured address/credentials fail fast at creation instead of on first
// query.
func ApplyPool(db *gorm.DB, pool PoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if pool.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
	if pool.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	}
	timeout := pool.PingTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

// Ping verifies the connection pool behind db can reach the database. It is a
// readiness/health-check hook usable by callers or an external checker.
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Stats exposes the runtime connection-pool statistics (InUse, Idle,
// WaitCount, ...) behind db without requiring OpenTelemetry.
func Stats(db *gorm.DB) (sql.DBStats, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}
	return sqlDB.Stats(), nil
}
