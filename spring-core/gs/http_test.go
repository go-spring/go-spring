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

package gs

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestNewSimpleHttpServer(t *testing.T) {
	t.Run("nil mux", func(t *testing.T) {
		s := NewSimpleHttpServer(nil, SimpleHttpServerConfig{Address: ":0"})
		assert.That(t, s.svr.Addr).Equal(":0")
		assert.That(t, s.svr.Handler).Nil()
	})
}
