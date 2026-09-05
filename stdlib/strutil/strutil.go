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

// Package strutil holds small string helpers.
package strutil

import "unicode/utf8"

// Truncate shortens s to at most max bytes in total, ending with "..." when
// something was cut, with the cut landing on a rune boundary. A max that
// leaves no room for content plus the marker degenerates to a raw byte cut.
// Use it to bound how much of a payload ends up in a log line or span
// attribute.
func Truncate(s string, max int) string {
	const marker = "..."
	if len(s) <= max {
		return s
	}
	cut := max - len(marker)
	if cut <= 0 {
		return s[:max]
	}
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + marker
}
