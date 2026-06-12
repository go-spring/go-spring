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

package validate

import (
	"testing"

	"github.com/go-spring/stdlib/errutil"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError error
	}{
		// Basic primary expressions
		{
			name:     "identifier",
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "dollar",
			input:    "$",
			expected: "$",
		},
		{
			name:     "nil",
			input:    "nil",
			expected: "nil",
		},
		{
			name:     "integer_positive",
			input:    "123",
			expected: "123",
		},
		{
			name:     "integer_negative",
			input:    "-456",
			expected: "-456",
		},
		{
			name:     "integer_positive_sign",
			input:    "+789",
			expected: "+789",
		},
		{
			name:     "hex_integer",
			input:    "0x1A3",
			expected: "0x1A3",
		},
		{
			name:     "float_simple",
			input:    "3.14",
			expected: "3.14",
		},
		{
			name:     "float_with_exp",
			input:    "1.23e+10",
			expected: "1.23e+10",
		},
		{
			name:     "float_negative",
			input:    "-2.5",
			expected: "-2.5",
		},
		{
			name:     "float_only_decimal",
			input:    ".5",
			expected: ".5",
		},
		{
			name:     "string",
			input:    `'hello'`,
			expected: `'hello'`,
		},
		{
			name:     "string_with_escape",
			input:    `'hello "world"'`,
			expected: `'hello "world"'`,
		},

		// Function calls
		{
			name:     "function_no_args",
			input:    "func()",
			expected: "func()",
		},
		{
			name:     "function_one_arg",
			input:    "func(foo)",
			expected: "func(foo)",
		},
		{
			name:     "function_multiple_args",
			input:    "func(foo, bar, 123)",
			expected: "func(foo, bar, 123)",
		},

		// Unary expressions
		{
			name:     "unary_not",
			input:    "!foo",
			expected: "!foo",
		},
		{
			name:     "unary_not_nested",
			input:    "!!foo",
			expected: "!!foo",
		},

		// Parenthesized expressions
		{
			name:     "parenthesized",
			input:    "(foo)",
			expected: "(foo)",
		},
		{
			name:     "nested_parentheses",
			input:    "((foo))",
			expected: "((foo))",
		},

		// Relational expressions
		{
			name:     "less_than",
			input:    "a < b",
			expected: "a < b",
		},
		{
			name:     "less_or_equal",
			input:    "a <= b",
			expected: "a <= b",
		},
		{
			name:     "greater_than",
			input:    "a > b",
			expected: "a > b",
		},
		{
			name:     "greater_or_equal",
			input:    "a >= b",
			expected: "a >= b",
		},

		// Equality expressions
		{
			name:     "equal",
			input:    "a == b",
			expected: "a == b",
		},
		{
			name:     "not_equal",
			input:    "a != b",
			expected: "a != b",
		},

		// Logical expressions
		{
			name:     "logical_and",
			input:    "a && b",
			expected: "a && b",
		},
		{
			name:     "logical_or",
			input:    "a || b",
			expected: "a || b",
		},

		// Complex expressions (testing operator precedence)
		{
			name:     "complex_precedence_1",
			input:    "a || b && c",
			expected: "a || b && c",
		},
		{
			name:     "complex_precedence_2",
			input:    "a && b || c",
			expected: "a && b || c",
		},
		{
			name:     "complex_precedence_3",
			input:    "a == b && c != d",
			expected: "a == b && c != d",
		},
		{
			name:     "complex_precedence_4",
			input:    "a < b && c > d",
			expected: "a < b && c > d",
		},
		{
			name:     "complex_precedence_5",
			input:    "!a && b",
			expected: "!a && b",
		},
		{
			name:     "complex_precedence_6",
			input:    "!(a && b)",
			expected: "!(a && b)",
		},
		{
			name:     "complex_with_parentheses",
			input:    "(a || b) && (c || d)",
			expected: "(a || b) && (c || d)",
		},
		{
			name:     "complex_nested_function",
			input:    "func1(func2(a, b), c)",
			expected: "func1(func2(a, b), c)",
		},
		{
			name:     "complex_all_operators",
			input:    "!func(a < b, c >= d) || (e == f && g != h)",
			expected: "!func(a < b, c >= d) || (e == f && g != h)",
		},

		// Additional test cases to improve coverage
		{
			name:     "float_with_negative_exp",
			input:    "1.23e-5",
			expected: "1.23e-5",
		},
		{
			name:     "function_with_complex_args",
			input:    "func(a && b, c || d, !e)",
			expected: "func(a && b, c || d, !e)",
		},
		{
			name:     "nested_parentheses_with_operators",
			input:    "((a && b) || (c && d))",
			expected: "((a && b) || (c && d))",
		},
		{
			name:     "complex_unary_with_function",
			input:    "!func(!a, !(b && c))",
			expected: "!func(!a, !(b && c))",
		},
		{
			name:     "string_with_multiple_escapes",
			input:    `'hello "world" \n\t'`,
			expected: `'hello "world" \n\t'`,
		},
		{
			name:     "zero_integer",
			input:    "0",
			expected: "0",
		},
		{
			name:     "zero_float",
			input:    "0.0",
			expected: "0.0",
		},
		{
			name:     "function_with_parentheses_in_args",
			input:    "func((a), (b && c))",
			expected: "func((a), (b && c))",
		},
		{
			name:     "mixed_relational_operators",
			input:    "a < b && c > d || e <= f && g >= h",
			expected: "a < b && c > d || e <= f && g >= h",
		},

		// Error cases
		{
			name:        "syntax_error_missing_paren",
			input:       "(a",
			expectError: errutil.Explain(nil, "line 1:2 missing ')' at '<EOF>' << text: \"(a\""),
		},
		{
			name:        "syntax_error_unexpected_token",
			input:       "a + b",
			expectError: errutil.Explain(nil, "line 1:2 token recognition error at: '+ ' << text: \"a + b\""),
		},
		{
			name:        "syntax_error_missing_comma_in_function",
			input:       "func(a b)",
			expectError: errutil.Explain(nil, "line 1:7 extraneous input 'b' expecting {')', ','} << text: \"func(a b)\""),
		},
		{
			name:        "syntax_error_trailing_token",
			input:       "a b",
			expectError: errutil.Explain(nil, "line 1:2 unexpected trailing token \"b\" << text: \"a b\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("Parse(%q) expected error, but got none", tt.input)
				} else if tt.expectError.Error() != err.Error() {
					t.Errorf("Parse(%q) returned unexpected error: %v", tt.input, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) returned unexpected error: %v", tt.input, err)
				return
			}

			if expr == nil {
				if tt.expected != "" {
					t.Errorf("Parse(%q) = nil, want %q", tt.input, tt.expected)
				}
				return
			}

			actual := expr.Text()
			if actual != tt.expected {
				t.Errorf("Parse(%q).Text() = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}
