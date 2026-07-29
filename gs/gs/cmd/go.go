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
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// NewGoCmd builds the `gs go` subcommand: a verbatim passthrough to the real
// `go` toolchain. It exists so future gs enhancements (go.work awareness,
// shared GOFLAGS, ...) can hook the toolchain from a single place; today it is
// a transparent forwarder.
//
// Flag parsing is disabled so that `go`'s own flags (`go build -v`,
// `go test -run`, ...) reach `go` untouched. This is also why gs cannot bind
// its shared -v via runcmd.BindFlag here: `-v` belongs to `go`. Instead gs
// peels leading verbosity flags — those before the go subcommand — since `go`
// never accepts a flag before its subcommand (`go -v build` is invalid), so a
// leading `-v` is unambiguously a gs flag.
func NewGoCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "go",
		Short:              "run the go toolchain (passthrough to `go`)",
		SilenceUsage:       true,
		SilenceErrors:      true, // go prints its own diagnostics to stderr
		DisableFlagParsing: true, // hand every flag to the real `go`
		RunE:               runGo,
	}
}

// runGo forwards args to the `go` binary with stdio fully inherited, so the
// toolchain's output streams live regardless of verbosity. gs verbosity only
// adds diagnostic lines around the call; it never captures or suppresses go's
// output.
func runGo(_ *cobra.Command, args []string) error {
	args = peelVerbosity(args)

	goBin, err := exec.LookPath("go")
	if err != nil {
		return errutil.Explain(err, "go toolchain not found in PATH")
	}

	c := exec.Command(goBin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	log.Printf("[INFO] go %s", strings.Join(args, " "))
	if runcmd.Verbosity >= runcmd.LevelCommand {
		log.Printf("[DEBUG] %s", strings.Join(c.Args, " "))
	}
	if runcmd.Verbosity >= runcmd.LevelStream {
		if wd, err := os.Getwd(); err == nil {
			log.Printf("[DEBUG] cwd: %s", wd)
		}
	}

	err = c.Run()

	if runcmd.Verbosity >= runcmd.LevelStream {
		exitCode := -1
		if c.ProcessState != nil {
			exitCode = c.ProcessState.ExitCode()
		}
		log.Printf("[DEBUG] exit=%d", exitCode)
	}

	if err != nil {
		// go already explained itself on stderr; surface a non-zero exit
		// without re-printing (SilenceErrors keeps cobra quiet too).
		return errutil.Explain(err, "go %s", strings.Join(args, " "))
	}
	return nil
}

// peelVerbosity consumes leading verbosity flags (-v, -vv, --verbose,
// --verbose=N) from args, folding them into runcmd.Verbosity, and returns the
// remaining args to hand to `go`. It stops at the first non-verbosity token —
// the go subcommand — so everything from there on (including go's own -v)
// passes through verbatim. The accepted shapes mirror runcmd.BindFlag's -v.
func peelVerbosity(args []string) []string {
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--verbose":
			runcmd.Verbosity++
		case strings.HasPrefix(a, "--verbose="):
			n, err := strconv.Atoi(a[len("--verbose="):])
			if err != nil {
				return args[i:]
			}
			runcmd.Verbosity += n
		case len(a) >= 2 && a[0] == '-' && strings.Trim(a[1:], "v") == "":
			runcmd.Verbosity += len(a) - 1
		default:
			return args[i:]
		}
	}
	return args[i:]
}
