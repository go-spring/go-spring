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
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-spring.org/stdlib/errutil"
)

// modulePlaceholder is the token every layout file uses in place of the real
// Go module path. Copy resolves it from replaces so copied artifacts and the
// import lines inserted into internal/init.go carry the project's real path.
const modulePlaceholder = "GS_PROJECT_MODULE"

// Copy adds the artifacts of every selected feature into an existing project,
// the inverse of Prune. For `gs add`: the layout superset is fetched at the
// project's pinned version into layoutDir, and each selected feature's Owns
// dirs/files are copied from layoutDir into projectDir with placeholders
// (replaces) applied, then its InitImports are inserted into internal/init.go.
//
// Copy refuses to overwrite: if a feature's owned dir or file already exists in
// projectDir the feature is considered already added and an error is returned,
// so `gs add grpc` run twice fails cleanly rather than clobbering local edits.
//
// Like Prune this stays layout-agnostic: what a feature owns is data
// (features.json), so this code needs no change as the layout evolves.
func Copy(projectDir, layoutDir string, m *Manifest, selected map[string]struct{}, replaces map[string]string) error {
	keys := replaceKeys(replaces)
	var addImports []string
	for _, f := range m.Features {
		if _, want := selected[f.Key]; !want {
			continue
		}
		for _, d := range f.Owns.Dirs {
			src := filepath.Join(layoutDir, filepath.FromSlash(d))
			dst := filepath.Join(projectDir, filepath.FromSlash(d))
			if _, err := os.Stat(dst); err == nil {
				return errutil.Explain(nil, "feature %q already added: %q exists", f.Key, dst)
			}
			if err := copyTree(src, dst, replaces, keys); err != nil {
				return errutil.Explain(err, "copy feature %q dir %q", f.Key, d)
			}
		}
		for _, file := range f.Owns.Files {
			src := filepath.Join(layoutDir, filepath.FromSlash(file))
			dst := filepath.Join(projectDir, filepath.FromSlash(file))
			if _, err := os.Stat(dst); err == nil {
				return errutil.Explain(nil, "feature %q already added: %q exists", f.Key, dst)
			}
			if err := copyFile(src, dst, replaces, keys); err != nil {
				return errutil.Explain(err, "copy feature %q file %q", f.Key, file)
			}
		}
		for _, imp := range f.Owns.InitImports {
			addImports = append(addImports, applyReplaces(imp, replaces, keys))
		}
	}
	if len(addImports) > 0 {
		modulePrefix := replaces[modulePlaceholder]
		if err := insertInitImports(filepath.Join(projectDir, filepath.FromSlash(initImportsFile)), addImports, modulePrefix); err != nil {
			return err
		}
	}
	return nil
}

// copyTree recursively copies the directory at src into dst, applying replaces
// to every file's contents and preserving file modes. Intermediate dirs are
// created as needed.
func copyTree(src, dst string, replaces map[string]string, keys []string) error {
	info, err := os.Stat(src)
	if err != nil {
		return errutil.Explain(err, "stat %q", src)
	}
	if !info.IsDir() {
		return copyFile(src, dst, replaces, keys)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return errutil.Explain(err, "create dir %q", dst)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return errutil.Explain(err, "read dir %q", src)
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), replaces, keys); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single regular file from src to dst, applying replaces to
// its contents and preserving its mode. The parent directory of dst is created
// if missing.
func copyFile(src, dst string, replaces map[string]string, keys []string) error {
	info, err := os.Stat(src)
	if err != nil {
		return errutil.Explain(err, "stat %q", src)
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return errutil.Explain(err, "read %q", src)
	}
	for _, k := range keys {
		b = bytes.ReplaceAll(b, []byte(k), []byte(replaces[k]))
	}
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return errutil.Explain(err, "create dir for %q", dst)
	}
	if err := os.WriteFile(dst, b, info.Mode().Perm()); err != nil {
		return errutil.Explain(err, "write %q", dst)
	}
	return nil
}

// insertInitImports adds blank-import lines for the given paths into file's
// import block, then rewrites the block canonically: internal imports (those
// prefixed with modulePrefix) in the first group, third-party imports in the
// second, each group sorted, separated by a blank line. Paths already present
// are not duplicated. It is the inverse of stripInitImports and, like it,
// stays line-based (not an AST rewrite) until the layout's init.go format
// freezes.
func insertInitImports(file string, imports []string, modulePrefix string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return errutil.Explain(err, "read %q", file)
	}
	lines := strings.Split(string(b), "\n")

	openIdx, closeIdx := -1, -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "import (" {
			openIdx = i
		} else if openIdx >= 0 && strings.TrimSpace(line) == ")" {
			closeIdx = i
			break
		}
	}
	if openIdx < 0 || closeIdx < 0 {
		return errutil.Explain(nil, "no import block found in %q", file)
	}

	// Gather existing blank-import paths from the block.
	paths := map[string]struct{}{}
	var order []string
	add := func(p string) {
		if _, dup := paths[p]; dup {
			return
		}
		paths[p] = struct{}{}
		order = append(order, p)
	}
	for _, line := range lines[openIdx+1 : closeIdx] {
		if p, ok := blankImportPath(line); ok {
			add(p)
		}
	}
	for _, p := range imports {
		add(p)
	}

	// Partition into internal (module-prefixed) and third-party groups.
	var internal, external []string
	for _, p := range order {
		if modulePrefix != "" && (p == modulePrefix || strings.HasPrefix(p, modulePrefix+"/")) {
			internal = append(internal, p)
		} else {
			external = append(external, p)
		}
	}
	sort.Strings(internal)
	sort.Strings(external)

	var block []string
	block = append(block, lines[openIdx]) // "import ("
	for _, p := range internal {
		block = append(block, "\t_ \""+p+"\"")
	}
	if len(internal) > 0 && len(external) > 0 {
		block = append(block, "")
	}
	for _, p := range external {
		block = append(block, "\t_ \""+p+"\"")
	}
	block = append(block, lines[closeIdx]) // ")"

	out := append([]string{}, lines[:openIdx]...)
	out = append(out, block...)
	out = append(out, lines[closeIdx+1:]...)
	if err := os.WriteFile(file, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return errutil.Explain(err, "write %q", file)
	}
	return nil
}

// blankImportPath extracts the import path from a blank-import line
// (`_ "path"`), returning ok=false for any other line.
func blankImportPath(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "_ ") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "_ "))
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", false
	}
	return rest[1 : len(rest)-1], true
}

// replaceKeys returns the replaces keys sorted longest-first, so a shorter key
// never partially overwrites a longer key that contains it as a prefix (mirrors
// cmd.replaceFiles).
func replaceKeys(replaces map[string]string) []string {
	keys := make([]string, 0, len(replaces))
	for k := range replaces {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

// applyReplaces applies replaces (in the given longest-first key order) to s.
func applyReplaces(s string, replaces map[string]string, keys []string) string {
	for _, k := range keys {
		s = strings.ReplaceAll(s, k, replaces[k])
	}
	return s
}
