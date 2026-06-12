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
	"fmt"
	"strconv"
	"strings"

	"github.com/go-spring/stdlib/errutil"
)

// PathType represents the type of a path element in a hierarchical key.
// A path element can either be a key (map field) or an index (array/slice element).
type PathType int8

const (
	PathTypeKey   PathType = iota // A named key in a map.
	PathTypeIndex                 // A numeric index in a list.
)

// String returns the string representation of PathType.
func (t PathType) String() string {
	switch t {
	case PathTypeKey:
		return "key"
	case PathTypeIndex:
		return "index"
	default:
		return fmt.Sprintf("PathType(%d)", int8(t))
	}
}

// Path represents a single segment in a parsed key path.
// A path is composed of multiple Path elements that can be joined or split.
// For example, "foo.bar[0]" parses into:
//
//	[{Type: PathTypeKey, Elem: "foo"},
//	 {Type: PathTypeKey, Elem: "bar"},
//	 {Type: PathTypeIndex, Elem: "0"}].
type Path struct {
	// Whether the element is a key or an index.
	Type PathType

	// Actual key or index value as a string.
	// For PathTypeKey, it's the key string;
	// for PathTypeIndex, it's the index number as a string.
	Elem string
}

// JoinPath converts a slice of Path objects into a string representation.
// Keys are joined with dots, and array indices are wrapped in square brackets.
// Example: [key, index(0), key] => "key[0].key".
func JoinPath(path []Path) string {
	var sb strings.Builder
	for i, p := range path {
		switch p.Type {
		case PathTypeKey:
			if i > 0 {
				sb.WriteString(".")
			}
			sb.WriteString(p.Elem)
		case PathTypeIndex:
			sb.WriteString("[")
			sb.WriteString(p.Elem)
			sb.WriteString("]")
		}
	}
	return sb.String()
}

// SplitPath parses a hierarchical key string into a slice of Path objects.
// It supports dot-notation for maps and bracket-notation for arrays.
// Examples:
//
//	"foo.bar[0]" -> [{Key:"foo"}, {Key:"bar"}, {Index:"0"}]
//	"a[1][2]"    -> [{Key:"a"}, {Index:"1"}, {Index:"2"}]
//
// Rules:
//   - Keys must be non-empty strings without spaces.
//   - Indices must be unsigned integers (no sign, no decimal).
//   - Empty maps/slices are not special-cased here.
//   - Returns an error if the key is malformed (e.g. unbalanced brackets,
//     unexpected characters, or empty keys if disallowed).
func SplitPath(key string) (_ []Path, err error) {
	if key == "" {
		return nil, errutil.Explain(nil, "SplitPath: invalid key: empty string")
	}

	var (
		path        []Path
		lastPos     int  // start index of current segment
		lastChar    rune // previous rune seen (0 initial)
		openBracket bool // whether we're inside '[' ... ']'
	)

	for i, c := range key {
		switch c {
		case ' ':
			return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: contains space", key, i)
		case '.':
			if openBracket {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: '.' not allowed inside brackets", key, i)
			}
			if lastChar == '.' {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: empty key between dots", key, i)
			}
			if lastChar != ']' {
				if path, err = appendKey(path, key[lastPos:i]); err != nil {
					return nil, errutil.Explain(err, "SplitPath: invalid key %q at pos %d", key, lastPos)
				}
			}
			lastPos = i + 1
			lastChar = '.'
		case '[':
			if openBracket {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: nested '['", key, i)
			}
			if lastChar == '.' {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: '[' cannot directly follow '.'", key, i)
			}
			if i > 0 && lastChar != ']' {
				if path, err = appendKey(path, key[lastPos:i]); err != nil {
					return nil, errutil.Explain(err, "SplitPath: invalid key %q at pos %d", key, lastPos)
				}
			}
			openBracket = true
			lastPos = i + 1
			lastChar = '['
		case ']':
			if !openBracket {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: ']' without matching '['", key, i)
			}
			if lastPos == i {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: empty index", key, lastPos)
			}
			if path, err = appendIndex(path, key[lastPos:i]); err != nil {
				return nil, errutil.Explain(err, "SplitPath: invalid key %q at pos %d", key, lastPos)
			}
			openBracket = false
			lastPos = i + 1
			lastChar = ']'
		default:
			// if previous char was ']' and now we see other char that's not '.' or '[' it's invalid:
			if lastChar == ']' {
				return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: unexpected character %q after ']'", key, i, c)
			}
			lastChar = c
		}
	}

	if openBracket {
		return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: unclosed '['", key, lastPos-1)
	}
	if lastChar == '.' {
		return nil, errutil.Explain(nil, "SplitPath: invalid key %q at pos %d: ends with '.'", key, len(key)-1)
	}
	if lastChar != ']' {
		if path, err = appendKey(path, key[lastPos:]); err != nil {
			return nil, errutil.Explain(err, "SplitPath: invalid key %q at pos %d", key, lastPos)
		}
	}

	return path, nil
}

// appendKey validates and appends a key segment.
func appendKey(path []Path, s string) ([]Path, error) {
	if s == "" {
		return nil, errutil.Explain(nil, "empty key segment")
	}
	if strings.ContainsRune(s, ' ') {
		return nil, errutil.Explain(nil, "key segment %q contains space", s)
	}
	return append(path, Path{Type: PathTypeKey, Elem: s}), nil
}

// appendIndex validates and appends an index segment.
func appendIndex(path []Path, s string) ([]Path, error) {
	if s == "" {
		return nil, errutil.Explain(nil, "empty index")
	}
	if _, err := strconv.ParseUint(s, 10, 64); err != nil {
		return nil, errutil.Explain(nil, "index must be an unsigned integer (got %q)", s)
	}
	return append(path, Path{Type: PathTypeIndex, Elem: s}), nil
}
