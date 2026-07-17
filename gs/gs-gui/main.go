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

// Command gs-gui is an external gs tool that serves a small browser-based
// wizard for creating Go-Spring projects. It is a thin front-end over
// `gs init`: the wizard collects a module path and doc language, then execs
// the sibling `gs` binary and streams its output back to the page.
//
// It follows the gs external-tool protocol (see gs/gs/tool/tool.go): the
// binary is named "gs-gui", lives next to the gs binary, and prints a
// two-line description/version pair for `gs-gui --version`.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ToolVersion is the version reported by `gs-gui --version`.
const ToolVersion = "v0.0.1"

// toolDesc is the short description reported by `gs-gui --version`.
const toolDesc = "A browser-based wizard for creating Go-Spring projects."

// defaultPort is the preferred listen port; if taken, an ephemeral port is
// chosen instead.
const defaultPort = "8639"

//go:embed web/index.html
var webFS embed.FS

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(toolDesc)
		fmt.Println(ToolVersion)
		return
	}

	ln, err := listen()
	if err != nil {
		log.Fatalf("gs-gui: cannot listen: %v", err)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/create", handleCreate)

	fmt.Printf("gs-gui serving at %s\n", url)
	openBrowser(url)

	if err := http.Serve(ln, mux); err != nil {
		log.Fatalf("gs-gui: server error: %v", err)
	}
}

// listen binds defaultPort, falling back to an OS-assigned ephemeral port
// when it is already in use.
func listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+defaultPort)
	if err == nil {
		return ln, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// handleIndex serves the embedded single-page wizard.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// createRequest is the JSON body posted by the wizard form.
type createRequest struct {
	Module string `json:"module"`
	Lang   string `json:"lang"`
}

// handleCreate execs `gs init` with the requested module/lang and streams
// its combined stdout+stderr back to the browser as it runs. The response is
// a plain chunked text stream the front-end reads incrementally.
func handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Module == "" {
		http.Error(w, "module is required", http.StatusBadRequest)
		return
	}
	if req.Lang == "" {
		req.Lang = "zh"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	gsBin, err := siblingGS()
	if err != nil {
		fmt.Fprintf(w, "gs-gui: %v\n", err)
		flusher.Flush()
		return
	}

	cmd := exec.Command(gsBin, "init", "-m", req.Module, "--lang", req.Lang)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(w, "gs-gui: %v\n", err)
		flusher.Flush()
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into the same stream

	fmt.Fprintf(w, "$ gs init -m %s --lang %s\n\n", req.Module, req.Lang)
	flusher.Flush()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "gs-gui: failed to start gs: %v\n", err)
		flusher.Flush()
		return
	}

	buf := make([]byte, 1024)
	for {
		n, readErr := pipe.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			flusher.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				fmt.Fprintf(w, "\ngs-gui: read error: %v\n", readErr)
				flusher.Flush()
			}
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(w, "\ngs-gui: gs init exited with error: %v\n", err)
	} else {
		fmt.Fprintf(w, "\ngs-gui: done.\n")
	}
	flusher.Flush()
}

// siblingGS returns the path to the gs binary next to this executable,
// matching the external-tool discovery convention (tools live beside gs).
func siblingGS() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate self: %w", err)
	}
	gsBin := filepath.Join(filepath.Dir(self), "gs")
	if _, err := os.Stat(gsBin); err != nil {
		return "", fmt.Errorf("gs binary not found next to gs-gui: %w", err)
	}
	return gsBin, nil
}

// openBrowser best-effort opens url in the user's default browser. Failures
// are non-fatal: the URL is already printed to the terminal.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // linux and others
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
