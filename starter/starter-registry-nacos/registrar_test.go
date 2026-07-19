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

package StarterRegistryNacos

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestSplitAddr(t *testing.T) {
	// A well-formed address splits into host and numeric port.
	host, port, err := splitAddr("10.0.0.5:8080")
	assert.Error(t, err).Nil()
	assert.That(t, host).Equal("10.0.0.5")
	assert.That(t, port).Equal(uint64(8080))

	// A missing port fails fast before any Nacos call.
	_, _, err = splitAddr("no-port")
	assert.Error(t, err).Matches("must be host:port")

	// A non-numeric port fails fast too.
	_, _, err = splitAddr("host:abc")
	assert.Error(t, err).Matches("non-numeric port")
}
