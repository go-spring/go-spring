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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"go-spring.org/spring/gs"

	_ "bookman/internal/app"
	_ "bookman/internal/biz"
)

func init() {
	gs.Provide(&TestRunner{}).Export(gs.As[gs.Runner]())
}

func main() {
	gs.Run()
}

// TestRunner performs a simple test after the application starts.
type TestRunner struct {
	// PropertiesRefresher is injected by Go-Spring and can reload dynamic
	// configuration values without restarting the application.
	AppConfig *gs.PropertiesRefresher `autowire:""`
}

// Run waits for the HTTP server to start, then performs test requests.
func (r *TestRunner) Run(ctx context.Context) error {
	go func() {
		time.Sleep(time.Second)

		runStep("list initial books", http.MethodGet, "/books", "")
		runStep("get one book", http.MethodGet, "/books/978-0134190440", "")

		runStep("save a new book", http.MethodPost, "/books", `{
			"title": "Clean Architecture",
			"author": "Robert C. Martin",
			"isbn": "978-0134494166",
			"publisher": "Prentice Hall"
		}`)

		runStep("list after save", http.MethodGet, "/books", "")

		// GS_ environment variables override properties after refresh.
		// Here GS_DYNC_REFRESH_TIME maps to dync.refresh.time,
		// and RefreshProperties updates the gs.Dync field injected into BookService.
		refreshTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
		if err := os.Setenv("GS_DYNC_REFRESH_TIME", refreshTime); err != nil {
			panic(err)
		}
		if err := r.AppConfig.RefreshProperties(); err != nil {
			panic(err)
		}

		runStep("list after config refresh", http.MethodGet, "/books", "")
		runStep("delete the new book", http.MethodDelete, "/books/978-0134494166", "")
		runStep("list after delete", http.MethodGet, "/books", "")

		fmt.Println()

		// Send SIGTERM to this process so the app can exit gracefully.
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	return nil
}

// runStep sends one HTTP request and prints a compact, curl-like result.
func runStep(name string, method string, path string, body string) {
	fmt.Printf("\n=== %s ===\n", name)

	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, bytes.NewBufferString(body))
	if err != nil {
		panic(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s %s -> %s\n%s", method, path, resp.Status, string(b))
}
