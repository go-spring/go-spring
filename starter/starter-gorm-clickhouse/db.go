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
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormConfig builds the *gorm.Config for a client. When SlowThreshold is set,
// GORM's logger reports queries slower than the threshold at warn level.
func gormConfig(c Config) *gorm.Config {
	cfg := &gorm.Config{}
	if c.SlowThreshold > 0 {
		cfg.Logger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: c.SlowThreshold,
				LogLevel:      logger.Warn,
				Colorful:      false,
			},
		)
	}
	return cfg
}

// applyPool applies connection-pool settings and performs a startup ping so
// misconfigured address/credentials fail fast at creation instead of on first
// query.
func applyPool(db *gorm.DB, c Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if c.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	}
	if c.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	}
	if c.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(c.ConnMaxLifetime)
	}
	if c.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(c.ConnMaxIdleTime)
	}
	timeout := c.PingTimeout
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

// buildTLSConfig assembles the *tls.Config for a secure ClickHouse connection.
// It honors TLSSkipVerify and loads CA/cert/key from the configured paths.
func buildTLSConfig(c Config) (*tls.Config, error) {
	cfg := &tls.Config{InsecureSkipVerify: c.TLSSkipVerify}
	if c.TLSCA != "" {
		pem, err := os.ReadFile(c.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("gorm clickhouse: read tls ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("gorm clickhouse: failed to append tls ca from %s", c.TLSCA)
		}
		cfg.RootCAs = pool
	}
	if c.TLSCert != "" || c.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("gorm clickhouse: load tls cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
