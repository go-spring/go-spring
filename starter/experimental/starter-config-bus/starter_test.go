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

package StarterConfigBus

import (
	"testing"

	"go-spring.org/spring/gs"
)

// TestBusNotAssembledWithoutConfig proves the conditional wiring: with no
// spring.config.bus.* property the starter assembles nothing, so blank-importing
// it in an app with zero NATS instances starts cleanly instead of failing on
// the missing "config-bus" connection.
func TestBusNotAssembledWithoutConfig(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Bus *ConfigBus `autowire:"?"`
	}) {
		if s.Bus != nil {
			t.Fatal("config bus must stay dormant without spring.config.bus.* config")
		}
	})
}
