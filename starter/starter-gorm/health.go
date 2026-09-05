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
	"gorm.io/gorm"
)

// NewGormHealth builds an indicator for a GORM (*gorm.DB) client. It is
// registered once per configured instance and exported as health.Indicator, so
// an application that also imports starter-actuator gets the database folded
// into /readiness with no extra wiring. prefix is the dialect label (e.g.
// "gorm:mysql:", "gorm:postgres:").
func NewGormHealth(prefix, name string, db *gorm.DB) *health.Indicator {
	return &health.Indicator{Name: prefix + name, Probe: func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}}
}
