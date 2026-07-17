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
//   - The flag name IS the feature key IS the manifest key — a single
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
// # Per-feature parameters
//
// A feature flag is not a plain bool: bare (--gorm-mysql) selects the feature
// with default structure; with a value (--gorm-mysql="k=v;k2=v2") it also
// passes *structural* parameters that shape what is generated. Parameters never
// carry runtime config (addr, db, pool size) — that lives in the generated
// conf/. See ParseParams for the value grammar.
//
// # Flag registration: the manifest is compiled into gs
//
// cobra/pflag registers flags *before* parsing argv, and the feature set
// defines those flags — so the feature list cannot be discovered at runtime
// from the cloned layout. It is therefore compiled into the gs binary (see
// embed.go / features.json). gs code stays generic over this data: adding a
// feature is a JSON edit + gs rebuild, not a Go change. The tradeoff is that
// the feature set is a build-time property of gs and must be kept in sync with
// the layout superset it prunes.
//
// This package provides the layout-agnostic model, parser, and prune
// primitives; wiring them into the init command follows once the layout freezes.
package feature

import (
	"encoding/json"
	"sort"
	"strings"

	"go-spring.org/stdlib/errutil"
)

// Param value grammar delimiters. A feature flag value is a ';'-separated list
// of "key=value" pairs; a value may itself contain ',' (for list params) but
// never ';'.
const (
	pairSep = ";"
	kvSep   = "="
	listSep = ","
)

// Manifest is the parsed features.json at the layout root.
type Manifest struct {
	Features []Feature `json:"features"`
}

// Feature is one selectable vertical slice. Key doubles as the CLI flag name.
type Feature struct {
	Key      string           `json:"key"`
	Category string           `json:"category,omitempty"`
	Desc     string           `json:"desc,omitempty"`
	Owns     Owns             `json:"owns"`
	Params   map[string]Param `json:"params,omitempty"`
}

// Owns enumerates the artifacts a feature is responsible for. Pruning an
// unselected feature removes exactly these. Paths are relative to the project
// root and use the GS_PROJECT_MODULE placeholder, because pruning runs on the
// raw cloned layout *before* placeholder replacement.
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

// Param declares one structural parameter a feature accepts. It shapes code
// generation only, never runtime behavior.
type Param struct {
	Type    string   `json:"type"`              // "string" | "list"
	Default any      `json:"default,omitempty"` // string or []string per Type
	Desc    string   `json:"desc,omitempty"`
	Enum    []string `json:"enum,omitempty"` // if set, values must be members
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

// ParseParams parses a feature flag value against the feature's param spec.
//
// Grammar: pairs are separated by ';', each pair is "key=value", and a value
// may contain ',' for list params but never ';'. An empty raw string (bare
// flag) yields the spec defaults. Unknown keys and out-of-enum values are
// rejected; missing keys fall back to their declared default.
//
// The returned map holds raw string values (lists remain comma-joined); the
// generator that consumes them decides how to split. This keeps the parser
// layout-agnostic.
func ParseParams(raw string, spec map[string]Param) (map[string]string, error) {
	out := make(map[string]string, len(spec))
	// Seed with declared defaults so callers always get a complete map.
	for name, p := range spec {
		switch d := p.Default.(type) {
		case nil:
		case string:
			out[name] = d
		case []any:
			parts := make([]string, 0, len(d))
			for _, v := range d {
				parts = append(parts, toString(v))
			}
			out[name] = strings.Join(parts, listSep)
		default:
			out[name] = toString(d)
		}
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}

	for pair := range strings.SplitSeq(raw, pairSep) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, val, ok := strings.Cut(pair, kvSep)
		if !ok {
			return nil, errutil.Explain(nil, "malformed feature parameter %q: expected key%svalue", pair, kvSep)
		}
		key = strings.TrimSpace(key)
		p, known := spec[key]
		if !known {
			return nil, errutil.Explain(nil, "unknown parameter %q; accepted: %s", key, strings.Join(sortedKeys(spec), ", "))
		}
		if len(p.Enum) > 0 && !inEnum(val, p.Enum, p.Type) {
			return nil, errutil.Explain(nil, "parameter %q value %q not in allowed set: %s", key, val, strings.Join(p.Enum, ", "))
		}
		out[key] = val
	}
	return out, nil
}

// inEnum reports whether val is allowed by enum. For list params every
// comma-separated element must be a member.
func inEnum(val string, enum []string, typ string) bool {
	allowed := make(map[string]struct{}, len(enum))
	for _, e := range enum {
		allowed[e] = struct{}{}
	}
	check := []string{val}
	if typ == "list" {
		check = strings.Split(val, listSep)
	}
	for _, v := range check {
		if _, ok := allowed[strings.TrimSpace(v)]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(spec map[string]Param) []string {
	keys := make([]string, 0, len(spec))
	for k := range spec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
