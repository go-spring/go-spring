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

package internal

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// StringAssertion encapsulates a string value and a test handler for making assertions on the string.
type StringAssertion struct {
	AssertionBase
	v string
}

// ThatString returns a StringAssertion for the given testing object and string value.
func ThatString(t TestingT, v string, fatalOnFailure bool) *StringAssertion {
	return &StringAssertion{
		AssertionBase: AssertionBase{
			t:              t,
			fatalOnFailure: fatalOnFailure,
		},
		v: v,
	}
}

// Length reports a test failure if the actual string's length is not equal to the expected length.
func (a *StringAssertion) Length(length int, msg ...string) *StringAssertion {
	a.t.Helper()
	if len(a.v) != length {
		str := fmt.Sprintf(`expected string to have length %d, but it has length %d
  actual: %q`, length, len(a.v), a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Blank reports a test failure if the actual string is not blank (i.e., contains non-whitespace characters).
func (a *StringAssertion) Blank(msg ...string) *StringAssertion {
	a.t.Helper()
	if strings.TrimSpace(a.v) != "" {
		str := fmt.Sprintf(`expected string to contain only whitespace, but it does not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotBlank reports a test failure if the actual string is blank (i.e., empty or contains only whitespace characters).
func (a *StringAssertion) NotBlank(msg ...string) *StringAssertion {
	a.t.Helper()
	if strings.TrimSpace(a.v) == "" {
		str := fmt.Sprintf(`expected string to be non-blank, but it is blank
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Equal reports a test failure if the actual string is not equal to the expected string.
func (a *StringAssertion) Equal(expect string, msg ...string) *StringAssertion {
	a.t.Helper()
	if a.v != expect {
		str := fmt.Sprintf(`expected strings to be equal, but they are not
  actual: %q
expected: %q`, a.v, expect)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotEqual reports a test failure if the actual string is equal to the given string.
func (a *StringAssertion) NotEqual(expect string, msg ...string) *StringAssertion {
	a.t.Helper()
	if a.v == expect {
		str := fmt.Sprintf(`expected strings to be different, but they are equal
  actual: %q
expected: %q`, a.v, expect)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// EqualFold reports a test failure if the actual string and the given string
// are not equal under Unicode case-folding.
func (a *StringAssertion) EqualFold(expect string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.EqualFold(a.v, expect) {
		str := fmt.Sprintf(`expected strings to be equal (case-insensitive), but they are not
  actual: %q
expected: %q`, a.v, expect)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// JSONEqual unmarshals both the actual and expected JSON strings into generic interfaces,
// then reports a test failure if their resulting structures are not deeply equal.
// If either string is invalid JSON, the test will fail with the unmarshal error.
func (a *StringAssertion) JSONEqual(expect string, msg ...string) *StringAssertion {
	a.t.Helper()
	var actualJSON any
	if err := json.Unmarshal([]byte(a.v), &actualJSON); err != nil {
		str := fmt.Sprintf(`expected strings to be JSON-equal, but failed to unmarshal actual value
  actual: %q
   error: %q`, a.v, err.Error())
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	var expectedJSON any
	if err := json.Unmarshal([]byte(expect), &expectedJSON); err != nil {
		str := fmt.Sprintf(`expected strings to be JSON-equal, but failed to unmarshal expected value
expected: %q
   error: %q`, expect, err.Error())
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	if !reflect.DeepEqual(actualJSON, expectedJSON) {
		str := fmt.Sprintf(`expected strings to be JSON-equal, but they are not
  actual: %q
expected: %q`, a.v, expect)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Matches reports a test failure if the actual string does not match the given regular expression.
func (a *StringAssertion) Matches(pattern string, msg ...string) *StringAssertion {
	a.t.Helper()
	if ok, err := regexp.MatchString(pattern, a.v); !ok {
		str := fmt.Sprintf(`expected string to match the pattern, but it does not
  actual: %q
 pattern: %q`, a.v, pattern)
		if err != nil {
			str += fmt.Sprintf("\n   error: %q", err.Error())
		}
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// HasPrefix fails the test if the actual string does not start with the specified prefix.
func (a *StringAssertion) HasPrefix(prefix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasPrefix(a.v, prefix) {
		str := fmt.Sprintf(`expected string to start with the specified prefix, but it does not
  actual: %q
  prefix: %q`, a.v, prefix)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// HasSuffix fails the test if the actual string does not end with the specified suffix.
func (a *StringAssertion) HasSuffix(suffix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasSuffix(a.v, suffix) {
		str := fmt.Sprintf(`expected string to end with the specified suffix, but it does not
  actual: %q
  suffix: %q`, a.v, suffix)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Contains fails the test if the actual string does not contain the specified substring.
func (a *StringAssertion) Contains(substr string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.Contains(a.v, substr) {
		str := fmt.Sprintf(`expected string to contain the specified substring, but it does not
  actual: %q
     sub: %q`, a.v, substr)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsLowerCase reports a test failure if the actual string contains any uppercase characters.
func (a *StringAssertion) IsLowerCase(msg ...string) *StringAssertion {
	a.t.Helper()
	if a.v != strings.ToLower(a.v) {
		str := fmt.Sprintf(`expected string to be all lowercase, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsUpperCase reports a test failure if the actual string contains any lowercase characters.
func (a *StringAssertion) IsUpperCase(msg ...string) *StringAssertion {
	a.t.Helper()
	if a.v != strings.ToUpper(a.v) {
		str := fmt.Sprintf(`expected string to be all uppercase, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsNumeric reports a test failure if the actual string contains any non-numeric characters.
func (a *StringAssertion) IsNumeric(msg ...string) *StringAssertion {
	a.t.Helper()
	for _, r := range a.v {
		if r < '0' || r > '9' {
			str := fmt.Sprintf(`expected string to contain only digits, but it does not
  actual: %q`, a.v)
			Fail(a.t, a.fatalOnFailure, str, msg...)
			break
		}
	}
	return a
}

// IsAlpha reports a test failure if the actual string contains any non-alphabetic characters.
func (a *StringAssertion) IsAlpha(msg ...string) *StringAssertion {
	a.t.Helper()
	for _, r := range a.v {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			str := fmt.Sprintf(`expected string to contain only letters, but it does not
  actual: %q`, a.v)
			Fail(a.t, a.fatalOnFailure, str, msg...)
			break
		}
	}
	return a
}

// IsAlphaNumeric reports a test failure if the actual string contains any non-alphanumeric characters.
func (a *StringAssertion) IsAlphaNumeric(msg ...string) *StringAssertion {
	a.t.Helper()
	for _, r := range a.v {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			str := fmt.Sprintf(`expected string to contain only letters and digits, but it does not
  actual: %q`, a.v)
			Fail(a.t, a.fatalOnFailure, str, msg...)
			break
		}
	}
	return a
}

// IsEmail reports a test failure if the actual string is not a valid email address.
func (a *StringAssertion) IsEmail(msg ...string) *StringAssertion {
	a.t.Helper()
	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	if ok, err := regexp.MatchString(emailRegex, a.v); err != nil || !ok {
		str := fmt.Sprintf(`expected string to be a valid email, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsURL reports a test failure if the actual string is not a valid URL.
func (a *StringAssertion) IsURL(msg ...string) *StringAssertion {
	a.t.Helper()
	urlRegex := `^(https?|ftp):\/\/[^\s/$.?#].[^\s]*$`
	if ok, err := regexp.MatchString(urlRegex, a.v); err != nil || !ok {
		str := fmt.Sprintf(`expected string to be a valid URL, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsIPv4 reports a test failure if the actual string is not a valid IPv4 address.
func (a *StringAssertion) IsIPv4(msg ...string) *StringAssertion {
	a.t.Helper()
	ipRegex := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	if ok, err := regexp.MatchString(ipRegex, a.v); err != nil || !ok {
		str := fmt.Sprintf(`expected string to be a valid IP, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsHex reports a test failure if the actual string is not a valid hexadecimal number.
func (a *StringAssertion) IsHex(msg ...string) *StringAssertion {
	a.t.Helper()
	hexRegex := `^[0-9a-fA-F]+$`
	if ok, err := regexp.MatchString(hexRegex, a.v); err != nil || !ok {
		str := fmt.Sprintf(`expected string to be a valid hexadecimal, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsBase64 reports a test failure if the actual string is not a valid Base64 encoded string.
func (a *StringAssertion) IsBase64(msg ...string) *StringAssertion {
	a.t.Helper()
	base64Regex := `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`
	if ok, err := regexp.MatchString(base64Regex, a.v); err != nil || !ok {
		str := fmt.Sprintf(`expected string to be a valid Base64, but it is not
  actual: %q`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}
