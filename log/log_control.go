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
	"go-spring.org/stdlib/ordered"
)

// LoggerInfo describes a configured logger and its effective level range,
// as reported to operational tooling (e.g. an actuator "loggers" endpoint).
type LoggerInfo struct {
	// Name is the configured logger name (RootLoggerName for the root logger).
	Name string
	// Level is the effective minimum level (upper-case, e.g. "INFO").
	Level string
}

// Loggers returns the configured loggers and their current effective levels.
//
// It reports the loggers defined by the last successful Refresh (including the
// root logger); it returns an empty slice before the logging system has been
// refreshed. The result is sorted by name for stable output.
func Loggers() []LoggerInfo {
	global.mutex.Lock()
	defer global.mutex.Unlock()

	out := make([]LoggerInfo, 0, len(global.named))
	for _, name := range ordered.MapKeys(global.named) {
		out = append(out, LoggerInfo{
			Name:  name,
			Level: global.named[name].GetLevel().MinLevel.UpperName(),
		})
	}
	return out
}


// AvailableLevels returns the selectable log level names (upper-case), ordered
// from most to least verbose. Bounds-only levels (NONE, MAX) are excluded.
func AvailableLevels() []string {
	levels := []Level{
		TraceLevel, DebugLevel, InfoLevel,
		WarnLevel, ErrorLevel, PanicLevel, FatalLevel,
	}
	names := make([]string, len(levels))
	for i, l := range levels {
		names[i] = l.UpperName()
	}
	return names
}
