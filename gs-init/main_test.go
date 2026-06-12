package main

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
			name:     "single word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "snake case with two words",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "snake case with multiple words",
			input:    "hello_world_test_case",
			expected: "HelloWorldTestCase",
		},
		{
			name:     "leading underscore",
			input:    "_hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "trailing underscore",
			input:    "hello_world_",
			expected: "HelloWorld",
		},
		{
			name:     "multiple consecutive underscores",
			input:    "hello__world",
			expected: "HelloWorld",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "already pascal case",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "with numbers",
			input:    "hello_2_world",
			expected: "Hello2World",
		},
		{
			name:     "mixed case input",
			input:    "Hello_world_Test",
			expected: "HelloWorldTest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toPascal(tt.input)
			if result != tt.expected {
				t.Errorf("toPascal(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
