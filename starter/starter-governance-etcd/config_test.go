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

package StarterGovernanceEtcd

import "testing"

func TestSourceFormat(t *testing.T) {
	tests := []struct {
		explicit string
		name     string
		want     string
	}{
		{"", "governance.yaml", "yaml"},       // inferred from extension
		{"", "governance.json", "json"},       // inferred from extension
		{"", "/app/governance", "properties"}, // no extension — properties
		{"", "/app/gov.", "properties"},       // bare dot is not an extension
		{"xml", "governance.yaml", "xml"},     // explicit format wins
		{"xml", "governance", "xml"},          // explicit format without extension
	}
	for _, tt := range tests {
		if got := sourceFormat(tt.explicit, tt.name); got != tt.want {
			t.Errorf("sourceFormat(%q, %q) = %q, want %q", tt.explicit, tt.name, got, tt.want)
		}
	}
}
