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
	"slices"
	"strings"
	"sync/atomic"

	"github.com/go-spring/stdlib/ordered"
)

// tagRegistry stores Tag instances keyed by their string names.
// Note: This registry is not concurrency-safe and is intended to be
// used only during the initialization phase.
var tagRegistry = map[string]*Tag{}

// loggerValue wraps a Logger instance.
type loggerValue struct {
	Logger
}

// Tag represents a named log tag, used to categorize logs by
// subsystem, business domain, RPC interaction, and so on.
// Each Tag maintains a reference to a Logger instance.
type Tag struct {
	tag    string
	logger atomic.Pointer[loggerValue]
}

// reset resets the logger associated with the tag.
func (t *Tag) reset() {
	t.logger.Store(&loggerValue{})
}

// GetAllTags returns the names of all registered tags.
func GetAllTags() []string {
	return ordered.MapKeys(tagRegistry)
}

// isValidTag validates a tag string according to the following rules:
//
//  1. Length must be between 3 and 36 characters.
//  2. Allowed characters: lowercase letters (a-z), digits (0-9), underscores (_).
//  3. The tag may optionally start with a single underscore.
//  4. After removing the optional leading underscore, the tag consists of
//     1 to 4 non-empty segments separated by underscores.
//  5. Consecutive underscores or trailing underscores are not allowed.
func isValidTag(tag string) bool {
	if len(tag) < 3 || len(tag) > 36 {
		return false
	}
	for i := range len(tag) {
		c := tag[i]
		// nolint: staticcheck
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '_' {
			return false
		}
	}
	ss := strings.Split(strings.TrimPrefix(tag, "_"), "_")
	if len(ss) < 1 || len(ss) > 4 {
		return false
	}
	return !slices.Contains(ss, "")
}

// RegisterTag retrieves or creates a Tag by name.
// If the tag is not yet registered, a new Tag is created and initialized.
//
// This function must be called during initialization. It panics if invoked
// after the logging system has been refreshed (i.e., when global.refreshed
// is already set).
//
// Normally, higher-level helpers such as RegisterAppTag, RegisterBizTag,
// or RegisterRPCTag should be used to enforce semantic consistency.
func RegisterTag(tag string) *Tag {
	if !isValidTag(tag) {
		panic("invalid log tag")
	}
	if global.refreshed {
		panic("log refresh already done")
	}
	m, ok := tagRegistry[tag]
	if !ok {
		m = &Tag{tag: tag}
		m.reset()
		tagRegistry[tag] = m
	}
	return m
}

// BuildTag constructs a structured tag string from mainType, subType,
// and an optional action.
//
// The resulting format is:
//
//	_<mainType>_<subType>
//	_<mainType>_<subType>_<action>
//
// Example:
//
//	BuildTag("app", "startup", "init") -> "_app_startup_init"
func BuildTag(mainType, subType, action string) string {
	if subType == "" {
		panic("subType cannot be empty")
	}
	if action == "" {
		return "_" + mainType + "_" + subType
	}
	return "_" + mainType + "_" + subType + "_" + action
}
