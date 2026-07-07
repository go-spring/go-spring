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

package textstyle_test

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/textstyle"
)

func TestAttribute_Sprint(t *testing.T) {
	tests := []struct {
		name     string
		attr     textstyle.Attribute
		input    string
		expected string
	}{
		{
			name:     "Bold attribute",
			attr:     textstyle.Bold,
			input:    "test",
			expected: "\x1b[1mtest\x1b[0m",
		},
		{
			name:     "Italic attribute",
			attr:     textstyle.Italic,
			input:    "test",
			expected: "\x1b[3mtest\x1b[0m",
		},
		{
			name:     "Underline attribute",
			attr:     textstyle.Underline,
			input:    "test",
			expected: "\x1b[4mtest\x1b[0m",
		},
		{
			name:     "ReverseVideo attribute",
			attr:     textstyle.ReverseVideo,
			input:    "test",
			expected: "\x1b[7mtest\x1b[0m",
		},
		{
			name:     "CrossedOut attribute",
			attr:     textstyle.CrossedOut,
			input:    "test",
			expected: "\x1b[9mtest\x1b[0m",
		},
		{
			name:     "Red color",
			attr:     textstyle.Red,
			input:    "test",
			expected: "\x1b[31mtest\x1b[0m",
		},
		{
			name:     "BgGreen background",
			attr:     textstyle.BgGreen,
			input:    "test",
			expected: "\x1b[42mtest\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.attr.Sprint(tt.input)
			assert.String(t, result).Equal(tt.expected)
		})
	}
}

func TestAttribute_Sprintf(t *testing.T) {
	result := textstyle.Red.Sprintf("hello %s", "world")
	assert.String(t, result).Equal("\x1b[31mhello world\x1b[0m")
}

func TestText_Sprint(t *testing.T) {
	// Test empty attributes
	text := textstyle.NewText()
	result := text.Sprint("test")
	assert.String(t, result).Equal("test")

	// Test multiple attributes
	attributes := []textstyle.Attribute{
		textstyle.Bold,
		textstyle.Red,
		textstyle.BgGreen,
	}
	textWithAttrs := textstyle.NewText(attributes...)
	result = textWithAttrs.Sprint("test")
	assert.String(t, result).Equal("\x1b[1;31;42mtest\x1b[0m")
}

func TestText_Sprintf(t *testing.T) {
	attributes := []textstyle.Attribute{
		textstyle.Bold,
		textstyle.Blue,
	}
	text := textstyle.NewText(attributes...)
	result := text.Sprintf("hello %s", "world")
	assert.String(t, result).Equal("\x1b[1;34mhello world\x1b[0m")
}

func TestWrapFunction(t *testing.T) {
	// Test that wrap function properly handles multiple attributes
	attributes := []textstyle.Attribute{textstyle.Bold, textstyle.Italic}
	result := textstyle.NewText(attributes...).Sprint("test")
	assert.String(t, result).Equal("\x1b[1;3mtest\x1b[0m")

	// Test empty attributes case
	emptyResult := textstyle.NewText().Sprint("test")
	assert.String(t, emptyResult).Equal("test")
}

func TestANSIFormatCorrectness(t *testing.T) {
	result := textstyle.Bold.Sprint("test")
	assert.String(t, result).HasPrefix("\x1b[")
	assert.String(t, result).HasSuffix("\x1b[0m")
	assert.String(t, result).Contains("test")
}
