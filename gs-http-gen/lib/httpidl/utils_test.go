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

package httpidl

import (
	"testing"
)

func TestToPascal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single lowercase word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "snake_case with two words",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "snake_case with multiple words",
			input:    "hello_world_golang",
			expected: "HelloWorldGolang",
		},
		{
			name:     "leading underscore",
			input:    "_hello",
			expected: "Hello",
		},
		{
			name:     "trailing underscore",
			input:    "hello_",
			expected: "Hello",
		},
		{
			name:     "multiple leading underscores",
			input:    "__hello",
			expected: "Hello",
		},
		{
			name:     "multiple trailing underscores",
			input:    "hello__",
			expected: "Hello",
		},
		{
			name:     "middle double underscore",
			input:    "hello__world",
			expected: "HelloWorld",
		},
		{
			name:     "multiple underscores",
			input:    "hello___world___golang",
			expected: "HelloWorldGolang",
		},
		{
			name:     "all uppercase snake_case",
			input:    "HELLO_WORLD",
			expected: "HELLOWORLD",
		},
		{
			name:     "mixed case snake_case",
			input:    "Hello_World",
			expected: "HelloWorld",
		},
		{
			name:     "with numbers",
			input:    "http_200_status",
			expected: "Http200Status",
		},
		{
			name:     "first character uppercase",
			input:    "Http_client",
			expected: "HttpClient",
		},
		{
			name:     "only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "single character words",
			input:    "a_b_c",
			expected: "ABC",
		},
		{
			name:     "non-ascii characters",
			input:    "héllo_wörld",
			expected: "HélloWörld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ToPascal(tt.input)
			if actual != tt.expected {
				t.Errorf("ToPascal(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
