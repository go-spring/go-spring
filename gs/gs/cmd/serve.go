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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// servePort is the TCP port `gs serve` listens on.
var servePort int

// NewServeCmd builds the `gs serve` subcommand: a throwaway static file
// server over the given directory (default "."), in the spirit of
// `python -m http.server`. Handy for eyeballing generated output or sharing
// a folder on the LAN without pulling in a real web server.
func NewServeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "serve [dir]",
		Short:        "serve a directory over HTTP (like python -m http.server)",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runServe,
	}
	c.Flags().IntVarP(&servePort, "port", "p", 8000, "port to listen on")
	runcmd.BindFlag(c)
	return c
}

// runServe resolves the target directory, binds the port, and serves files
// until interrupted. Verbosity layers extra detail onto the default startup
// line: -v logs the absolute root and every request; -vv adds remote addr.
func runServe(_ *cobra.Command, args []string) error {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return errutil.Explain(err, "resolve directory %q", dir)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return errutil.Explain(err, "access directory %q", dir)
	}
	if !info.IsDir() {
		return errutil.Explain(nil, "%q is not a directory", dir)
	}

	// Bind before announcing so a port clash surfaces as an error rather
	// than a "serving" line for a server that never came up.
	addr := ":" + strconv.Itoa(servePort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errutil.Explain(err, "listen on %s", addr)
	}

	log.Printf("[INFO] serving %q at http://localhost:%d/ (Ctrl+C to stop)", dir, servePort)
	if runcmd.Verbosity >= runcmd.LevelCommand {
		log.Printf("[DEBUG] root: %s", abs)
	}

	handler := http.FileServer(http.Dir(abs))
	if err := http.Serve(ln, logRequests(handler)); err != nil {
		return errutil.Explain(err, "serve %s", addr)
	}
	return nil
}

// logRequests wraps h with access logging gated on verbosity. At -v it logs
// method, path, and latency; at -vv it also logs the client's remote addr.
// Below -v it is a no-op so quiet mode stays quiet.
func logRequests(h http.Handler) http.Handler {
	if runcmd.Verbosity < runcmd.LevelCommand {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if runcmd.Verbosity >= runcmd.LevelStream {
			log.Printf("[DEBUG] %s %s from %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
		} else {
			log.Printf("[DEBUG] %s %s (%s)", r.Method, r.URL.Path, time.Since(start))
		}
	})
}
