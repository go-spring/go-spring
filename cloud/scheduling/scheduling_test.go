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

package scheduling_test

import (
	"testing"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

func TestConcurrencyPolicyString(t *testing.T) {
	assert.That(t, scheduling.Skip.String()).Equal("skip")
	assert.That(t, scheduling.Queue.String()).Equal("queue")
	assert.That(t, scheduling.Replace.String()).Equal("replace")
	assert.That(t, scheduling.ConcurrencyPolicy(99).String()).Equal("unknown")
}
