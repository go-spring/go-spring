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

package admission

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestSettingsValidate pins the document-level checks: an empty document is
// valid (the zero Policy protects nothing), while every rule must name its
// entrance and entrances must be unique.
func TestSettingsValidate(t *testing.T) {
	assert.Error(t, (Settings{}).Validate()).Nil()
	assert.Error(t, (Settings{Rules: []Rule{{Service: ""}}}).Validate()).Matches("without a service")
	assert.Error(t, (Settings{Rules: []Rule{
		{Service: "gin::8080"},
		{Service: "gin::8080"},
	}}).Validate()).Matches("duplicate entry")
}
