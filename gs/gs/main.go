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

package main

import (
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go-spring.org/gs/cmd"
	"go-spring.org/gs/internal/logfmt"
	"go-spring.org/gs/tool"
)

const Version = "v0.3.0"

// builtins are subcommands compiled directly into the gs binary.
var builtins = map[string]*cobra.Command{
	"init": cmd.NewInitCmd(),
	"gen":  cmd.NewGenCmd(),
	"add":  cmd.NewAddCmd(),
}

// helpFlags trigger showHelp when passed as the first argument.
// `-h` mirrors what cobra auto-adds to every built-in subcommand,
// so `gs --help` and `gs <tool> --help` share the same shape.
var helpFlags = []string{"--help", "-h"}

func main() {
	// Subcommands emit progress lines via the standard log package; keep the
	// prefix compact and let the default stderr sink stay.
	log.SetFlags(log.Ltime)
	logfmt.Setup()

	if len(os.Args) <= 1 || slices.Contains(helpFlags, os.Args[1]) {
		showHelp()
		return
	}

	sub := os.Args[1]

	// Built-in subcommand: hand off to cobra with argv[0] shifted so its
	// own name shows up in usage strings.
	if c, ok := builtins[sub]; ok {
		os.Args = os.Args[1:]
		if err := c.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// External tool: dispatch to gs-<sub> next to the gs binary.
	tool.Call(sub, os.Args[2:]...)
}

func showHelp() {
	// tabwriter aligns tab-separated cells row-to-row. Lines without tabs
	// pass through unchanged and break the alignment run.
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()

	fmt.Fprintf(tw, "Go-Spring Toolkit Manager %s.\n\n", Version)
	fmt.Fprintln(tw, "gs runs built-in subcommands directly. Any other command <name> is dispatched")
	fmt.Fprintln(tw, "to a `gs-<name>` executable next to gs (typically in `$GOPATH/bin`).")
	fmt.Fprintln(tw)

	fmt.Fprintln(tw, "Built-in subcommands:")
	for _, n := range slices.Sorted(maps.Keys(builtins)) {
		fmt.Fprintf(tw, "  %s\t%s\n", n, builtins[n].Short)
	}

	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "External tools:")
	externals := tool.Scan()
	if len(externals) == 0 {
		fmt.Fprintln(tw, "  No external tools found.")
	}
	for _, n := range externals {
		v, desc, err := tool.Info(n)
		if err != nil {
			fmt.Fprintf(tw, "  %s\t\tFailed to get info: %s\n", n, err.Error())
			continue
		}
		fmt.Fprintf(tw, "  %s\t(%s)\t%s\n", n, v, desc)
	}

	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "Usage:")
	fmt.Fprintln(tw, "  gs --help         Show this help")
	fmt.Fprintln(tw, "  gs <tool> --help  Show help for <tool>")
	fmt.Fprintln(tw, "  gs <tool> [args]  Run <tool> with the given arguments")
}
