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

// Package claude wraps the local `claude` CLI launch: PATH availability, a
// gs-skill freshness check that offers an in-place upgrade, and foreground
// exec. The launch confirmation prompt is the caller's responsibility.
package claude

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// The gs skill is versioned as a normal subproject of the go-spring monorepo:
// its releases are tagged `skills/gs/vX.Y.Z`, and the highest such tag on the
// remote is authoritative for "latest".
const (
	skillRemoteURL = "https://github.com/go-spring/go-spring.git"
	skillTagPrefix = "skills/gs/"
)

// Available reports whether the local `claude` CLI is on PATH. Callers use
// this as an early gate so the user isn't prompted for a launch that would
// fail on exec.
func Available() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return errutil.Explain(err, "claude not found in PATH")
	}
	return nil
}

// Run launches the local `claude` CLI attached to the current terminal. It
// stays in the foreground so claude owns the TTY exactly like `bash -c claude`
// would. Callers are expected to have called Available and obtained user
// consent already; this function only runs skill-freshness checks before exec.
//
// Verbosity (runcmd.Verbosity) follows the shared -v ladder: level 0 logs the
// step line, -v adds the argv, -vv adds the child exit code. The child's
// stdout/stderr are always live since claude owns the TTY.
func Run() {
	if err := checkSkillVersion(); err != nil {
		fmt.Fprintf(os.Stderr, "skill check failed: %s\n", err)
		os.Exit(1)
	}

	log.Printf("[INFO] Launching claude")
	c := exec.Command("claude")
	if runcmd.Verbosity >= runcmd.LevelCommand {
		log.Printf("[DEBUG] %s", strings.Join(c.Args, " "))
	}
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if runcmd.Verbosity >= runcmd.LevelStream {
		exitCode := 0
		if c.ProcessState != nil {
			exitCode = c.ProcessState.ExitCode()
		}
		log.Printf("[DEBUG] exit=%d", exitCode)
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "claude exited with error: %s\n", err)
		os.Exit(1)
	}
}

// skillDir is the on-disk location of the installed gs skill. Honors the
// same CLAUDE_SKILLS_DIR override that install.sh reads, so the freshness
// check and the installer stay in agreement.
func skillDir() string {
	if d := os.Getenv("CLAUDE_SKILLS_DIR"); d != "" {
		return d + "/gs"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude/skills/gs"
	}
	return home + "/.claude/skills/gs"
}

// checkSkillVersion checks that the installed gs skill exists and, if it is
// behind the latest release tagged on the remote, prompts the user to upgrade
// in place. Existence is checked first so the "not installed" hint is
// unambiguous; the remote fetch only runs when we have something to compare
// against. A network failure, a declined prompt, or a failed upgrade all
// degrade to a warning so the user can still launch claude.
func checkSkillVersion() error {
	log.Printf("[INFO] Checking gs skill version")
	path := skillDir() + "/SKILL.md"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errutil.Explain(err, "gs skill not installed at %s; run skills/gs/install.sh from the go-spring repo", skillDir())
		}
		return errutil.Explain(err, "read %s", path)
	}
	installed := parseFrontmatterVersion(data)
	if installed == "" {
		return errutil.Explain(nil, "gs skill at %s has no version field; run skills/gs/install.sh from the go-spring repo to refresh", skillDir())
	}
	if !semver.IsValid(installed) {
		return errutil.Explain(nil, "gs skill at %s has invalid version %q", skillDir(), installed)
	}
	latest, err := fetchLatestSkillVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping skill version check: %s\n", err)
		return nil
	}
	if semver.Compare(installed, latest) < 0 {
		promptUpgrade(installed, latest)
	}
	return nil
}

// promptUpgrade tells the user the skill is behind and offers to upgrade it in
// place, mirroring the launch y/n prompt in the caller. Declining, or a failed
// upgrade, is non-fatal: the launch proceeds with the installed version, just
// like the offline degrade-to-warning path.
func promptUpgrade(installed, latest string) {
	fmt.Fprintf(os.Stderr, "gs skill outdated (installed: %s, latest: %s).\n", installed, latest)
	fmt.Fprint(os.Stderr, "Upgrade now? y/n [y]: ")
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "", "y", "yes":
		if err := upgradeSkill(latest); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skill upgrade failed: %s; continuing with installed version\n", err)
			return
		}
		log.Printf("[INFO] gs skill upgraded to %s", latest)
	}
}

// upgradeSkill installs the skill at version latest into skillDir, mirroring
// install.sh: a sparse, blobless clone of just the skills/gs subtree at the
// skills/gs/<latest> tag, then copied over skillDir with install.sh dropped.
func upgradeSkill(latest string) error {
	tmp, err := os.MkdirTemp("", "gs-skill-")
	if err != nil {
		return errutil.Explain(err, "create temp dir")
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	repo := filepath.Join(tmp, "repo")
	sub := strings.TrimSuffix(skillTagPrefix, "/") // "skills/gs"
	ref := skillTagPrefix + latest                 // "skills/gs/vX.Y.Z"

	cloneArgs := []string{"clone", "--depth", "1", "--branch", ref,
		"--filter=blob:none", "--sparse"}
	if !runcmd.Streaming() {
		cloneArgs = append(cloneArgs, "--quiet")
	}
	cloneArgs = append(cloneArgs, skillRemoteURL, repo)
	if err := runcmd.Run(exec.Command("git", cloneArgs...), "Cloning gs skill "+latest); err != nil {
		return err
	}
	if err := runcmd.Run(exec.Command("git", "-C", repo, "sparse-checkout", "set", "--no-cone", sub), "Selecting skill subtree"); err != nil {
		return err
	}
	return mirrorSkill(filepath.Join(repo, sub), skillDir())
}

// mirrorSkill replaces dst with the contents of src, dropping install.sh (the
// installed skill has no use for the bootstrap script). dst is removed first
// so files deleted upstream don't linger, matching install.sh's rsync --delete.
func mirrorSkill(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return errutil.Explain(err, "clear %s", dst)
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if rel == "install.sh" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
}

// fetchLatestSkillVersion asks the remote monorepo for tags matching
// `skills/gs/*` via `git ls-remote` and returns the highest by semver. A 3s
// timeout keeps the check from stalling the launch when the network is slow.
// Honors the shared -v ladder: -v logs the argv, -vv logs the exit code. The
// output is parsed here, so it is captured rather than streamed even at -vv.
func fetchLatestSkillVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", "ls-remote", "--tags",
		skillRemoteURL, "refs/tags/"+skillTagPrefix+"*")
	if runcmd.Verbosity >= runcmd.LevelCommand {
		log.Printf("[DEBUG] %s", strings.Join(c.Args, " "))
	}
	out, err := c.Output()
	if runcmd.Verbosity >= runcmd.LevelStream {
		exitCode := -1
		if c.ProcessState != nil {
			exitCode = c.ProcessState.ExitCode()
		}
		log.Printf("[DEBUG] exit=%d", exitCode)
	}
	if err != nil {
		return "", errutil.Explain(err, "git ls-remote %s", skillRemoteURL)
	}
	// semver.Compare treats invalid versions (incl. "") as less than any valid
	// one, so the zero value seeds the max scan correctly.
	latest := ""
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		_, ref, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		v := strings.TrimPrefix(ref, "refs/tags/"+skillTagPrefix)
		if semver.Compare(v, latest) > 0 {
			latest = v
		}
	}
	if latest == "" {
		return "", errutil.Explain(nil, "no %s* tag on remote", skillTagPrefix)
	}
	return latest, nil
}

// parseFrontmatterVersion pulls the `version:` value out of a SKILL.md YAML
// frontmatter block. Returns "" if not present.
func parseFrontmatterVersion(data []byte) string {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return ""
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return ""
	}
	for line := range strings.SplitSeq(s[4:4+end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "version" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
