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

package StarterTransactionSagaGorm

// gormConfig binds ${spring.transaction.saga.gorm}. It configures the durable,
// gorm-backed saga-log Store contributed by this starter.
type gormConfig struct {
	// DB names which *gorm.DB instance to use when the application registers
	// several (as gs.Group named beans). Empty selects the single/default
	// instance. The first version always autowires that default instance;
	// selecting a named one dynamically is left for a later revision, so this
	// field is currently informational.
	DB string `value:"${db:=}"`
}
