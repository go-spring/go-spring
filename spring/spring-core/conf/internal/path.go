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
	"strings"
)

type PathType int

const (
	PathTypeKey PathType = iota
	PathTypeIndex
)

type Path struct {
	Type PathType
	Elem string
}

// SplitPath splits the key into individual parts.
func SplitPath(key string) ([]Path, error) {
	if key == "" {
		return nil, nil
	}
	var (
		path        []Path
		lastChar    int32
		lastIndex   int
		leftBracket bool
	)
	for i, c := range key {
		switch c {
		case ' ':
			return nil, fmt.Errorf("invalid key '%s'", key)
		case '.':
			if leftBracket {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			if lastChar == ']' {
				lastIndex = i + 1
				lastChar = c
				continue
			}
			if lastIndex == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeKey, key[lastIndex:i]})
			lastIndex = i + 1
			lastChar = c
		case '[':
			if leftBracket {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			if i == 0 || lastChar == ']' {
				lastIndex = i + 1
				leftBracket = true
				lastChar = c
				continue
			}
			if lastChar == '.' || lastIndex == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeKey, key[lastIndex:i]})
			lastIndex = i + 1
			leftBracket = true
			lastChar = c
		case ']':
			if !leftBracket || lastIndex == i {
				return nil, fmt.Errorf("invalid key '%s'", key)
			}
			path = append(path, Path{PathTypeIndex, key[lastIndex:i]})
			lastIndex = i + 1
			leftBracket = false
			lastChar = c
		default:
			lastChar = c
		}
	}
	if leftBracket || lastChar == '.' {
		return nil, fmt.Errorf("invalid key '%s'", key)
	}
	if lastChar != ']' {
		path = append(path, Path{PathTypeKey, key[lastIndex:]})
	}
	return path, nil
}

func GenPath(path []Path) string {
	var s strings.Builder
	for i, p := range path {
		if p.Type == PathTypeKey {
			if i > 0 {
				s.WriteString(".")
			}
			s.WriteString(p.Elem)
			continue
		}
		if p.Type == PathTypeIndex {
			s.WriteString("[")
			s.WriteString(p.Elem)
			s.WriteString("]")
			continue
		}
	}
	return s.String()
}
