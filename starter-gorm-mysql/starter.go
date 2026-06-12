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
	"github.com/go-spring/spring-core/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	// Register a single default GORM client.
	// This client will only be created if the property "spring.gorm.addr" is set.
	// It uses the configuration tagged with "${spring.gorm}" and is named "__default__".
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.addr")).
		Name("__default__")

	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.instances}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.instances}", newClient, nil)
}

// newClient creates a GORM database client using the MySQL driver.
func newClient(c Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(c.DSN()))
}
