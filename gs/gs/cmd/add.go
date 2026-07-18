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

package cmd

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/gs/cmd/feature"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// projectMeta mirrors the gs.json written at `gs init`. It records how the
// project was scaffolded so `gs add` can reproduce the same substitutions and
// pin the same layout version when copying in new feature slices.
type projectMeta struct {
	Module        string `json:"module"`
	Lang          string `json:"lang"`
	LayoutVersion string `json:"layout_version"`
}

// NewAddCmd builds the `gs add` subcommand.
func NewAddCmd() *cobra.Command {
	var list bool

	c := &cobra.Command{
		Use:          "add [feature...]",
		Short:        "add feature slices (server + idl + controllers) to a project",
		Example:      "  gs add grpc trpc",
		SilenceUsage: true,
	}
	c.Flags().BoolVar(&list, "list-features", false, "list selectable features and exit")
	runcmd.BindFlag(c)

	m, manifestErr := feature.Embedded()

	c.RunE = func(cmd *cobra.Command, args []string) error {
		if manifestErr != nil {
			return manifestErr
		}
		if list {
			cmd.Print(formatFeatures(m))
			return nil
		}
		if len(args) == 0 {
			return cmd.Help()
		}
		return runAdd(m, args)
	}
	return c
}

// runAdd copies the requested feature slices into the project rooted at the
// current working directory. It reads gs.json for the module path, docs
// language, and pinned layout version, then fetches that exact layout tag and
// copies each selected feature's artifacts in.
func runAdd(m *feature.Manifest, keys []string) error {
	currDir, err := os.Getwd()
	if err != nil {
		return errutil.Explain(err, "get working directory")
	}

	meta, err := readProjectMeta(currDir)
	if err != nil {
		return err
	}

	selected := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if _, ok := m.Get(k); !ok {
			return errutil.Explain(nil, "unknown feature %q; run `gs add --list-features` to list", k)
		}
		selected[k] = struct{}{}
	}

	tag := "layout/" + meta.LayoutVersion
	srcDir, cleanup, err := fetchLayoutAt(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	log.Println("[INFO] Selecting documentation language")
	if err := stripLangSuffix(srcDir, meta.Lang); err != nil {
		return err
	}

	pkgName := toPascal(moduleLeaf(meta.Module))
	replaces := map[string]string{
		"GS_PROJECT_MODULE": meta.Module,
		"GS_PROJECT_NAME":   pkgName,
		"GS_PROJECT_LANG":   meta.Lang,
		"GS_LAYOUT_VERSION": meta.LayoutVersion,
	}

	log.Printf("[INFO] Copying features: %s", strings.Join(keys, ", "))
	if err := feature.Copy(currDir, srcDir, m, selected, replaces); err != nil {
		return err
	}

	log.Println("[INFO] Done. Run `gs go mod tidy` to resolve any new dependencies.")
	return nil
}

// readProjectMeta loads gs.json from dir. A missing gs.json means dir is not a
// project root created by `gs init`.
func readProjectMeta(dir string) (projectMeta, error) {
	var meta projectMeta
	b, err := os.ReadFile(filepath.Join(dir, "gs.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return meta, errutil.Explain(err, "gs.json not found: run `gs add` from a project root")
		}
		return meta, errutil.Explain(err, "read gs.json")
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, errutil.Explain(err, "parse gs.json")
	}
	if meta.Module == "" {
		return meta, errutil.Explain(nil, "gs.json is missing the module path")
	}
	if meta.LayoutVersion == "" {
		return meta, errutil.Explain(nil, "gs.json is missing layout_version; cannot pin the layout to fetch")
	}
	if meta.Lang == "" {
		meta.Lang = "zh"
	}
	return meta, nil
}

// moduleLeaf returns the final path segment of a Go module path, used to derive
// the project's Go package name.
func moduleLeaf(module string) string {
	ss := strings.Split(module, "/")
	return ss[len(ss)-1]
}
