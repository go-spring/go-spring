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

// Package k8s holds the Kubernetes deploy scaffolding templates compiled into
// the gs binary and renders them into a project. The templates are embedded
// (not fetched from the layout repo) so `gs k8s` is self-contained and works
// offline, mirroring how the feature manifest is embedded.
package k8s

import (
	"bytes"
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"

	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// templates carries every scaffolding file. The `all:` prefix is required so
// dotfiles like .dockerignore are embedded too (the default pattern skips
// names beginning with "." or "_").
//
//go:embed all:templates
var templates embed.FS

// Write renders every embedded template into destDir, applying the given
// placeholder replacements to file contents. Placeholders are applied
// longest-key-first so a shorter key never partially overwrites a longer one
// that contains it as a prefix. Existing files are skipped (with an [INFO]
// line) unless force is true, so re-running `gs k8s` never silently clobbers
// hand-edited manifests.
func Write(destDir string, replaces map[string]string, force bool) error {
	keys := make([]string, 0, len(replaces))
	for k := range replaces {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	const root = "templates"
	return fs.WalkDir(templates, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return errutil.Explain(err, "resolve template path %q", path)
		}
		outPath := filepath.Join(destDir, rel)

		if !force {
			if _, statErr := os.Stat(outPath); statErr == nil {
				log.Printf("[INFO] skip existing %s (use --force to overwrite)", rel)
				return nil
			}
		}

		b, err := templates.ReadFile(path)
		if err != nil {
			return errutil.Explain(err, "read template %q", path)
		}
		for _, old := range keys {
			b = bytes.ReplaceAll(b, []byte(old), []byte(replaces[old]))
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return errutil.Explain(err, "create directory for %q", outPath)
		}
		if err := os.WriteFile(outPath, b, 0o644); err != nil {
			return errutil.Explain(err, "write %q", outPath)
		}

		log.Printf("[INFO] writing %s", rel)
		if runcmd.Verbosity >= runcmd.LevelCommand {
			log.Printf("[DEBUG] -> %s", outPath)
		}
		return nil
	})
}
