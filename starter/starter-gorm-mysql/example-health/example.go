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

// Command example demonstrates starter-gorm-mysql's health integration: each
// configured instance gets a health.Indicator (pool liveness) that is folded
// into starter-actuator's /readyz aggregate with zero per-component wiring.
// The smoke test connects, then asserts the actuator probes report UP while the
// database is healthy.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator" // aggregates the gorm health.Indicator
	gormcore "go-spring.org/starter-gorm"
	_ "go-spring.org/starter-gorm-mysql"
)

// Service autowires the "primary" gorm instance; instantiating it registers the
// per-instance health.Indicator the actuator collects.
type Service struct {
	DB *gormcore.DB `autowire:"mysql.primary"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	svc := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svc.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest verifies the database is reachable and the actuator probes report UP
// (liveness + readiness), proving the gorm health.Indicator is aggregated.
func runTest(s *Service) {
	sqlDB, err := s.DB.DB.DB()
	if err != nil {
		fail("db handle: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		fail("database ping: %v", err)
	}
	fmt.Println("database ping OK")

	const base = "http://127.0.0.1:9370"
	mustStatus(base+"/health", http.StatusOK)
	mustStatus(base+"/readyz", http.StatusOK)
	mustStatus(base+"/startupz", http.StatusOK)
	fmt.Println("health: actuator probes UP (gorm health.Indicator aggregated)")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func mustStatus(url string, want int) {
	resp, err := http.Get(url)
	if err != nil {
		fail("request %s: %v", url, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		fail("unexpected status for %s: got %d want %d", url, resp.StatusCode, want)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) resolve against the source location.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
