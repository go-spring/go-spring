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

package discovery

// Addressing is the config block every discovery-citing client starter shares:
// the two keys that switch an entry from direct addressing to discovery
// routing. Embed it in the starter's Config; the relative value tags resolve
// under whatever block prefix the entry binds to (the same embedding
// security.TLSConfig uses across the starter families).
//
// The tags are deliberately plain — the family-wide default fallback
// (${<family>.default.discovery}) cannot live here: its key carries the family
// prefix, and tags are static strings. Each starter's wiring binds the resolved
// label as a value argument instead, with the fallback in the tag:
//
//	${<family>.instances.<name>.discovery:=${<family>.default.discovery:=none}}
//
// The starter then resolves that label against the discovery directory the
// governance center carries (discovery.Manager.Get), so "none" — the sentinel
// that names nothing — reads a nil backend.
//
// Resolution order: the entry's own ${discovery} wins; unset falls back to the
// family default; both unset while service-name is set fails loud at
// assembly. Citing a label that names no backend also fails loud.
//
// A ${scheme} key is deliberately NOT here: http-client has no such key and
// elasticsearch gives it a different meaning, so each family keeps its own.
type Addressing struct {
	// ServiceName routes the entry through service discovery (and load
	// balancing) instead of a fixed address. Empty means direct addressing;
	// when set, Discovery must resolve to a backend bean or assembly fails.
	ServiceName string `value:"${service-name:=}"`

	// Discovery is the label of the discovery backend that resolves
	// ServiceName (label = bean name = "<backend>.<name>", registered by a
	// discovery starter). Empty means unset: no discovery backend is wired, and
	// the entry must not route by service-name alone. Falls back to the
	// family-wide ${<family>.default.discovery} via the starter's wiring, not
	// this tag.
	Discovery string `value:"${discovery:=}"`
}
