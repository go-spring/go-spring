/*
 * Copyright 2012-2019 the original author or authors.
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
	"fmt"
	"strconv"
	"strings"
)

type PathType int

const (
	PathTypeKey   PathType = iota // PathTypeKey is map key like a/b in a[0][1].b
	PathTypeIndex                 // PathTypeIndex is array index like 0/1 in a[0][1].b
)

type Path struct {
	Type PathType
	Elem string
}

// JoinPath joins path elements into a single path.
func JoinPath(path []Path) string {
	var s strings.Builder
	for i, p := range path {
		switch p.Type {
		case PathTypeKey:
			if i > 0 {
				s.WriteString(".")
			}
			s.WriteString(p.Elem)
		case PathTypeIndex:
			s.WriteString("[")
			s.WriteString(p.Elem)
			s.WriteString("]")
		}
	}
	return s.String()
}

// SplitPath splits key into individual path elements.
func SplitPath(key string) ([]Path, error) {
	if key == "" {
		return nil, nil
	}
	var (
		path        []Path
		lastPos     int
		lastChar    int32
		openBracket bool
	)
	for i, c := range key {
		switch c {
		case ' ':
			return nil, fmt.Errorf("invalid key '%s'", key)
		case '.':
			if openBracket {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			if lastChar == ']' {
				lastPos = i + 1
				lastChar = c
				continue
			}
			if lastPos == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeKey, key[lastPos:i]})
			lastPos = i + 1
			lastChar = c
		case '[':
			if openBracket {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			if i == 0 || lastChar == ']' {
				lastPos = i + 1
				openBracket = true
				lastChar = c
				continue
			}
			if lastChar == '.' || lastPos == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeKey, key[lastPos:i]})
			lastPos = i + 1
			openBracket = true
			lastChar = c
		case ']':
			if !openBracket || lastPos == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			s := key[lastPos:i]
			_, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeIndex, s})
			lastPos = i + 1
			openBracket = false
			lastChar = c
		default:
			lastChar = c
		}
	}
	if openBracket || lastChar == '.' {
		return nil, fmt.Errorf("invalid key '%s'", key)
	}
	if lastChar != ']' {
		path = append(path, Path{PathTypeKey, key[lastPos:]})
	}
	return path, nil
}
