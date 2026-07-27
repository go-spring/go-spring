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

// Command gs-serve is an external gs tool that serves a directory over HTTP,
// like `python -m http.server`.
//
// It follows the gs external-tool protocol (see gs/gs/tool/tool.go): the
// binary is named "gs-serve", lives next to the gs binary, and prints a
// two-line description/version pair for `gs-serve --version`.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ToolVersion = "v0.0.1"

const toolDesc = "Serve a directory over HTTP (like python -m http.server)."

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--version" {
		fmt.Println(toolDesc)
		fmt.Println(ToolVersion)
		return
	}

	log.SetFlags(log.Ltime)

	args := os.Args[1:]
	verbosity := 0
	args = peelVerbosity(args, &verbosity)

	dir := "."
	port := 8000
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--port":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					port = v
				}
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				dir = args[i]
			}
		}
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("gs-serve: resolve directory %q: %v", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		log.Fatalf("gs-serve: access directory %q: %v", dir, err)
	}
	if !info.IsDir() {
		log.Fatalf("gs-serve: %q is not a directory", dir)
	}

	addr := ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("gs-serve: listen on %s: %v", addr, err)
	}

	log.Printf("[INFO] serving %q at http://localhost:%d/ (Ctrl+C to stop)", dir, port)
	if verbosity >= 1 {
		log.Printf("[DEBUG] root: %s", abs)
	}

	handler := http.FileServer(http.Dir(abs))
	if err := http.Serve(ln, logRequests(handler, verbosity)); err != nil {
		log.Fatalf("gs-serve: serve %s: %v", addr, err)
	}
}

// logRequests wraps h with access logging gated on verbosity.
func logRequests(h http.Handler, verbosity int) http.Handler {
	if verbosity < 1 {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if verbosity >= 2 {
			log.Printf("[DEBUG] %s %s from %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
		} else {
			log.Printf("[DEBUG] %s %s (%s)", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

// peelVerbosity consumes leading verbosity flags from args.
func peelVerbosity(args []string, verbosity *int) []string {
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--verbose":
			*verbosity++
		case len(a) >= 2 && a[0] == '-' && strings.Trim(a[1:], "v") == "":
			*verbosity += len(a) - 1
		default:
			return args[i:]
		}
	}
	return args[i:]
}
