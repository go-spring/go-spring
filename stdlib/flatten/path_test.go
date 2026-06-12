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

package flatten

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestPath(t *testing.T) {

	// Test cases for SplitPath function
	splitPathTestCases := []struct {
		name     string
		key      string
		expected []Path
		err      string
	}{
		// Error cases
		{
			name: "empty key",
			key:  "",
			err:  "SplitPath: invalid key: empty string",
		},
		{
			name: "space key",
			key:  " ",
			err:  "SplitPath: invalid key \" \" at pos 0: contains space",
		},
		{
			name: "single dot",
			key:  ".",
			err:  "SplitPath: invalid key \".\" at pos 0: empty key segment",
		},
		{
			name: "double dot",
			key:  "..",
			err:  "SplitPath: invalid key \"..\" at pos 0: empty key segment",
		},
		{
			name: "unmatched opening bracket",
			key:  "[",
			err:  "SplitPath: invalid key \"[\" at pos 0: unclosed '['",
		},
		{
			name: "unmatched closing bracket",
			key:  "]",
			err:  "SplitPath: invalid key \"]\" at pos 0: ']' without matching '['",
		},
		{
			name: "empty brackets",
			key:  "[]",
			err:  "SplitPath: invalid key \"[]\" at pos 1: empty index",
		},
		{
			name: "invalid index",
			key:  "[a]",
			err:  "SplitPath: invalid key \"[a]\" at pos 1: index must be an unsigned integer (got \"a\")",
		},
		{
			name: "dot after key",
			key:  "a.",
			err:  "SplitPath: invalid key \"a.\" at pos 1: ends with '.'",
		},
		{
			name: "dot after dot",
			key:  "a..b",
			err:  "SplitPath: invalid key \"a..b\" at pos 2: empty key between dots",
		},
		{
			name: "unmatched opening bracket in path",
			key:  "a[",
			err:  "SplitPath: invalid key \"a[\" at pos 1: unclosed '['",
		},
		{
			name: "unmatched closing bracket in path",
			key:  "a]",
			err:  "SplitPath: invalid key \"a]\" at pos 1: ']' without matching '['",
		},
		{
			name: "dot after bracket",
			key:  "a.[0]",
			err:  "SplitPath: invalid key \"a.[0]\" at pos 2: '[' cannot directly follow '.'",
		},
		{
			name: "dot after closing bracket",
			key:  "a[0]..b",
			err:  "SplitPath: invalid key \"a[0]..b\" at pos 5: empty key between dots",
		},
		{
			name: "characters after closing bracket",
			key:  "a[0]b",
			err:  "SplitPath: invalid key \"a[0]b\" at pos 4: unexpected character 'b' after ']'",
		},
		{
			name: "negative index",
			key:  "[-1]",
			err:  "SplitPath: invalid key \"[-1]\" at pos 1: index must be an unsigned integer (got \"-1\")",
		},
		{
			name: "dot inside brackets",
			key:  "[.]",
			err:  "SplitPath: invalid key \"[.]\" at pos 1: '.' not allowed inside brackets",
		},
		{
			name: "nested opening bracket",
			key:  "a[[0]]",
			err:  "SplitPath: invalid key \"a[[0]]\" at pos 2: nested '['",
		},
		{
			name: "index overflow",
			key:  "a[18446744073709551616]", // uint64 max + 1
			err:  "SplitPath: invalid key \"a[18446744073709551616]\" at pos 2: index must be an unsigned integer (got \"18446744073709551616\")",
		},

		// Valid cases
		{
			name: "simple key",
			key:  "a",
			expected: []Path{
				{PathTypeKey, "a"},
			},
		},
		{
			name: "simple index",
			key:  "[0]",
			expected: []Path{
				{PathTypeIndex, "0"},
			},
		},
		{
			name: "key with index",
			key:  "a[0]",
			expected: []Path{
				{PathTypeKey, "a"},
				{PathTypeIndex, "0"},
			},
		},
		{
			name: "key starting with digit",
			key:  "0[0]",
			expected: []Path{
				{PathTypeKey, "0"},
				{PathTypeIndex, "0"},
			},
		},
		{
			name: "nested indices",
			key:  "a[0][1]",
			expected: []Path{
				{PathTypeKey, "a"},
				{PathTypeIndex, "0"},
				{PathTypeIndex, "1"},
			},
		},
		{
			name: "key with dot notation",
			key:  "a.b",
			expected: []Path{
				{PathTypeKey, "a"},
				{PathTypeKey, "b"},
			},
		},
		{
			name: "key with dot and index",
			key:  "a.0.b",
			expected: []Path{
				{PathTypeKey, "a"},
				{PathTypeKey, "0"},
				{PathTypeKey, "b"},
			},
		},
		{
			name: "key with index and dot",
			key:  "a[0].b",
			expected: []Path{
				{PathTypeKey, "a"},
				{PathTypeIndex, "0"},
				{PathTypeKey, "b"},
			},
		},
		{
			name: "key with special characters",
			key:  "_key-name.test_1",
			expected: []Path{
				{PathTypeKey, "_key-name"},
				{PathTypeKey, "test_1"},
			},
		},
	}

	for _, tc := range splitPathTestCases {
		t.Run("SplitPath/"+tc.name, func(t *testing.T) {
			p, err := SplitPath(tc.key)
			if tc.err != "" {
				assert.That(t, err).NotNil()
				assert.Error(t, err).String(tc.err)
				return
			}

			assert.That(t, err).Nil()
			assert.That(t, p).Equal(tc.expected)
			assert.That(t, JoinPath(p)).Equal(tc.key)
		})
	}

	// Test cases for JoinPath function
	joinPathTestCases := []struct {
		name     string
		path     []Path
		expected string
	}{
		{
			name:     "empty path",
			path:     []Path{},
			expected: "",
		},
		{
			name: "single key",
			path: []Path{
				{PathTypeKey, "a"},
			},
			expected: "a",
		},
		{
			name: "single index",
			path: []Path{
				{PathTypeIndex, "0"},
			},
			expected: "[0]",
		},
		{
			name: "multiple keys",
			path: []Path{
				{PathTypeKey, "a"},
				{PathTypeKey, "b"},
			},
			expected: "a.b",
		},
		{
			name: "key with index",
			path: []Path{
				{PathTypeKey, "a"},
				{PathTypeIndex, "0"},
			},
			expected: "a[0]",
		},
		{
			name: "multiple indices",
			path: []Path{
				{PathTypeIndex, "0"},
				{PathTypeIndex, "1"},
			},
			expected: "[0][1]",
		},
		{
			name: "key with special characters",
			path: []Path{
				{PathTypeKey, "_key-name"},
				{PathTypeKey, "test_1"},
			},
			expected: "_key-name.test_1",
		},
		{
			name: "index then key",
			path: []Path{
				{PathTypeIndex, "0"},
				{PathTypeKey, "a"},
			},
			expected: "[0].a",
		},
	}

	for _, tc := range joinPathTestCases {
		t.Run("JoinPath/"+tc.name, func(t *testing.T) {
			result := JoinPath(tc.path)
			assert.That(t, result).Equal(tc.expected)

			// Verify round-trip conversion
			if len(tc.path) > 0 {
				p, err := SplitPath(tc.expected)
				assert.That(t, err).Nil()
				assert.That(t, p).Equal(tc.path)
			}
		})
	}

	// Test cases for appendKey function
	t.Run("appendKey/valid key", func(t *testing.T) {
		path := []Path{}
		path, err := appendKey(path, "key")
		assert.That(t, err).Nil()
		assert.That(t, path).Equal([]Path{{PathTypeKey, "key"}})
	})

	t.Run("appendKey/empty key", func(t *testing.T) {
		_, err := appendKey([]Path{}, "")
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("empty key segment")
	})

	t.Run("appendKey/key with space", func(t *testing.T) {
		_, err := appendKey([]Path{}, "key with space")
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("contains space")
	})

	t.Run("appendKey/append to existing path", func(t *testing.T) {
		path := []Path{{PathTypeKey, "existing"}}
		path, err := appendKey(path, "new")
		assert.That(t, err).Nil()
		assert.That(t, path).Equal([]Path{
			{PathTypeKey, "existing"},
			{PathTypeKey, "new"},
		})
	})

	// Test cases for appendIndex function
	t.Run("appendIndex/valid index", func(t *testing.T) {
		var path []Path
		path, err := appendIndex(path, "0")
		assert.That(t, err).Nil()
		assert.That(t, path).Equal([]Path{{PathTypeIndex, "0"}})
	})

	t.Run("appendIndex/empty index", func(t *testing.T) {
		_, err := appendIndex([]Path{}, "")
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("empty index")
	})

	t.Run("appendIndex/invalid index", func(t *testing.T) {
		_, err := appendIndex([]Path{}, "abc")
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("index must be an unsigned integer")
	})

	t.Run("appendIndex/negative index", func(t *testing.T) {
		_, err := appendIndex([]Path{}, "-1")
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("index must be an unsigned integer")
	})

	t.Run("appendIndex/large index", func(t *testing.T) {
		path, err := appendIndex([]Path{}, "18446744073709551615")
		assert.That(t, err).Nil()
		assert.That(t, path).Equal([]Path{{PathTypeIndex, "18446744073709551615"}})
	})

	t.Run("appendIndex/append to existing path", func(t *testing.T) {
		path := []Path{{PathTypeKey, "existing"}}
		path, err := appendIndex(path, "1")
		assert.That(t, err).Nil()
		assert.That(t, path).Equal([]Path{
			{PathTypeKey, "existing"},
			{PathTypeIndex, "1"},
		})
	})
}
