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

package chain

import (
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestConnectErrorPreservesTheOriginal(t *testing.T) {
	inner := errors.New("refused")
	wrapped := ConnectError(inner)
	assert.That(t, errors.Is(wrapped, inner)).True()
	assert.That(t, wrapped.Error()).Equal("refused")
	// nil in is NOT nil out by design: ConnectError is the failure path's marker,
	// and a caller that runs it on a success path gets a loud non-nil error
	// rather than a silent pass-through.
	assert.That(t, ConnectError(nil)).NotNil()
}
