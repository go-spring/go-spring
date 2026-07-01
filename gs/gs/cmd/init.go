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
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/stdlib/errutil"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const layoutRepoURL = "https://go-spring.org/stdlib/go-spring.git"

// NewInitCmd builds the `gs init` subcommand.
func NewInitCmd() *cobra.Command {
	var module string

	c := &cobra.Command{
		Use:          "init",
		Short:        "init go server project",
		Example:      "  gs init -m github.com/you/hello",
		SilenceUsage: true,
	}

	c.Flags().StringVarP(&module, "module", "m", "", `Go module path (required), e.g. "github.com/you/hello"`)

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
		return runInit(module)
	}

	return c
}

// runInit is the RunE handler for `gs init`. The caller must have already
// validated module via gomodule.CheckPath.
func runInit(module string) error {
	ss := strings.Split(module, "/")
	projectName := ss[len(ss)-1]

	// Convert project name to PascalCase for Go package naming, then verify
	// it is a legal Go identifier — otherwise the generated project won't build.
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

	if err := replaceFiles(srcDir, module, pkgName); err != nil {
		return err
	}

	if err := os.Rename(srcDir, projectName); err != nil {
		return errutil.Explain(err, "rename directory %q to %q", srcDir, projectName)
	}

	log.Println("[INFO] Generating project code")
	if err := genProject(projectName); err != nil {
		return errutil.Explain(err, "run gs gen")
	}
	return nil
}

// fetchLayout sparse-checks-out only the layout/ subdirectory of the go-spring
// repo at the latest layout/vX.Y.Z tag, and returns the local path along with
// a cleanup func that removes the surrounding temp directory. On error the
// cleanup func is a no-op — any temp scaffolding created has already been
// removed before returning.
func fetchLayout() (srcDir string, cleanup func(), err error) {
	cleanup = func() {}

	log.Println("[INFO] Resolving latest layout tag")
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

	log.Printf("[INFO] Fetching layout %s", tag)

	// Clone with blob filter and sparse mode — only root tree is checked out.
	cmd := exec.Command(
		"git", "clone",
		"--filter=blob:none",
		"--sparse",
		"--depth", "1",
		"--branch", tag,
		"--single-branch",
		layoutRepoURL,
	)
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return "", cleanup, errutil.Explain(err, "git clone")
	}

	// Pull only the layout/ directory into the working tree.
	repoDir := filepath.Join(tempDir, "go-spring")
	cmd = exec.Command("git", "sparse-checkout", "set", "layout")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return "", cleanup, errutil.Explain(err, "git sparse-checkout")
	}

	// Move layout/ out of the repo and discard the rest.
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
// layout/vX.Y.Z (or richer semver like layout/vX.Y.Z-rc.1) published on the
// go-spring remote. Pre-release tags are skipped so `gs init` sticks to
// stable releases by default.
func latestLayoutTag() (string, error) {
	out, err := exec.Command(
		"git", "ls-remote", "--tags", "--refs", layoutRepoURL, "layout/v*",
	).Output()
	if err != nil {
		return "", errutil.Explain(err, "list remote tags")
	}

	var bestTag, bestVer string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
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

// replaceFiles recursively replaces placeholders in file contents and file
// names under dir. Directory names are assumed not to carry placeholders.
func replaceFiles(dir string, module, pkgName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errutil.Explain(err, "read directory %q", dir)
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := replaceFiles(filepath.Join(dir, e.Name()), module, pkgName); err != nil {
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

		b = bytes.ReplaceAll(b, []byte("GS_PROJECT_MODULE"), []byte(module))
		b = bytes.ReplaceAll(b, []byte("GS_PROJECT_NAME"), []byte(pkgName))

		newName := strings.ReplaceAll(fileName, "GS_PROJECT_NAME", pkgName)
		if newName != fileName {
			if err = os.Remove(fileName); err != nil {
				return errutil.Explain(err, "remove file %q", fileName)
			}
		}

		// Preserve the layout file's original mode so executable scripts
		// stay executable and read-only files stay 0644.
		if err = os.WriteFile(newName, b, info.Mode().Perm()); err != nil {
			return errutil.Explain(err, "write file %q", newName)
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
