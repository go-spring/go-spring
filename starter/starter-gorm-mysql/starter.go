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
	"go-spring.org/spring/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.mysql}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.mysql}", newClient, nil)
}

// newClient creates a GORM database client using the MySQL driver, bridged into
// go-spring's unified observability. The otel plugin emits client spans and
// connection-pool metrics through the OTel globals that starter-otel installs;
// when starter-otel is absent those globals are no-ops, so this stays a
// zero-config, zero-overhead opt-in that needs no per-component adaptation.
func newClient(c Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(c.DSN()))
	if err != nil {
		return nil, err
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("mysql"))); err != nil {
		return nil, err
	}
	return db, nil
}
