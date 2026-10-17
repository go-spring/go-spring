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

// Package label is the neutral home of the governance service-label convention.
// Both traffic directions build labels with it: a client names the dependency it
// calls ("redis:cache"), a server names the entrance it presents
// ("gin:0.0.0.0:8080"). It is a leaf package so neither direction imports the
// other just to spell a label.
package label

// ServiceLabel joins prefix with the first non-empty name in names, falling
// back to prefix alone when none is set.
//
//	resilience.ServiceLabel("redis", c.ServiceName, c.MasterName, c.Addr)
//	label.ServiceLabel("gin", cfg.Address)
func ServiceLabel(prefix string, names ...string) string {
	for _, n := range names {
		if n != "" {
			return prefix + ":" + n
		}
	}
	return prefix
}
