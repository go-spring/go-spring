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

// Package label is the governance service-label convention. Both traffic
// directions and every protocol adapter build labels with [ServiceLabel]: a
// client names the dependency it calls ("redis:cache"), a server names the
// entrance it presents ("gin:0.0.0.0:8080").
//
// The identical string is what lets one governance rule match a client executor
// and an inbound guard, so the join must not be re-implemented per starter — a
// drift between the two sides would silently stop the rule from matching. The
// package stays a dependency-free leaf; it lives under the governance theme
// because the convention is part of governance, not of any one direction.
package label

// ServiceLabel joins prefix with the first non-empty name in names, falling
// back to prefix alone when none is set.
//
//	label.ServiceLabel("redis", c.ServiceName, c.MasterName, c.Addr)
//	label.ServiceLabel("gin", cfg.Address)
func ServiceLabel(prefix string, names ...string) string {
	for _, n := range names {
		if n != "" {
			return prefix + ":" + n
		}
	}
	return prefix
}
