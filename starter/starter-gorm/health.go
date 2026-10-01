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

package gormcore

import (
	"context"

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck is the module's single connectivity probe: it verifies the pool
// behind db can reach the database. It goes straight to the raw pool on purpose
// — a readiness check must reflect the backend, not the rate limiter, and must
// not feed the observe plugin or the breaker's statistics — so it delegates to
// [Ping] on the embedded *gorm.DB, which pings the underlying *sql.DB with no
// gorm callbacks in the path. The startup probe in [Module] and the actuator
// probe in [NewClientHealth] both delegate here, so there is exactly one
// implementation.
func HealthCheck(ctx context.Context, db *DB) error {
	return Ping(ctx, db.DB)
}

// NewClientHealth builds an indicator for a GORM (*DB) client. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets the database folded into
// /readiness with no extra wiring. prefix is the dialect label (e.g.
// "gorm:mysql:", "gorm:postgres:"). The probe only calls [HealthCheck], so it
// shares the module's one connectivity check.
func NewClientHealth(prefix, name string, db *DB) *health.Indicator {
	return &health.Indicator{Name: prefix + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, db)
	}}
}
