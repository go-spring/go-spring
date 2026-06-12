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

package testcase_test

import (
	"strings"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
	"github.com/go-spring/stdlib/testing/internal"
	"github.com/go-spring/stdlib/testing/require"
)

func TestString_Length(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "0").Length(1)
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "0").Length(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to have length 0, but it has length 1
  actual: "0"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "0").Length(0, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to have length 0, but it has length 1
  actual: "0"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").Length(0)
	assert.String(t, m.String()).Equal("")

	// Test with multi-byte UTF-8 characters
	m.Reset()
	assert.String(m, "你好").Length(6) // "你好" has 6 bytes in UTF-8
	assert.String(t, m.String()).Equal("")

	// Test with multi-byte UTF-8 characters - failure case
	m.Reset()
	assert.String(m, "你好").Length(2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to have length 2, but it has length 6
  actual: "你好"`)

	// Test with special characters
	m.Reset()
	assert.String(m, "\n\t\r").Length(3)
	assert.String(t, m.String()).Equal("")

	// Test failure with longer string
	m.Reset()
	assert.String(m, "hello world").Length(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to have length 5, but it has length 11
  actual: "hello world"`)

	// Test with custom message - success case (no output)
	m.Reset()
	assert.String(m, "test").Length(4, "custom message")
	assert.String(t, m.String()).Equal("")
}

func TestString_Blank(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case with regular spaces
	m.Reset()
	assert.String(m, "   ").Blank()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "hello").Blank()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only whitespace, but it does not
  actual: "hello"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "hello").Blank("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to contain only whitespace, but it does not
  actual: "hello"
 message: "index is 0"`)

	// Test with empty string - should pass as it's considered blank
	m.Reset()
	assert.String(m, "").Blank()
	assert.String(t, m.String()).Equal("")

	// Test with various whitespace characters
	m.Reset()
	assert.String(m, " \t\n\r  ").Blank()
	assert.String(t, m.String()).Equal("")

	// Test with string containing a single non-whitespace character - should fail
	m.Reset()
	assert.String(m, " a ").Blank()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only whitespace, but it does not
  actual: " a "`)

	// Test with Unicode non-whitespace character - should fail
	m.Reset()
	assert.String(m, " \t中文\r\n ").Blank() // Contains Chinese characters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only whitespace, but it does not
  actual: " \t中文\r\n "`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "text").Blank("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only whitespace, but it does not
  actual: "text"
 message: "custom failure message"`)
}

func TestString_NotBlank(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello").NotBlank()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "   ").NotBlank()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be non-blank, but it is blank
  actual: "   "`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, " \n  ").NotBlank("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be non-blank, but it is blank
  actual: " \n  "
 message: "index is 0"`)

	// Test with empty string - should fail
	m.Reset()
	assert.String(m, "").NotBlank()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be non-blank, but it is blank
  actual: ""`)

	// Test with various whitespace characters - should fail
	m.Reset()
	assert.String(m, " \t\n\r  ").NotBlank()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be non-blank, but it is blank
  actual: " \t\n\r  "`)

	// Test with string containing a single non-whitespace character - should pass
	m.Reset()
	assert.String(m, " a ").NotBlank()
	assert.String(t, m.String()).Equal("")

	// Test with Unicode non-whitespace character - should pass
	m.Reset()
	assert.String(m, " \t中文\r\n ").NotBlank() // Contains Chinese characters
	assert.String(t, m.String()).Equal("")

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "  ").NotBlank("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be non-blank, but it is blank
  actual: "  "
 message: "custom failure message"`)

	// Test with single character - should pass
	m.Reset()
	assert.String(m, "a").NotBlank()
	assert.String(t, m.String()).Equal("")
}

func TestString_Equal(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "0").Equal("0")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "0").Equal("1")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal, but they are not
  actual: "0"
expected: "1"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "0").Equal("1", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected strings to be equal, but they are not
  actual: "0"
expected: "1"
 message: "index is 0"`)

	// Test with empty strings
	m.Reset()
	assert.String(m, "").Equal("")
	assert.String(t, m.String()).Equal("")

	// Test with Unicode strings - failure case
	m.Reset()
	assert.String(m, "你好世界").Equal("再见世界")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal, but they are not
  actual: "你好世界"
expected: "再见世界"`)

	// Test with strings containing special characters
	m.Reset()
	assert.String(m, "hello\nworld\t!").Equal("hello\nworld\t!")
	assert.String(t, m.String()).Equal("")

	// Test with very long strings - failure case
	longStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, longStr).Equal(longStr + "x")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal, but they are not
  actual: "` + longStr + `"
expected: "` + longStr + `x"`)

	// Test with strings containing only whitespace - failure case
	m.Reset()
	assert.String(m, "   ").Equal("  ")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal, but they are not
  actual: "   "
expected: "  "`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").Equal("expected", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal, but they are not
  actual: "actual"
expected: "expected"
 message: "custom failure message"`)
}

func TestString_NotEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "0").NotEqual("1")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "0").NotEqual("0")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "0"
expected: "0"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "0").NotEqual("0", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected strings to be different, but they are equal
  actual: "0"
expected: "0"
 message: "index is 0"`)

	// Test with empty strings - failure case
	m.Reset()
	assert.String(m, "").NotEqual("")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: ""
expected: ""`)

	// Test with Unicode strings - failure case
	m.Reset()
	assert.String(m, "你好世界").NotEqual("你好世界")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "你好世界"
expected: "你好世界"`)

	// Test with strings containing special characters - failure case
	m.Reset()
	assert.String(m, "hello\nworld\t!").NotEqual("hello\nworld\t!")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "hello\nworld\t!"
expected: "hello\nworld\t!"`)

	// Test with very long strings - failure case
	longStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, longStr).NotEqual(longStr)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "` + longStr + `"
expected: "` + longStr + `"`)

	// Test with strings containing only whitespace - failure case
	m.Reset()
	assert.String(m, "   ").NotEqual("   ")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "   "
expected: "   "`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").NotEqual("actual", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be different, but they are equal
  actual: "actual"
expected: "actual"
 message: "custom failure message"`)
}

func TestString_EqualFold(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello, world!").EqualFold("Hello, World!")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "hello, world!").EqualFold("Hello, Jimmy!")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "hello, world!"
expected: "Hello, Jimmy!"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "hello, world!").EqualFold("Hello, Jimmy!", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "hello, world!"
expected: "Hello, Jimmy!"
 message: "index is 0"`)

	// Test with empty strings
	m.Reset()
	assert.String(m, "").EqualFold("")
	assert.String(t, m.String()).Equal("")

	// Test with Unicode strings - failure case
	m.Reset()
	assert.String(m, "ПРИВЕТ").EqualFold("ПОКА") // Russian "HELLO" vs "BYE"
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "ПРИВЕТ"
expected: "ПОКА"`)

	// Test with strings containing special Unicode characters
	m.Reset()
	assert.String(m, "café").EqualFold("CAFÉ") // With accented characters
	assert.String(t, m.String()).Equal("")

	// Test with very long strings - failure case
	longStr := strings.Repeat("A", 1000)
	expectedLongStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, longStr).EqualFold(expectedLongStr + "x")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "` + longStr + `"
expected: "` + expectedLongStr + `x"`)

	// Test with strings containing only whitespace - different amount
	m.Reset()
	assert.String(m, "   ").EqualFold("  ")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "   "
expected: "  "`)

	// Test with mixed case strings including numbers
	m.Reset()
	assert.String(m, "User@Example.COM").EqualFold("user@example.com")
	assert.String(t, m.String()).Equal("")

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").EqualFold("expected", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be equal (case-insensitive), but they are not
  actual: "actual"
expected: "expected"
 message: "custom failure message"`)
}

func TestString_JSONEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, `{"a":0,"b":1}`).JSONEqual(`{"b":1,"a":0}`)
	assert.String(t, m.String()).Equal("")

	// Test failure case with unmarshal error in actual value
	m.Reset()
	assert.String(m, `this is an error`).JSONEqual(`[{"b":1},{"a":0}]`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but failed to unmarshal actual value
  actual: "this is an error"
   error: "invalid character 'h' in literal true (expecting 'r')"`)

	// Test failure case with unmarshal error in expected value
	m.Reset()
	require.String(m, `{"a":0,"b":1}`).JSONEqual(`this is an error`)
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected strings to be JSON-equal, but failed to unmarshal expected value
expected: "this is an error"
   error: "invalid character 'h' in literal true (expecting 'r')"`)

	// Test JSON structure mismatch
	m.Reset()
	assert.String(m, `{"a":0,"b":1}`).JSONEqual(`[{"b":1},{"a":0}]`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but they are not
  actual: "{\"a\":0,\"b\":1}"
expected: "[{\"b\":1},{\"a\":0}]"`)

	// Test value mismatch
	m.Reset()
	require.String(m, `{"a":0}`).JSONEqual(`{"a":1}`, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected strings to be JSON-equal, but they are not
  actual: "{\"a\":0}"
expected: "{\"a\":1}"
 message: "index is 0"`)

	// Test with nested JSON objects - failure case
	m.Reset()
	assert.String(m, `{"user":{"name":"John","age":30}}`).JSONEqual(`{"user":{"name":"Jane","age":30}}`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but they are not
  actual: "{\"user\":{\"name\":\"John\",\"age\":30}}"
expected: "{\"user\":{\"name\":\"Jane\",\"age\":30}}"`)

	// Test with JSON arrays containing objects - different order
	m.Reset()
	assert.String(m, `[{"id":1,"name":"John"},{"id":2,"name":"Jane"}]`).JSONEqual(`[{"id":2,"name":"Jane"},{"id":1,"name":"John"}]`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but they are not
  actual: "[{\"id\":1,\"name\":\"John\"},{\"id\":2,\"name\":\"Jane\"}]"
expected: "[{\"id\":2,\"name\":\"Jane\"},{\"id\":1,\"name\":\"John\"}]"`)

	// Test with invalid JSON in actual value
	m.Reset()
	assert.String(m, `{"invalid":}`).JSONEqual(`{"valid":true}`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but failed to unmarshal actual value
  actual: "{\"invalid\":}"
   error: "invalid character '}' looking for beginning of value"`)

	// Test with invalid JSON in expected value
	m.Reset()
	assert.String(m, `{"valid":true}`).JSONEqual(`{"invalid":}`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but failed to unmarshal expected value
expected: "{\"invalid\":}"
   error: "invalid character '}' looking for beginning of value"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, `{"actual":true}`).JSONEqual(`{"expected":false}`, "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected strings to be JSON-equal, but they are not
  actual: "{\"actual\":true}"
expected: "{\"expected\":false}"
 message: "custom failure message"`)

	// Test with whitespace differences (should still be equal as JSON)
	m.Reset()
	assert.String(m, `{"a": 1, "b": 2}`).JSONEqual(`{"b":2,"a":1}`)
	assert.String(t, m.String()).Equal("")
}

func TestString_Matches(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "this is an error").Matches("this is an error")
	assert.String(t, m.String()).Equal("")

	// Test failure case with regex error
	m.Reset()
	assert.String(m, "this is an error").Matches("an error (")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "this is an error"
 pattern: "an error ("
   error: "error parsing regexp: missing closing ): ` + "`an error (`\"")

	// Test failure case with pattern not matching
	m.Reset()
	assert.String(m, "there's no error").Matches("an error")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "there's no error"
 pattern: "an error"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "there's no error").Matches("an error", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to match the pattern, but it does not
  actual: "there's no error"
 pattern: "an error"
 message: "index is 0"`)

	// Test with empty string and non-empty pattern - should fail
	m.Reset()
	assert.String(m, "").Matches("non-empty")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: ""
 pattern: "non-empty"`)

	// Test with simple regex patterns - failure case
	m.Reset()
	assert.String(m, "123abc").Matches(`[a-z]+\d+`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "123abc"
 pattern: "[a-z]+\\d+"`)

	// Test with email-like pattern - failure case
	m.Reset()
	assert.String(m, "user@").Matches(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "user@"
 pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"`)

	// Test with Unicode characters in pattern - failure case
	m.Reset()
	assert.String(m, "123你好").Matches(`[\x{4e00}-\x{9fa5}]+\d+`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "123你好"
 pattern: "[\\x{4e00}-\\x{9fa5}]+\\d+"`)

	// Test with anchors in pattern - failure case
	m.Reset()
	assert.String(m, "not exact string").Matches(`^exact$`)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "not exact string"
 pattern: "^exact$"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "test").Matches(`test\d+`, "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to match the pattern, but it does not
  actual: "test"
 pattern: "test\\d+"
 message: "custom failure message"`)
}

func TestString_HasPrefix(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello, world!").HasPrefix("hello")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "hello, world!").HasPrefix("Hello, Jimmy!")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "hello, world!"
  prefix: "Hello, Jimmy!"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "hello, world!").HasPrefix("Hello, Jimmy!", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "hello, world!"
  prefix: "Hello, Jimmy!"
 message: "index is 0"`)

	// Test with empty string and non-empty prefix - should fail
	m.Reset()
	assert.String(m, "").HasPrefix("hello")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: ""
  prefix: "hello"`)

	// Test with Unicode characters - failure case
	m.Reset()
	assert.String(m, "你好世界").HasPrefix("世界")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "你好世界"
  prefix: "世界"`)

	// Test with special characters - failure case
	m.Reset()
	assert.String(m, "hello\nworld\t!").HasPrefix("world")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "hello\nworld\t!"
  prefix: "world"`)

	// Test with very long prefix that doesn't match
	longStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, longStr+"suffix").HasPrefix(longStr + "x")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "` + longStr + `suffix"
  prefix: "` + longStr + `x"`)

	// Test with single character prefix - no match
	m.Reset()
	assert.String(m, "b").HasPrefix("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "b"
  prefix: "a"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").HasPrefix("expected", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to start with the specified prefix, but it does not
  actual: "actual"
  prefix: "expected"
 message: "custom failure message"`)
}

func TestString_HasSuffix(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello, world!").HasSuffix("world!")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "hello, world!").HasSuffix("Hello, Jimmy!")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "hello, world!"
  suffix: "Hello, Jimmy!"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "hello, world!").HasSuffix("Hello, Jimmy!", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "hello, world!"
  suffix: "Hello, Jimmy!"
 message: "index is 0"`)

	// Test with empty string and non-empty suffix - should fail
	m.Reset()
	assert.String(m, "").HasSuffix("hello")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: ""
  suffix: "hello"`)

	// Test with Unicode characters - failure case
	m.Reset()
	assert.String(m, "你好世界").HasSuffix("你好")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "你好世界"
  suffix: "你好"`)

	// Test with special characters - failure case
	m.Reset()
	assert.String(m, "hello\nworld\t!").HasSuffix("hello")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "hello\nworld\t!"
  suffix: "hello"`)

	// Test with very long suffix that matches
	longStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, "prefix"+longStr).HasSuffix(longStr)
	assert.String(t, m.String()).Equal("")

	// Test with single character suffix - no match
	m.Reset()
	assert.String(m, "b").HasSuffix("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "b"
  suffix: "a"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").HasSuffix("expected", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to end with the specified suffix, but it does not
  actual: "actual"
  suffix: "expected"
 message: "custom failure message"`)
}

func TestString_Contains(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello, world!").Contains("hello")
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "hello, world!").Contains("Hello, Jimmy!")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "hello, world!"
     sub: "Hello, Jimmy!"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "hello, world!").Contains("Hello, Jimmy!", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "hello, world!"
     sub: "Hello, Jimmy!"
 message: "index is 0"`)

	// Test with empty string and non-empty substring - should fail
	m.Reset()
	assert.String(m, "").Contains("hello")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: ""
     sub: "hello"`)

	// Test with Unicode characters - failure case
	m.Reset()
	assert.String(m, "你好世界").Contains("再见")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "你好世界"
     sub: "再见"`)

	// Test with special characters - failure case
	m.Reset()
	assert.String(m, "hello\nworld\t!").Contains("universe")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "hello\nworld\t!"
     sub: "universe"`)

	// Test with very long string and substring that doesn't match
	longStr := strings.Repeat("a", 1000)
	m.Reset()
	assert.String(m, "prefix"+longStr+"suffix").Contains(longStr + "x")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "prefix` + longStr + `suffix"
     sub: "` + longStr + `x"`)

	// Test with substring in the middle
	m.Reset()
	assert.String(m, "hello world!").Contains("lo wo")
	assert.String(t, m.String()).Equal("")

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "actual").Contains("expected", "custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain the specified substring, but it does not
  actual: "actual"
     sub: "expected"
 message: "custom failure message"`)
}

func TestString_IsLowerCase(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "hello").IsLowerCase()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "Hello").IsLowerCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "Hello"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "Hello").IsLowerCase("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be all lowercase, but it is not
  actual: "Hello"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").IsLowerCase()
	assert.String(t, m.String()).Equal("")

	// Test with numbers only
	m.Reset()
	assert.String(m, "1234567890").IsLowerCase()
	assert.String(t, m.String()).Equal("")

	// Test with mixed case letters - should fail
	m.Reset()
	assert.String(m, "HeLLo").IsLowerCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "HeLLo"`)

	// Test with Unicode uppercase letters - should fail
	m.Reset()
	assert.String(m, "ΑΒΓΔΕΖΗΘ").IsLowerCase() // Greek uppercase letters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "ΑΒΓΔΕΖΗΘ"`)

	// Test with Unicode mixed case letters - should fail
	m.Reset()
	assert.String(m, "αΒγΔεΖηΘ").IsLowerCase() // Greek mixed case letters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "αΒγΔεΖηΘ"`)

	// Test with whitespace and uppercase - should fail
	m.Reset()
	assert.String(m, "Hello World").IsLowerCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "Hello World"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "Actual").IsLowerCase("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all lowercase, but it is not
  actual: "Actual"
 message: "custom failure message"`)
}

func TestString_IsUpperCase(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "HELLO").IsUpperCase()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "Hello").IsUpperCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "Hello"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "Hello").IsUpperCase("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be all uppercase, but it is not
  actual: "Hello"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").IsUpperCase()
	assert.String(t, m.String()).Equal("")

	// Test with numbers only
	m.Reset()
	assert.String(m, "1234567890").IsUpperCase()
	assert.String(t, m.String()).Equal("")

	// Test with mixed case letters - should fail
	m.Reset()
	assert.String(m, "HeLLo").IsUpperCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "HeLLo"`)

	// Test with Unicode lowercase letters - should fail
	m.Reset()
	assert.String(m, "αβγδεζηθ").IsUpperCase() // Greek lowercase letters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "αβγδεζηθ"`)

	// Test with Unicode mixed case letters - should fail
	m.Reset()
	assert.String(m, "ΑβΓδΕζΗθ").IsUpperCase() // Greek mixed case letters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "ΑβΓδΕζΗθ"`)

	// Test with whitespace and lowercase - should fail
	m.Reset()
	assert.String(m, "Hello World").IsUpperCase()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "Hello World"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "Actual").IsUpperCase("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be all uppercase, but it is not
  actual: "Actual"
 message: "custom failure message"`)
}

func TestString_IsNumeric(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "123456").IsNumeric()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "123a456").IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "123a456"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "123a456").IsNumeric("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to contain only digits, but it does not
  actual: "123a456"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").IsNumeric()
	assert.String(t, m.String()).Equal("")

	// Test with negative number - should fail
	m.Reset()
	assert.String(m, "-123").IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "-123"`)

	// Test with decimal number - should fail
	m.Reset()
	assert.String(m, "123.456").IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "123.456"`)

	// Test with letters - should fail
	m.Reset()
	assert.String(m, "abc").IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "abc"`)

	// Test with special characters - should fail
	m.Reset()
	assert.String(m, "!@#$%").IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "!@#$%"`)

	// Test with Unicode digits - should fail (only ASCII digits 0-9 are allowed)
	m.Reset()
	assert.String(m, "１２３４５６").IsNumeric() // Full-width digits
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "１２３４５６"`)

	// Test with very long alphanumeric string - should fail
	longNumeric := strings.Repeat("9", 1000)
	longAlphanumeric := longNumeric + "a"
	m.Reset()
	assert.String(m, longAlphanumeric).IsNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "` + longAlphanumeric + `"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "123a45").IsNumeric("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only digits, but it does not
  actual: "123a45"
 message: "custom failure message"`)
}

func TestString_IsAlpha(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "abcdef").IsAlpha()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "abc123").IsAlpha()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "abc123"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "abc123").IsAlpha("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to contain only letters, but it does not
  actual: "abc123"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").IsAlpha()
	assert.String(t, m.String()).Equal("")

	// Test with numbers only - should fail
	m.Reset()
	assert.String(m, "1234567890").IsAlpha()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "1234567890"`)

	// Test with special characters only - should fail
	m.Reset()
	assert.String(m, "!@#$%^&*()_+-=[]{}|;':\",./<>?").IsAlpha()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "!@#$%^&*()_+-=[]{}|;':\",./<>?"`)

	// Test with letters and special characters - should fail
	m.Reset()
	assert.String(m, "abc!@#").IsAlpha()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "abc!@#"`)

	// Test with Unicode letters - should fail (only ASCII letters a-z and A-Z are allowed)
	m.Reset()
	assert.String(m, "αβγδεζηθ").IsAlpha() // Greek letters
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "αβγδεζηθ"`)

	// Test with very long alphanumeric string - should fail
	longAlpha := strings.Repeat("a", 1000)
	longAlphanumeric := longAlpha + "1"
	m.Reset()
	assert.String(m, longAlphanumeric).IsAlpha()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "` + longAlphanumeric + `"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "abc123").IsAlpha("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters, but it does not
  actual: "abc123"
 message: "custom failure message"`)
}

func TestString_IsAlphaNumeric(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "abc123").IsAlphaNumeric()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "abc@123").IsAlphaNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "abc@123"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "abc@123").IsAlphaNumeric("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "abc@123"
 message: "index is 0"`)

	// Test with empty string
	m.Reset()
	assert.String(m, "").IsAlphaNumeric()
	assert.String(t, m.String()).Equal("")

	// Test with special characters only - should fail
	m.Reset()
	assert.String(m, "!@#$%^&*()_+-=[]{}|;':\",./<>?").IsAlphaNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "!@#$%^&*()_+-=[]{}|;':\",./<>?"`)

	// Test with letters, digits and special characters - should fail
	m.Reset()
	assert.String(m, "abc123!@#").IsAlphaNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "abc123!@#"`)

	// Test with whitespace characters - should fail
	m.Reset()
	assert.String(m, "abc123 def456").IsAlphaNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "abc123 def456"`)

	// Test with Unicode letters - should fail (only ASCII letters a-z and A-Z are allowed)
	m.Reset()
	assert.String(m, "αβγδεζηθ123").IsAlphaNumeric() // Greek letters with digits
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "αβγδεζηθ123"`)

	// Test with very long alphanumeric string with special character - should fail
	longAlpha := strings.Repeat("a", 500)
	longNumeric := strings.Repeat("9", 500)
	longAlphanumeric := longAlpha + longNumeric
	m.Reset()
	assert.String(m, longAlphanumeric+"!").IsAlphaNumeric()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "` + longAlphanumeric + `!"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "abc123@").IsAlphaNumeric("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to contain only letters and digits, but it does not
  actual: "abc123@"
 message: "custom failure message"`)
}

func TestString_IsEmail(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "test@example.com").IsEmail()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "invalid-email").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "invalid-email"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "invalid-email").IsEmail("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be a valid email, but it is not
  actual: "invalid-email"
 message: "index is 0"`)

	// Test with empty string - should fail
	m.Reset()
	assert.String(m, "").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: ""`)

	// Test with various valid email formats
	m.Reset()
	assert.String(m, "user+tag@example.co.uk").IsEmail()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.String(m, "a@b.co").IsEmail() // Minimal valid email
	assert.String(t, m.String()).Equal("")

	// Test with invalid email formats
	m.Reset()
	assert.String(m, "@example.com").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "@example.com"`)

	m.Reset()
	assert.String(m, "user@@example.com").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "user@@example.com"`)

	m.Reset()
	assert.String(m, "user@example").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "user@example"`)

	m.Reset()
	assert.String(m, "user name@example.com").IsEmail() // Space in local part
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "user name@example.com"`)

	// Test with special characters
	m.Reset()
	assert.String(m, "user#tag@example.com").IsEmail()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "user#tag@example.com"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "invalid-email").IsEmail("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid email, but it is not
  actual: "invalid-email"
 message: "custom failure message"`)
}

func TestString_IsURL(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "https://www.example.com").IsURL()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "invalid-url").IsURL()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "invalid-url"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "invalid-url").IsURL("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be a valid URL, but it is not
  actual: "invalid-url"
 message: "index is 0"`)

	// Test with empty string - should fail
	m.Reset()
	assert.String(m, "").IsURL()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: ""`)

	// Test with various valid URL formats
	m.Reset()
	assert.String(m, "ftp://example.com").IsURL()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.String(m, "https://subdomain.example.com/path").IsURL()
	assert.String(t, m.String()).Equal("")

	// Test with invalid URL formats
	m.Reset()
	assert.String(m, "://example.com").IsURL()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "://example.com"`)

	m.Reset()
	assert.String(m, "http://").IsURL()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "http://"`)

	m.Reset()
	assert.String(m, "http://example.com ").IsURL() // trailing space
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "http://example.com "`)

	// Test with unsupported protocols
	m.Reset()
	assert.String(m, "file:///path/to/file").IsURL()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "file:///path/to/file"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "invalid-url").IsURL("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid URL, but it is not
  actual: "invalid-url"
 message: "custom failure message"`)
}

func TestString_IsIPv4(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "192.168.1.1").IsIPv4()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "invalid-ip").IsIPv4()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "invalid-ip"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "invalid-ip").IsIPv4("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be a valid IP, but it is not
  actual: "invalid-ip"
 message: "index is 0"`)

	// Test with empty string - should fail
	m.Reset()
	assert.String(m, "").IsIPv4()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: ""`)

	// Test with various valid IPv4 formats
	m.Reset()
	assert.String(m, "255.255.255.255").IsIPv4()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.String(m, "8.8.8.8").IsIPv4()
	assert.String(t, m.String()).Equal("")

	// Test with edge case values
	m.Reset()
	assert.String(m, "192.168.001.001").IsIPv4() // Leading zeros
	assert.String(t, m.String()).Equal("")

	// Test with invalid IPv4 formats
	m.Reset()
	assert.String(m, "256.1.1.1").IsIPv4() // Value > 255
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "256.1.1.1"`)

	m.Reset()
	assert.String(m, "1.1.1").IsIPv4() // Missing octet
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "1.1.1"`)

	m.Reset()
	assert.String(m, "1.1.1.1.1").IsIPv4() // Extra octet
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "1.1.1.1.1"`)

	m.Reset()
	assert.String(m, "1.-1.1.1").IsIPv4() // Negative number
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "1.-1.1.1"`)

	// Test with non-numeric characters
	m.Reset()
	assert.String(m, "1.1.1.a").IsIPv4()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "1.1.1.a"`)

	m.Reset()
	assert.String(m, "1.1.1.*").IsIPv4() // Special character
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "1.1.1.*"`)

	// Test with custom message - failure case
	m.Reset()
	assert.String(m, "invalid-ip").IsIPv4("custom failure message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid IP, but it is not
  actual: "invalid-ip"
 message: "custom failure message"`)
}

func TestString_IsHex(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "abcdef123456").IsHex()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "abcdefg").IsHex()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "abcdefg"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "abcdefg").IsHex("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "abcdefg"
 message: "index is 0"`)

	// Test various valid hexadecimal strings
	m.Reset()
	assert.String(m, "0123456789ABCDEFabcdef").IsHex()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.String(m, "ffffffff").IsHex()
	assert.String(t, m.String()).Equal("")

	// Test various invalid hexadecimal strings
	m.Reset()
	assert.String(m, "").IsHex() // Empty string
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: ""`)

	m.Reset()
	assert.String(m, "xyz").IsHex() // Completely invalid
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "xyz"`)

	m.Reset()
	assert.String(m, "abc def").IsHex() // Space
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "abc def"`)

	m.Reset()
	assert.String(m, "0x123").IsHex() // Hex prefix
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "0x123"`)

	// Test with custom message
	m.Reset()
	assert.String(m, "invalid-hex").IsHex("This should be a valid hexadecimal string")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid hexadecimal, but it is not
  actual: "invalid-hex"
 message: "This should be a valid hexadecimal string"`)
}

func TestString_IsBase64(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test success case
	m.Reset()
	assert.String(m, "SGVsbG8gd29ybGQ=").IsBase64()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.String(m, "invalid-base64").IsBase64()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "invalid-base64"`)

	// Test with Require() and custom message
	m.Reset()
	require.String(m, "invalid-base64").IsBase64("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "invalid-base64"
 message: "index is 0"`)

	// Test various valid Base64 strings
	m.Reset()
	assert.String(m, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/").IsBase64()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.String(m, "YW55IGNhcm5hbCBwbGVhc3Vy").IsBase64() // "any carnal pleasur"
	assert.String(t, m.String()).Equal("")

	// Test various invalid Base64 strings
	m.Reset()
	assert.String(m, "=").IsBase64() // Just padding
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "="`)

	m.Reset()
	assert.String(m, "AAA==").IsBase64() // Too much padding
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "AAA=="`)

	m.Reset()
	assert.String(m, "123!").IsBase64() // Invalid character
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "123!"`)

	m.Reset()
	assert.String(m, "12 3").IsBase64() // Space
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "12 3"`)

	// Test with custom message
	m.Reset()
	assert.String(m, "invalid-base64!").IsBase64("This should be a valid Base64 string")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected string to be a valid Base64, but it is not
  actual: "invalid-base64!"
 message: "This should be a valid Base64 string"`)
}
