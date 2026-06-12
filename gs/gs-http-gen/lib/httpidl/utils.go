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
	"slices"
	"strings"
)

// IsPascal reports whether a string starts with an uppercase ASCII letter.
// It does not validate full PascalCase formatting.
func IsPascal(name string) bool {
	return name[0] >= 'A' && name[0] <= 'Z'
}

// ToPascal converts a snake_case identifier into a Pascal-style identifier
// by capitalizing the first letter of each underscore-separated segment.
// It does not normalize the case of the remaining characters.
func ToPascal(s string) string {
	var sb strings.Builder
	for part := range strings.SplitSeq(s, "_") {
		if part == "" {
			continue
		}
		c := part[0]
		if 'a' <= c && c <= 'z' {
			c = c - 'a' + 'A'
		}
		sb.WriteByte(c)
		if len(part) > 1 {
			sb.WriteString(part[1:])
		}
	}
	return sb.String()
}

// EnumRef represents a resolved enum definition and its location.
type EnumRef struct {
	Type  Enum
	File  string
	Index int
}

// FindEnum searches all documents for a non-extended enum type
// with the given name and returns its definition and location.
func FindEnum(files map[string]Document, name string) (EnumRef, bool) {
	for file, doc := range files {
		if i, ok := doc.EnumTypes[name]; ok {
			if e := doc.Enums[i]; e.Kind != EnumKindExtends {
				return EnumRef{Type: e, File: file, Index: i}, true
			}
		}
	}
	return EnumRef{}, false
}

// TypeRef represents a resolved type definition and its location.
type TypeRef struct {
	Type  Type
	File  string
	Index int
}

// FindType searches all documents for a type with the given name.
func FindType(files map[string]Document, name string) (TypeRef, bool) {
	for file, doc := range files {
		if i, ok := doc.TypeTypes[name]; ok {
			t := doc.Types[i]
			return TypeRef{Type: t, File: file, Index: i}, true
		}
	}
	return TypeRef{}, false
}

// FindAnnotation searches through a slice of annotations and returns
// the first annotation whose key matches any of the provided names.
func FindAnnotation(arr []Annotation, names ...string) (Annotation, bool) {
	for _, a := range arr {
		if slices.Contains(names, a.Key) {
			return a, true
		}
	}
	return Annotation{}, false
}
