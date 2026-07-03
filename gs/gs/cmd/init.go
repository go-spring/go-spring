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
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const layoutRepoURL = "https://github.com/go-spring/go-spring.git"

// supportedLayouts enumerates the layout styles `gs init` can scaffold. The
// map value is reserved for future per-layout metadata.
var supportedLayouts = map[string]struct{}{
	"mvc":    {},
	"domain": {},
}

// NewInitCmd builds the `gs init` subcommand.
func NewInitCmd() *cobra.Command {
	var module string
	var layout string

	c := &cobra.Command{
		Use:          "init",
		Short:        "init go server project",
		Example:      "  gs init -m github.com/you/hello",
		SilenceUsage: true,
	}

	c.Flags().StringVarP(&module, "module", "m", "", `Go module path (required), e.g. "github.com/you/hello"`)
	c.Flags().StringVar(&layout, "layout", "mvc", `Project layout style: "mvc" (default) or "domain"`)
	runcmd.BindFlag(c)

	c.RunE = func(cmd *cobra.Command, args []string) error {
		if module == "" {
			return errutil.Explain(nil, "module name is required")
		}
		if err := gomodule.CheckPath(module); err != nil {
			return errutil.Explain(err, "invalid module path %q", module)
		}
		if _, major, _ := gomodule.SplitPathVersion(module); major != "" {
			return errutil.Explain(nil, "module path %q has major version suffix %q; drop it when initializing a new project", module, major)
		}
		if _, ok := supportedLayouts[layout]; !ok {
			return errutil.Explain(nil, "unknown layout %q; supported: mvc, domain", layout)
		}
		return runInit(module, layout)
	}

	return c
}

// runInit is the RunE handler for `gs init`. The caller must have already
// validated module via gomodule.CheckPath and layout against supportedLayouts.
func runInit(module, layout string) error {
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

	srcDir, cleanup, err := fetchLayout()
	if err != nil {
		return err
	}
	defer cleanup()

	log.Println("[INFO] Selecting layout variant directories")
	renamed, err := stripLayoutSuffix(srcDir, layout)
	if err != nil {
		return err
	}

	// Placeholder substitutions plus the "<base>-<layout>" → "<base>" rewrites
	// mirroring the directory renames done above.
	replaces := map[string]string{
		"GS_PROJECT_MODULE": module,
		"GS_PROJECT_NAME":   pkgName,
	}
	for _, base := range renamed {
		replaces[base+"-"+layout] = base
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
// repo at the latest layout/vX.Y.Z tag, and returns the local path along with
// a cleanup func that removes the surrounding temp directory. On error the
// cleanup func is a no-op.
func fetchLayout() (srcDir string, cleanup func(), err error) {
	cleanup = func() {}

	tag, err := latestLayoutTag()
	if err != nil {
		return "", cleanup, err
	}

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
// layout/vX.Y.Z published on the go-spring remote. Pre-release tags are
// skipped.
func latestLayoutTag() (string, error) {
	cmd := exec.Command(
		"git", "ls-remote", "--tags", "--refs", layoutRepoURL, "layout/v*",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := runcmd.Run(cmd, "Resolving latest layout tag"); err != nil {
		return "", err
	}

	var bestTag, bestVer string
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		_, tag, ok := strings.Cut(line, "refs/tags/")
		if !ok {
			continue
		}
		ver, ok := strings.CutPrefix(tag, "layout/")
		if !ok || !semver.IsValid(ver) || semver.Prerelease(ver) != "" {
			continue
		}
		if bestVer == "" || semver.Compare(ver, bestVer) > 0 {
			bestTag, bestVer = tag, ver
		}
	}
	if bestTag == "" {
		return "", errutil.Explain(nil, "no layout/v* release tags found on remote")
	}
	return bestTag, nil
}

// matchLayoutVariant reports whether name ends with a "-<variant>" suffix for
// any supported layout, returning the stripped base and the matched variant.
func matchLayoutVariant(name string) (base, variant string, ok bool) {
	for v := range supportedLayouts {
		if b, matched := strings.CutSuffix(name, "-"+v); matched {
			return b, v, true
		}
	}
	return "", "", false
}

// stripLayoutSuffix walks dir and, for each directory named "<base>-<variant>"
// where <variant> is any supported layout, keeps only the entry whose variant
// matches the selected layout — renaming it to "<base>" — and removes the
// others. Non-variant directories are recursed into; variant directories are
// not (the layout convention doesn't nest variants). Returns the base names
// whose "<base>-<layout>" suffix was stripped so replaceFiles can rewrite
// matching path references in file contents.
func stripLayoutSuffix(dir, layout string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errutil.Explain(err, "read directory %q", dir)
	}
	var renamed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())

		base, variant, ok := matchLayoutVariant(e.Name())
		if !ok {
			sub, err := stripLayoutSuffix(child, layout)
			if err != nil {
				return nil, err
			}
			renamed = append(renamed, sub...)
			continue
		}
		if variant != layout {
			if err := os.RemoveAll(child); err != nil {
				return nil, errutil.Explain(err, "remove %q", child)
			}
			continue
		}
		newPath := filepath.Join(dir, base)
		if err := os.Rename(child, newPath); err != nil {
			return nil, errutil.Explain(err, "rename %q to %q", child, newPath)
		}
		renamed = append(renamed, base)
	}
	return renamed, nil
}

// replaceFiles recursively rewrites file contents under dir by applying every
// entry in replaces. File names are only rewritten for the GS_PROJECT_NAME
// placeholder; module paths contain "/" and would corrupt file names.
func replaceFiles(dir string, replaces map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errutil.Explain(err, "read directory %q", dir)
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := replaceFiles(filepath.Join(dir, e.Name()), replaces); err != nil {
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

		newFileName := fileName
		for old, new := range replaces {
			b = bytes.ReplaceAll(b, []byte(old), []byte(new))
			if old == "GS_PROJECT_NAME" {
				newFileName = strings.ReplaceAll(newFileName, old, new)
			}
		}
		if newFileName != fileName {
			if err = os.Remove(fileName); err != nil {
				return errutil.Explain(err, "remove file %q", fileName)
			}
		}

		// Preserve the layout file's original mode.
		if err = os.WriteFile(newFileName, b, info.Mode().Perm()); err != nil {
			return errutil.Explain(err, "write file %q", newFileName)
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
