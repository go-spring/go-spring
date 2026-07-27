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

// Package bom implements Go-Spring's BOM-style version governance: a single
// versions.yaml at the repo root records the "blessed" third-party dependency
// versions, and this package scans every go.mod under go.work to report where
// modules deviate from that baseline.
//
// The scan is read-only by default; only Apply writes, and only to one module
// at a time, so it never conflicts with concurrent work on other modules.
// Internal modules (the go-spring.org/... workspace members) are resolved via
// go.work and must never be pinned through require, so they are skipped
// everywhere.
package bom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"

	"go-spring.org/stdlib/errutil"
)

// InternalPrefix is the module-path prefix of the workspace's own modules.
// Requires on these are resolved by go.work, never pinned, so the baseline
// never governs them and the scanner ignores them.
const InternalPrefix = "go-spring.org/"

// Baseline is the parsed versions.yaml: the blessed Go version, the set of
// versions known to be broken, and the pinned third-party dependency versions.
type Baseline struct {
	Go           string              `yaml:"go"`
	Disabled     map[string][]string `yaml:"disabled"`
	Dependencies map[string]string   `yaml:"dependencies"`
}

// LoadBaseline reads and parses a versions.yaml file.
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errutil.Explain(err, "read baseline %s", path)
	}
	var b Baseline
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, errutil.Explain(err, "parse baseline %s", path)
	}
	return &b, nil
}

// Module is one workspace member discovered through go.work: its declared
// module path, the directory holding its go.mod (relative to the repo root),
// and its direct+indirect require entries.
type Module struct {
	Path     string // module path from the module directive
	Dir      string // directory relative to repo root, e.g. "starter/starter-actuator"
	Requires []Require
}

// Require is a single require entry from a module's go.mod.
type Require struct {
	Path     string
	Version  string
	Indirect bool
}

// FindRoot walks upward from start until it finds the directory containing
// go.work, and returns that directory. It is how the tool locates the repo
// root (and thus versions.yaml) from any working directory inside it.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", errutil.Explain(err, "resolve %s", start)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.work found from %s upward", start)
		}
		dir = parent
	}
}

// ScanWorkspace parses go.work under root and, for every use directive that is
// not an internal module, parses its go.mod and returns the module with its
// require entries. Directories whose go.mod is missing or whose module path is
// internal are skipped silently — internal members are governed by go.work,
// not by the baseline.
func ScanWorkspace(root string) ([]Module, error) {
	workPath := filepath.Join(root, "go.work")
	data, err := os.ReadFile(workPath)
	if err != nil {
		return nil, errutil.Explain(err, "read %s", workPath)
	}
	wf, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return nil, errutil.Explain(err, "parse %s", workPath)
	}

	var mods []Module
	for _, use := range wf.Use {
		rel := filepath.Clean(use.Path)
		modPath := filepath.Join(root, rel, "go.mod")
		m, err := parseModule(modPath)
		if err != nil {
			// A missing or malformed go.mod in the workspace is not fatal to a
			// governance scan; skip it so one broken member can't blind the
			// rest. (go.work can list dirs whose go.mod is transiently absent.)
			continue
		}
		m.Dir = filepath.ToSlash(rel)
		mods = append(mods, m)
	}
	return mods, nil
}

// parseModule reads and parses a single go.mod into a Module (without Dir set).
func parseModule(goModPath string) (Module, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return Module{}, err
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return Module{}, err
	}
	if f.Module == nil {
		return Module{}, fmt.Errorf("%s has no module directive", goModPath)
	}
	m := Module{Path: f.Module.Mod.Path}
	for _, r := range f.Require {
		m.Requires = append(m.Requires, Require{
			Path:     r.Mod.Path,
			Version:  r.Mod.Version,
			Indirect: r.Indirect,
		})
	}
	return m, nil
}

// DriftKind classifies how a module's require deviates from the baseline.
type DriftKind string

const (
	// DriftLower means the module pins an older version than the baseline.
	DriftLower DriftKind = "lower"
	// DriftHigher means the module pins a newer version than the baseline.
	DriftHigher DriftKind = "higher"
)

// Drift is one deviation of a module's require entry from the baseline.
type Drift struct {
	Module   string // module path
	Dir      string // directory relative to repo root
	Dep      string // dependency module path
	Found    string // version the module requires
	Baseline string // blessed version from versions.yaml
	Indirect bool   // whether the require is // indirect
	Kind     DriftKind
}

// Check scans every governed module and returns the drifts against the
// baseline, i.e. require entries whose version differs from a blessed version.
// Only dependencies listed in the baseline are considered; a dependency a
// module doesn't use, and requires on internal modules, produce no drift.
func Check(root string, base *Baseline) ([]Drift, error) {
	mods, err := ScanWorkspace(root)
	if err != nil {
		return nil, err
	}

	var drifts []Drift
	for _, m := range mods {
		for _, r := range m.Requires {
			if strings.HasPrefix(r.Path, InternalPrefix) {
				continue
			}
			want, ok := base.Dependencies[r.Path]
			if !ok {
				continue
			}
			kind, deviates := classify(r.Version, want)
			if !deviates {
				continue
			}
			drifts = append(drifts, Drift{
				Module:   m.Path,
				Dir:      m.Dir,
				Dep:      r.Path,
				Found:    r.Version,
				Baseline: want,
				Indirect: r.Indirect,
				Kind:     kind,
			})
		}
	}
	return drifts, nil
}

// classify compares a found version against the blessed one and reports the
// drift kind, plus whether they differ at all. Non-semver versions (pseudo or
// malformed) that don't equal the baseline are reported as lower, since an
// uncomparable pin is safest treated as "needs review, below baseline".
func classify(found, want string) (DriftKind, bool) {
	if found == want {
		return "", false
	}
	if !semver.IsValid(found) || !semver.IsValid(want) {
		return DriftLower, true
	}
	switch semver.Compare(found, want) {
	case 0:
		return "", false
	case -1:
		return DriftLower, true
	default:
		return DriftHigher, true
	}
}

// Apply aligns a single module's go.mod require versions to the baseline and
// writes the file back. It targets one module (by module path or by its
// workspace directory) so serial remediation never touches more than one
// go.mod at a time. It returns the list of changes made.
func Apply(root string, base *Baseline, target string) ([]Drift, error) {
	mods, err := ScanWorkspace(root)
	if err != nil {
		return nil, err
	}
	var found *Module
	for i := range mods {
		if mods[i].Path == target || mods[i].Dir == filepath.ToSlash(filepath.Clean(target)) {
			found = &mods[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("module %q not found in workspace (use its module path or workspace dir)", target)
	}

	goModPath := filepath.Join(root, filepath.FromSlash(found.Dir), "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, errutil.Explain(err, "read %s", goModPath)
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, errutil.Explain(err, "parse %s", goModPath)
	}

	var changes []Drift
	for _, r := range f.Require {
		if strings.HasPrefix(r.Mod.Path, InternalPrefix) {
			continue
		}
		want, ok := base.Dependencies[r.Mod.Path]
		if !ok {
			continue
		}
		if _, deviates := classify(r.Mod.Version, want); !deviates {
			continue
		}
		changes = append(changes, Drift{
			Module: found.Path, Dir: found.Dir, Dep: r.Mod.Path,
			Found: r.Mod.Version, Baseline: want, Indirect: r.Indirect,
		})
		if err := f.AddRequire(r.Mod.Path, want); err != nil {
			return nil, errutil.Explain(err, "set %s@%s", r.Mod.Path, want)
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}

	f.Cleanup()
	out, err := f.Format()
	if err != nil {
		return nil, errutil.Explain(err, "format %s", goModPath)
	}
	if err := os.WriteFile(goModPath, out, 0o644); err != nil {
		return nil, errutil.Explain(err, "write %s", goModPath)
	}
	return changes, nil
}
