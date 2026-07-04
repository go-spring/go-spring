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

// Package tool discovers and dispatches external gs subcommands.
//
// # External tool protocol
//
// An external tool is any executable that satisfies:
//
//  1. Naming: the binary is named "gs-<name>" and lives in the same
//     directory as the gs binary. Scan only filters by this filename
//     prefix; it does not check the executable bit and does not recurse
//     into subdirectories.
//
//  2. --version output: when invoked as `gs-<name> --version`, the tool
//     must print exactly two lines to stdout:
//
//     line 1: short description
//     line 2: version string
//
//     This output feeds `gs --help` listings via Info.
//
// # Invocation
//
// Call wires the tool's stdin, stdout, and stderr live to the user's
// terminal and propagates its exit code. Arguments are passed through
// unchanged.
package tool

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"go-spring.org/stdlib/errutil"
)

// Prefix is the naming convention every external tool must follow:
// the binary is looked up as "gs-<name>" in the same directory as gs.
const Prefix = "gs-"

// childEnv is the environment passed to every external tool. It inherits
// the parent env and silences mockey's gcflags self-check, which prints
// a two-line warning on startup for any binary that links mockey (e.g.
// gs-mock) and breaks the two-line --version contract.
func childEnv() []string {
	return append(os.Environ(), "MOCKEY_CHECK_GCFLAGS=false")
}

// execDir is the directory that hosts the gs binary; external tools
// are discovered next to it.
var execDir string

func init() {
	filename, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to determine executable path: %s\n", err.Error())
		os.Exit(1)
	}
	execDir = filepath.Dir(filename)
}

// Scan lists external tools next to the gs binary. Returned names are
// the subcommand form (no "gs-" prefix), sorted lexicographically.
func Scan() []string {
	entries, err := os.ReadDir(execDir)
	if err != nil {
		fmt.Printf("Error reading directory %s: %s\n", execDir, err.Error())
		os.Exit(1)
	}

	var tools []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if name, ok := strings.CutPrefix(entry.Name(), Prefix); ok {
			tools = append(tools, name)
		}
	}
	slices.Sort(tools)
	return tools
}

// Call runs the external tool "gs-<name>" with the given arguments,
// streaming its stdin/stdout/stderr live. Propagates the tool's exit
// code on failure; exits with status 1 if the tool cannot be started.
func Call(name string, args ...string) {
	toolPath := filepath.Join(execDir, Prefix+name)
	cmd := exec.Command(toolPath, args...)
	cmd.Env = childEnv()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fmt.Printf("Error running tool '%s': %s\n", Prefix+name, err.Error())
		os.Exit(1)
	}
}

// Info returns metadata for an external tool by invoking `gs-<name> --version`.
// See the package doc for the --version output contract.
func Info(name string) (version string, desc string, err error) {
	toolPath := filepath.Join(execDir, Prefix+name)
	cmd := exec.Command(toolPath, "--version")
	cmd.Env = childEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", errutil.Explain(err, "[output] %s", string(output))
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", "", errutil.Explain(nil, "invalid output: %s", string(output))
	}
	return lines[1], lines[0], nil
}
