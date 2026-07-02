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

// Package runcmd runs external commands under a shared -v/-vv switch.
//
// Every call announces the step with "[INFO] <label>" so users can follow
// progress. Higher verbosity layers reveal more:
//
//   - -v   also prints the full argv (the command).
//   - -vv  also tees the child's stdout/stderr live and prints the exit
//     code (the execution process and its result).
//
// Regardless of verbosity, on failure the captured stderr is folded into
// the returned error so quiet mode still explains what broke.
package runcmd

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/stdlib/errutil"
)

// Verbosity levels; higher includes everything from lower levels.
const (
	LevelQuiet   = 0 // step lines only
	LevelCommand = 1 // + argv
	LevelStream  = 2 // + live stdout/stderr + exit code
)

// Verbosity is set by BindFlag via the shared -v count flag.
var Verbosity int

// Streaming reports whether child stdio should be shown live. Callers use
// it to decide flags like --quiet that would otherwise suppress the
// progress users asked to see at -vv or above.
func Streaming() bool { return Verbosity >= LevelStream }

// BindFlag registers -v on c, bound to the package-level Verbosity. Users
// can pass -v or -vv (or --verbose=N) on any subcommand.
func BindFlag(c *cobra.Command) {
	c.Flags().CountVarP(&Verbosity, "verbose", "v",
		"increase output detail (-v shows commands, -vv streams output and exit code)")
}

// Run executes cmd. The label doubles as the progress line ("[INFO] <label>")
// and as the prefix for the error message, so pass a short human-readable
// phrase like "Cloning layout repository" rather than an argv fragment.
func Run(cmd *exec.Cmd, label string) error {
	log.Printf("[INFO] %s", label)

	if Verbosity >= LevelCommand {
		log.Printf("[DEBUG] %s", strings.Join(cmd.Args, " "))
	}
	if Verbosity >= LevelStream && cmd.Dir != "" {
		log.Printf("[DEBUG] cwd: %s", cmd.Dir)
	}

	var stderr bytes.Buffer
	if Verbosity >= LevelStream {
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
		if cmd.Stdout == nil {
			cmd.Stdout = os.Stdout
		}
	} else {
		cmd.Stderr = &stderr
	}

	err := cmd.Run()

	if Verbosity >= LevelStream {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		log.Printf("[DEBUG] exit=%d", exitCode)
	}

	if err != nil {
		return errutil.Explain(err, "%s: %s", label, strings.TrimSpace(stderr.String()))
	}
	return nil
}
