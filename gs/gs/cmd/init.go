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
	"bytes"
	"fmt"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/gs/cmd/feature"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const layoutRepoURL = "https://github.com/go-spring/go-spring.git"

// supportedLangs enumerates the documentation languages `gs init` can pick
// when a layout ships files in "<stem>.<lang><ext>" variants. The map value
// is reserved for future per-lang metadata.
var supportedLangs = map[string]struct{}{
	"zh": {},
	"en": {},
}

// NewInitCmd builds the `gs init` subcommand.
func NewInitCmd() *cobra.Command {
	var module string
	var lang string
	var list bool

	c := &cobra.Command{
		Use:          "init",
		Short:        "init go server project",
		Example:      "  gs init -m github.com/you/hello --grpc --http",
		SilenceUsage: true,
	}

	c.Flags().StringVarP(&module, "module", "m", "", `Go module path (required), e.g. "github.com/you/hello"`)
	c.Flags().StringVar(&lang, "lang", "zh", `Documentation language: "zh" (default) or "en"`)
	c.Flags().BoolVar(&list, "list-features", false, "list selectable features and exit")
	runcmd.BindFlag(c)

	// The feature manifest is compiled into gs, so its flags must be registered
	// now, before argv is parsed. A parse error is a build defect; surface it in
	// RunE rather than panic at construction.
	m, manifestErr := feature.Embedded()
	var selected func() map[string]struct{}
	if manifestErr == nil {
		selected = bindFeatureFlags(c, m)
	}

	c.RunE = func(cmd *cobra.Command, args []string) error {
		if manifestErr != nil {
			return manifestErr
		}
		if list {
			fmt.Print(formatFeatures(m))
			return nil
		}
		if module == "" {
			return errutil.Explain(nil, "module name is required")
		}
		if err := gomodule.CheckPath(module); err != nil {
			return errutil.Explain(err, "invalid module path %q", module)
		}
		if _, major, _ := gomodule.SplitPathVersion(module); major != "" {
			return errutil.Explain(nil, "module path %q has major version suffix %q; drop it when initializing a new project", module, major)
		}
		if _, ok := supportedLangs[lang]; !ok {
			return errutil.Explain(nil, "unknown lang %q; supported: zh, en", lang)
		}
		return runInit(module, lang, m, selected())
	}

	return c
}

// runInit is the RunE handler for `gs init`. The caller must have already
// validated module via gomodule.CheckPath and lang against supportedLangs.
// selected names the features to keep; every other manifest feature is pruned
// from the cloned superset.
func runInit(module, lang string, m *feature.Manifest, selected map[string]struct{}) error {
	ss := strings.Split(module, "/")
	projectName := ss[len(ss)-1]

	// Convert project name to PascalCase for Go package naming and verify
	// it is a legal Go identifier.
	pkgName := toPascal(projectName)
	if !token.IsIdentifier(pkgName) || token.Lookup(pkgName).IsKeyword() {
		return errutil.Explain(nil, "cannot derive a Go package name from %q", projectName)
	}

	if _, err := os.Stat(projectName); err == nil {
		return errutil.Explain(nil, "directory %q already exists", projectName)
	} else if !os.IsNotExist(err) {
		return errutil.Explain(err, "stat directory %q", projectName)
	}

	srcDir, layoutVersion, cleanup, err := fetchLayout()
	if err != nil {
		return err
	}
	defer cleanup()

	log.Println("[INFO] Selecting documentation language")
	if err := stripLangSuffix(srcDir, lang); err != nil {
		return err
	}

	// Prune must run on the raw layout, before placeholder replacement: a
	// feature's Owns paths and init imports use the GS_PROJECT_MODULE token.
	log.Println("[INFO] Pruning unselected features")
	if err := feature.Prune(srcDir, m, selected); err != nil {
		return err
	}

	replaces := map[string]string{
		"GS_PROJECT_MODULE": module,
		"GS_PROJECT_NAME":   pkgName,
		"GS_PROJECT_LANG":   lang,
		"GS_LAYOUT_VERSION": layoutVersion,
	}

	log.Println("[INFO] Rewriting module path and package names")
	if err := replaceFiles(srcDir, replaces); err != nil {
		return err
	}

	log.Printf("[INFO] Placing project at ./%s", projectName)
	if err := os.Rename(srcDir, projectName); err != nil {
		return errutil.Explain(err, "rename directory %q to %q", srcDir, projectName)
	}

	// genProject's child processes cd into subdirs of the project, so pass an
	// absolute path.
	projectDir, err := filepath.Abs(projectName)
	if err != nil {
		return errutil.Explain(err, "resolve project dir %q", projectName)
	}

	log.Println("[INFO] Generating project code")
	if err := genProject(projectDir); err != nil {
		return errutil.Explain(err, "run gs gen")
	}
	return nil
}

// fetchLayout sparse-checks-out only the layout/ subdirectory of the go-spring
// repo at the latest layout/vX.Y.Z tag, and returns the local path, the
// resolved version (e.g. "v1.2.3"), and a cleanup func that removes the
// surrounding temp directory. On error the cleanup func is a no-op.
func fetchLayout() (srcDir, version string, cleanup func(), err error) {
	tag, version, err := latestLayoutTag()
	if err != nil {
		return "", "", func() {}, err
	}
	srcDir, cleanup, err = fetchLayoutAt(tag)
	return srcDir, version, cleanup, err
}

// fetchLayoutAt sparse-checks-out the layout/ subdirectory at the given tag
// (e.g. "layout/v1.2.3") and returns its local path plus a cleanup func that
// removes the surrounding temp directory. `gs add` uses this with the version
// pinned in the project's gs.json so newly copied slices match the layout the
// project was created from; on error the cleanup func is a no-op.
func fetchLayoutAt(tag string) (srcDir string, cleanup func(), err error) {
	cleanup = func() {}

	tempDir, err := os.MkdirTemp("", "gs-layout-")
	if err != nil {
		return "", cleanup, errutil.Explain(err, "create temp directory")
	}
	cleanup = func() { _ = os.RemoveAll(tempDir) }
	defer func() {
		if err != nil {
			cleanup()
			cleanup = func() {}
		}
	}()

	// Clone with blob filter and sparse mode — only root tree is checked out.
	args := []string{
		"git",
		"-c", "advice.detachedHead=false",
		"clone",
		"--filter=blob:none",
		"--sparse",
		"--depth", "1",
		"--branch", tag,
		"--single-branch",
	}
	if !runcmd.Streaming() {
		args = append(args, "--quiet")
	}
	args = append(args, layoutRepoURL)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = tempDir
	if err = runcmd.Run(cmd, fmt.Sprintf("Cloning go-spring at %s", tag)); err != nil {
		return "", cleanup, err
	}

	// Pull only the layout/ directory into the working tree.
	repoDir := filepath.Join(tempDir, "go-spring")
	cmd = exec.Command("git", "sparse-checkout", "set", "layout")
	cmd.Dir = repoDir
	if err = runcmd.Run(cmd, "Extracting layout directory"); err != nil {
		return "", cleanup, err
	}

	// Move layout/ out of the repo and discard the rest.
	log.Println("[INFO] Cleaning up temporary git metadata")
	projectDir := filepath.Join(tempDir, "layout")
	if err = os.Rename(filepath.Join(repoDir, "layout"), projectDir); err != nil {
		return "", cleanup, errutil.Explain(err, "move layout directory")
	}
	if err = os.RemoveAll(repoDir); err != nil {
		return "", cleanup, errutil.Explain(err, "remove repo directory")
	}
	return projectDir, cleanup, nil
}

// latestLayoutTag returns the highest-semver release tag of the form
// layout/vX.Y.Z published on the go-spring remote, along with the stripped
// semver ("vX.Y.Z"). Pre-release tags are skipped.
func latestLayoutTag() (tag, version string, err error) {
	cmd := exec.Command(
		"git", "ls-remote", "--tags", "--refs", layoutRepoURL, "layout/v*",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := runcmd.Run(cmd, "Resolving latest layout tag"); err != nil {
		return "", "", err
	}

	var bestTag, bestVer string
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		_, t, ok := strings.Cut(line, "refs/tags/")
		if !ok {
			continue
		}
		ver, ok := strings.CutPrefix(t, "layout/")
		if !ok || !semver.IsValid(ver) || semver.Prerelease(ver) != "" {
			continue
		}
		if bestVer == "" || semver.Compare(ver, bestVer) > 0 {
			bestTag, bestVer = t, ver
		}
	}
	if bestTag == "" {
		return "", "", errutil.Explain(nil, "no layout/v* release tags found on remote")
	}
	return bestTag, bestVer, nil
}

// matchLangVariant reports whether name has a "<stem>.<lang><ext>" pattern for
// any supported lang, returning the "<stem><ext>" base and the matched lang.
// Files without an outer extension (e.g. "AGENTS.zh") are not treated as lang
// variants — the pattern requires a trailing extension after the lang tag.
func matchLangVariant(name string) (base, variant string, ok bool) {
	ext := filepath.Ext(name)
	if ext == "" {
		return "", "", false
	}
	stem := strings.TrimSuffix(name, ext)
	inner := filepath.Ext(stem)
	if inner == "" {
		return "", "", false
	}
	v := strings.TrimPrefix(inner, ".")
	if _, supported := supportedLangs[v]; !supported {
		return "", "", false
	}
	return strings.TrimSuffix(stem, inner) + ext, v, true
}

// stripLangSuffix walks dir and, for each regular file (or symlink) named
// "<stem>.<variant><ext>" where <variant> is any supported lang, keeps only
// the entry whose variant matches the selected lang — renaming it to
// "<stem><ext>" — and removes the others. Directories are recursed into.
// Symlinks are renamed via os.Rename (which does not follow), so the link
// itself is renamed; the target text is left untouched. Layout scaffolds
// should point cross-language symlinks at the post-strip name to avoid
// dangling references.
func stripLangSuffix(dir, lang string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errutil.Explain(err, "read directory %q", dir)
	}
	for _, e := range entries {
		child := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := stripLangSuffix(child, lang); err != nil {
				return err
			}
			continue
		}
		base, variant, ok := matchLangVariant(e.Name())
		if !ok {
			continue
		}
		if variant != lang {
			if err := os.Remove(child); err != nil {
				return errutil.Explain(err, "remove %q", child)
			}
			continue
		}
		newPath := filepath.Join(dir, base)
		if err := os.Rename(child, newPath); err != nil {
			return errutil.Explain(err, "rename %q to %q", child, newPath)
		}
	}
	return nil
}

// replaceFiles recursively rewrites file contents under dir by applying every
// entry in replaces. Placeholders are applied longest-first so a shorter key
// never partially overwrites a longer key that contains it as a prefix. File
// names are not rewritten: layout files whose names carry a placeholder live
// under paths that `gs gen` wipes and regenerates.
func replaceFiles(dir string, replaces map[string]string) error {
	keys := make([]string, 0, len(replaces))
	for k := range replaces {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return replaceFilesWithOrder(dir, replaces, keys)
}

func replaceFilesWithOrder(dir string, replaces map[string]string, keys []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errutil.Explain(err, "read directory %q", dir)
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := replaceFilesWithOrder(filepath.Join(dir, e.Name()), replaces, keys); err != nil {
				return err
			}
			continue
		}

		fileName := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			return errutil.Explain(err, "stat file %q", fileName)
		}
		b, err := os.ReadFile(fileName)
		if err != nil {
			return errutil.Explain(err, "read file %q", fileName)
		}

		for _, old := range keys {
			b = bytes.ReplaceAll(b, []byte(old), []byte(replaces[old]))
		}

		// Preserve the layout file's original mode.
		if err = os.WriteFile(fileName, b, info.Mode().Perm()); err != nil {
			return errutil.Explain(err, "write file %q", fileName)
		}
	}
	return nil
}

// toPascal converts a name in snake_case or kebab-case (or a mix) to
// PascalCase, so it can be used as a Go package name. Input is assumed ASCII
// (enforced upstream by gomodule.CheckPath).
func toPascal(s string) string {
	var sb strings.Builder
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	}) {
		c := part[0]
		if 'a' <= c && c <= 'z' {
			c = c - 'a' + 'A'
		}
		sb.WriteByte(c)
		if len(part) > 1 {
			sb.WriteString(part[1:])
		}
	}
	return sb.String()
}
