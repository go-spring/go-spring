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
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/ordered"
)

// LoggerInfo describes a configured logger and its effective level range,
// as reported to operational tooling (e.g. an actuator "loggers" endpoint).
type LoggerInfo struct {
	// Name is the configured logger name (RootLoggerName for the root logger).
	Name string
	// Level is the effective minimum level (upper-case, e.g. "INFO"). It
	// reflects a runtime override if one has been applied via SetLoggerLevel.
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

// SetLoggerLevel overrides the level range of a configured logger at runtime.
//
// name is a logger name reported by Loggers (use RootLoggerName for the root
// logger). level is parsed by ParseLevelRange, so it accepts forms like "INFO"
// or "INFO~ERROR". The change takes effect immediately for subsequent events.
// It returns an error for an unknown logger name or an invalid level.
func SetLoggerLevel(name, level string) error {
	r, err := ParseLevelRange(level)
	if err != nil {
		return err
	}

	global.mutex.Lock()
	defer global.mutex.Unlock()

	l, ok := global.named[name]
	if !ok {
		return errutil.Explain(nil, "logger %q not found", name)
	}
	l.SetLevel(r)
	return nil
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
