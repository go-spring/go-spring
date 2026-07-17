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
	"go-spring.org/spring/gs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.postgres}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.postgres}", newClient, nil)
}

// newClient creates a GORM database client using the PostgreSQL driver.
func newClient(c Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(c.DSN()))
}
