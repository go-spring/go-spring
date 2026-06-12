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

package log

import (
	"fmt"
	"strings"

	"github.com/go-spring/stdlib/errutil"
)

func init() {
	RegisterConverter(ParseLevelRange)
}

var (
	NoneLevel  = RegisterLevel(0, "NONE")    // No logging
	TraceLevel = RegisterLevel(100, "TRACE") // Very detailed logging, typically used for debugging
	DebugLevel = RegisterLevel(200, "DEBUG") // Debugging information useful during development
	InfoLevel  = RegisterLevel(300, "INFO")  // General informational messages about application progress
	WarnLevel  = RegisterLevel(400, "WARN")  // Warnings about potential issues or unusual situations
	ErrorLevel = RegisterLevel(500, "ERROR") // Errors that allow the application to continue running
	PanicLevel = RegisterLevel(600, "PANIC") // Severe issues that may cause a panic in the application
	FatalLevel = RegisterLevel(700, "FATAL") // Critical issues that will terminate the application
	MaxLevel   = RegisterLevel(999, "MAX")   // Maximum level (used as the upper bound for comparisons)
)

var levelRegistry = map[string]Level{}

// Level represents a logging severity level. Each level
// has a numeric code (for comparison) and a string name (for display).
type Level struct {
	code      int32
	lowerName string
	upperName string
}

// Code returns the numeric code of the Level.
// Levels with higher codes represent higher severity.
func (l Level) Code() int32 {
	return l.code
}

// LowerName returns the lowercase name of the Level.
func (l Level) LowerName() string {
	return l.lowerName
}

// UpperName returns the uppercase name of the Level.
func (l Level) UpperName() string {
	return l.upperName
}

// RegisterLevel defines a new logging Level with the given code and name.
// The name is normalized to uppercase and stored in a global registry for lookup.
//
// It must be called during initialization only and is not safe for concurrent use.
// It panics if the same name is registered with a different code.
//
// Multiple names may share the same code (aliases). Such levels are considered
// equivalent in comparisons (by code), but remain distinct values.
func RegisterLevel(code int32, name string) Level {
	if l, ok := levelRegistry[strings.ToUpper(name)]; ok {
		if l.code == code {
			return l
		}
		panic(fmt.Sprintf("log: level %s already registered with a different code", name))
	}
	l := Level{
		code:      code,
		lowerName: strings.ToLower(name),
		upperName: strings.ToUpper(name),
	}
	levelRegistry[l.upperName] = l
	return l
}

// LevelRange represents a range of log levels [MinLevel, MaxLevel).
type LevelRange struct {
	MinLevel Level
	MaxLevel Level
}

// Enable returns true if the given Level 'l' falls within the LevelRange.
// The check is inclusive of MinLevel and exclusive of MaxLevel.
func (c LevelRange) Enable(l Level) bool {
	return l.code >= c.MinLevel.code && l.code < c.MaxLevel.code
}

// ParseLevelRange parses a string into a LevelRange.
//
// Supported formats:
//
//	""           → [NONE, MAX)
//	"INFO"       → [INFO, MAX)
//	"INFO~ERROR" → [INFO, ERROR)
//
// The comparison is case-insensitive. Returns an error for unknown levels.
func ParseLevelRange(s string) (LevelRange, error) {
	if s = strings.TrimSpace(s); s == "" {
		return LevelRange{
			MinLevel: NoneLevel,
			MaxLevel: MaxLevel,
		}, nil
	}

	var (
		ok       bool
		minLevel = NoneLevel
		maxLevel = MaxLevel
	)

	ss := strings.Split(s, "~")
	if len(ss) > 2 {
		return LevelRange{}, errutil.Explain(nil, "invalid log level: %q", s)
	}
	s0 := strings.TrimSpace(ss[0])
	minLevel, ok = levelRegistry[strings.ToUpper(s0)]
	if !ok {
		return LevelRange{}, errutil.Explain(nil, "invalid log level: %q", s0)
	}
	if len(ss) == 2 {
		if s1 := strings.TrimSpace(ss[1]); s1 == "" {
			maxLevel = MaxLevel
		} else {
			maxLevel, ok = levelRegistry[strings.ToUpper(s1)]
			if !ok {
				return LevelRange{}, errutil.Explain(nil, "invalid log level: %q", s1)
			}
		}
	}
	if maxLevel.code <= minLevel.code {
		return LevelRange{}, errutil.Explain(nil, "invalid log level: %q", s)
	}

	return LevelRange{MinLevel: minLevel, MaxLevel: maxLevel}, nil
}
