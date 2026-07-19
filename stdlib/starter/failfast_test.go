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

package starter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireField(t *testing.T) {
	assert.NoError(t, RequireField("mail", "host", "smtp.example.com"))
	assert.NoError(t, RequireField("mail", "host", "  x  "))

	err := RequireField("mail", "host", "")
	assert.EqualError(t, err, "mail: host is required")

	err = RequireField("mail", "host", "   ")
	assert.EqualError(t, err, "mail: host is required")
}

func TestRequireAny(t *testing.T) {
	// Any present -> nil.
	assert.NoError(t, RequireAny("http-client",
		Field{Name: "addr", Value: "1.2.3.4"},
		Field{Name: "service-name", Value: ""},
	))
	assert.NoError(t, RequireAny("http-client",
		Field{Name: "addr", Value: ""},
		Field{Name: "service-name", Value: "orders"},
	))

	// All empty -> standardized message.
	err := RequireAny("http-client",
		Field{Name: "addr", Value: ""},
		Field{Name: "service-name", Value: "  "},
	)
	assert.EqualError(t, err, "http-client: one of addr or service-name is required")
}
