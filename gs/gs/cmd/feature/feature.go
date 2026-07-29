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

// Package feature models the layout's feature manifest that drives `gs init`
// (and, later, `gs add`) customization.
//
// # Design constraints (fixed before layout is finalized)
//
// `gs init` customization is a *pruning* problem: the layout ships a full
// superset (every protocol server, every controller variant, every starter),
// and init deletes what the user did not select. The vocabulary of selectable
// units is a *feature*.
//
//   - One feature == one vertical slice: its IDL dir, its server dir, its
//     controller/converter variants, its blank imports in internal/init.go, and
//     its starter. A feature owns exactly those artifacts (see Owns) so gs can
//     prune without hardcoding any layout path.
//   - The flag name IS the feature key IS the manifest key - a single
//     vocabulary shared by `gs init` and `gs add`. e.g. flag `--gorm-mysql`
//     resolves to the manifest feature keyed "gorm-mysql".
//   - Naming favors expressiveness: a framework-protocol name is used only when
//     one framework carries several protocols/backends (--kitex-thrift,
//     --kitex-pb, --dubbo-triple, --gorm-mysql); otherwise the shortest
//     unambiguous name (--grpc, --gozero, --http). Symmetry is not a goal.
//   - Features are strictly independent: no feature declares a dependency on
//     another. The layout is authored to keep them decoupled.
//   - Category is metadata for grouping in `--list-features` output only; users
//     never type it.
//
// # Flag registration: the manifest is compiled into gs
//
// cobra/pflag registers flags *before* parsing argv, and the feature set
// defines those flags - so the feature list cannot be discovered at runtime
// from the cloned layout. It is therefore compiled into the gs binary (see
// features.json). gs code stays generic over this data: adding a feature is a
// JSON edit + gs rebuild, not a Go change. The tradeoff is that the feature set
// is a build-time property of gs and must be kept in sync with the layout
// superset it prunes.
//
// This package provides the layout-agnostic model, parser, and prune
// primitives; wiring them into the init command follows once the layout freezes.
package feature

import (
	_ "embed"
	"encoding/json"
	"sync"

	"go-spring.org/stdlib/errutil"
)

// ModulePlaceholder is the token every layout file uses in place of the real
// Go module path. Prune/Copy resolve it from replaces so copied artifacts and
// the import lines inserted into internal/init.go carry the project's real
// path. Cmd builds the replaces map with this key.
const ModulePlaceholder = "GS_PROJECT_MODULE"

// Manifest is the parsed features.json at the layout root.
type Manifest struct {
	Features []Feature `json:"features"`
}

// Feature is one selectable vertical slice. Key doubles as the CLI flag name.
type Feature struct {
	Key      string `json:"key"`
	Category string `json:"category,omitempty"`
	Desc     string `json:"desc,omitempty"`
	Owns     Owns   `json:"owns"`
}

// Owns enumerates the artifacts a feature is responsible for. Pruning an
// unselected feature removes exactly these. Paths are relative to the project
// root and use the ModulePlaceholder, because pruning runs on the raw cloned
// layout *before* placeholder replacement.
type Owns struct {
	// Dirs are whole directories removed when the feature is unselected.
	Dirs []string `json:"dirs,omitempty"`
	// Files are individual files removed when the feature is unselected
	// (used for per-protocol variants that live beside shared base files).
	Files []string `json:"files,omitempty"`
	// InitImports are blank-import lines to strip from internal/init.go when
	// the feature is unselected (server package and/or starter).
	InitImports []string `json:"init_imports,omitempty"`
}

// parse unmarshals and validates a manifest from raw JSON bytes. name labels
// the source in error messages.
func parse(b []byte, name string) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, errutil.Explain(err, "parse feature manifest %s", name)
	}
	seen := make(map[string]struct{}, len(m.Features))
	for _, f := range m.Features {
		if f.Key == "" {
			return nil, errutil.Explain(nil, "feature manifest %s has an entry with empty key", name)
		}
		if _, dup := seen[f.Key]; dup {
			return nil, errutil.Explain(nil, "feature manifest %s has duplicate key %q", name, f.Key)
		}
		seen[f.Key] = struct{}{}
	}
	return &m, nil
}

// Get returns the feature with the given key, or false if absent.
func (m *Manifest) Get(key string) (Feature, bool) {
	for _, f := range m.Features {
		if f.Key == key {
			return f, true
		}
	}
	return Feature{}, false
}

// embeddedManifest is the feature list compiled into the gs binary. It MUST be
// compiled in (not fetched from the cloned layout at runtime), because cobra
// registers flags before argv is parsed and the feature set defines those
// flags. Adding/removing a feature is therefore a gs release: edit this JSON
// and rebuild. Keep it in sync with the layout superset it prunes.
//
//go:embed features.json
var embeddedManifest []byte

var (
	embeddedOnce sync.Once
	embedded     *Manifest
	embeddedErr  error
)

// Embedded returns the manifest compiled into gs. It is the source of truth for
// which feature flags `gs init` (and `gs add`) expose.
func Embedded() (*Manifest, error) {
	embeddedOnce.Do(func() {
		embedded, embeddedErr = parse(embeddedManifest, "embedded features.json")
	})
	return embedded, embeddedErr
}
