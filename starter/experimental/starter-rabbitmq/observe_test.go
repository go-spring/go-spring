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

package StarterRabbitMQ

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// A connection event's counter attribute and its log field must be the same key
// with the same value, or a dashboard selecting
// messaging.client.connection.state_changes{state=...} lands on lines that
// cannot be joined to it. record returns both together for exactly that
// reason; this pins the field side.
func TestConnStateRecordCarriesTheMetricKeys(t *testing.T) {
	fields := newConnStateCounter().record(context.Background(), connBlocked)

	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "state"})
}
