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

// Package main is the load-test binary for starter-gorm-postgres. It wires a
// gorm PostgreSQL client (resilience + fault hot-reloadable) and drives SELECT 1
// round trips through the shared [loadtest] harness, printing throughput /
// latency / error breakdown. Toggle fault.* in conf/app.properties to "set fire"
// and watch the closed loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/experimental/loadtest"
	"go-spring.org/spring/gs"

	"go-spring.org/starter-gorm"
	_ "go-spring.org/starter-gorm-postgres"
)

// Service is the root bean holding the autowired client instance "load".
type Service struct {
	DB *gormcore.DB `autowire:"postgres.load"`
}

var (
	concurrency = flag.Int("concurrency", 16, "concurrent workers")
	duration    = flag.Duration("duration", 5*time.Second, "load duration")
	manual      = flag.Bool("manual", false, "keep the process running for ad-hoc probing")
)

func main() {
	flag.Parse()
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")
	bean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond) // let gs wire + the client dial + ping
			runLoad(bean.Interface().(*Service))
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad(s *Service) {
	ctx := context.Background()
	db := s.DB
	op := func(ctx context.Context) error {
		var x int
		// WithContext threads the harness deadline through gorm so the
		// resilience timeout / breaker can interrupt in-flight queries.
		return db.WithContext(ctx).Raw("SELECT 1").Scan(&x).Error
	}
	loadtest.Run(ctx, loadtest.Config{Concurrency: *concurrency, Duration: *duration}, op).Print(os.Stdout)
	if !*manual {
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}
}

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
