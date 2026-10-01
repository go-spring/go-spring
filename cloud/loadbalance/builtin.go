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

package loadbalance

// The built-in strategies, one [Factory] per file. They are the entries every
// [Directory] starts from; a deployment adds its own beside them rather than
// registering into a package-level table, so there is exactly one way to make a
// strategy available.
func builtinFactories() map[string]Factory {
	return map[string]Factory{
		RoundRobin:     roundRobinFactory{},
		LeastConn:      leastConnFactory{},
		ConsistentHash: consistentHashFactory{},
		Weighted:       weightedFactory{},
		ZoneAware:      zoneAwareFactory{},
		Random:         randomFactory{},
		P2C:            p2cFactory{},
	}
}
