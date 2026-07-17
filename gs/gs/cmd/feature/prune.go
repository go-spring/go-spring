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

package feature

import (
	"os"
	"path/filepath"
	"strings"

	"go-spring.org/stdlib/errutil"
)

// initImportsFile is the file whose blank-import block a pruned feature's
// InitImports lines are stripped from.
const initImportsFile = "internal/init.go"

// Prune deletes the artifacts of every manifest feature whose key is NOT in
// selected, turning the full-superset layout into the user's chosen subset.
//
// It must run on the raw cloned layout *before* placeholder replacement,
// because Owns paths and InitImports use the GS_PROJECT_MODULE placeholder.
//
// This is the layout-agnostic core of the pruning pattern. The exact set of
// features and their Owns entries is data (features.json), so this code needs
// no change as the layout evolves — only the manifest does.
func Prune(projectDir string, m *Manifest, selected map[string]struct{}) error {
	var dropImports []string
	for _, f := range m.Features {
		if _, keep := selected[f.Key]; keep {
			continue
		}
		for _, d := range f.Owns.Dirs {
			p := filepath.Join(projectDir, filepath.FromSlash(d))
			if err := os.RemoveAll(p); err != nil {
				return errutil.Explain(err, "prune feature %q: remove dir %q", f.Key, p)
			}
		}
		for _, file := range f.Owns.Files {
			p := filepath.Join(projectDir, filepath.FromSlash(file))
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return errutil.Explain(err, "prune feature %q: remove file %q", f.Key, p)
			}
		}
		dropImports = append(dropImports, f.Owns.InitImports...)
	}
	if len(dropImports) > 0 {
		if err := stripInitImports(filepath.Join(projectDir, filepath.FromSlash(initImportsFile)), dropImports); err != nil {
			return err
		}
	}
	return nil
}

// stripInitImports removes any line in file that references one of the given
// import paths. Kept intentionally simple (substring match on the import path)
// because internal/init.go's import block format is not yet frozen; tighten to
// an AST rewrite once the layout stabilizes.
func stripInitImports(file string, imports []string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errutil.Explain(err, "read %q", file)
	}
	lines := strings.Split(string(b), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if importedAny(line, imports) {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(file, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		return errutil.Explain(err, "write %q", file)
	}
	return nil
}

// importedAny reports whether line is a blank-import statement of one of the
// given import paths.
func importedAny(line string, imports []string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "_ ") {
		return false
	}
	for _, imp := range imports {
		if strings.Contains(trimmed, `"`+imp+`"`) {
			return true
		}
	}
	return false
}
