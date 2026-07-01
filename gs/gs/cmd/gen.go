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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go-spring.org/gs/cmd/proto"
	"go-spring.org/stdlib/errutil"
)

// NewGenCmd builds the `gs gen` subcommand, which generates Go server code
// from IDL files under the project's `idl/` directory.
func NewGenCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "gen",
		Short:        "gen go server code from idl files",
		SilenceUsage: true,
		RunE:         runGen,
	}
}

// runGen is the RunE handler for `gs gen`. It must be invoked from a project
// root that owns a `gs.json` and an `idl/` directory; each protocol
// subdirectory of `idl/` is dispatched to its matching generator.
func runGen(_ *cobra.Command, _ []string) error {
	currDir, err := os.Getwd()
	if err != nil {
		return errutil.Explain(err, "get working directory")
	}
	return genProject(currDir)
}

// genProject runs the code generators for the project rooted at dir. It is
// shared between `gs gen` (which passes the current working directory) and
// `gs init` (which passes the freshly created project directory).
func genProject(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "gs.json")); err != nil {
		return errutil.Explain(err, "gs.json not found: run `gs gen` from a project root")
	}

	entries, err := os.ReadDir(filepath.Join(dir, "idl"))
	if err != nil {
		return errutil.Explain(err, "read idl dir")
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// One case per supported protocol; unknown dirs are ignored so users
		// can stash notes/schemas alongside `http/`, `grpc/`, etc.
		switch e.Name() {
		case "http":
			if err := proto.GenHttp(dir); err != nil {
				return err
			}
		default: // for linter
		}
	}

	return nil
}
