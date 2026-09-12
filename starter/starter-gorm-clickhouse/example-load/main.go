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

// Package main is the load-test binary for starter-gorm-clickhouse. It wires a
// gorm ClickHouse client (resilience + fault hot-reloadable) and drives SELECT 1
// round trips through the shared [loadtest] harness, printing throughput /
// latency / error breakdown. Toggle fault.* in conf/app.properties to "set fire"
// and watch the closed loop.
package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/loadtest"
	"go-spring.org/spring/gs"
	gormcore "go-spring.org/starter-gorm"
	_ "go-spring.org/starter-gorm-clickhouse"
	_ "go-spring.org/starter-governance"
)

// Service is the root bean holding the autowired client instance "load".
type Service struct {
	DB *gormcore.DB `autowire:"clickhouse.load"`
}

var (
	concurrency = flag.Int("concurrency", 16, "concurrent workers")
	duration    = flag.Duration("duration", 5*time.Second, "load duration")
	manual      = flag.Bool("manual", false, "keep the process running for ad-hoc probing")
)

func main() {
	flag.Parse()
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
		// SELECT 1 is a safe round trip across all dialects; ClickHouse's
		// column-store schema model makes per-iteration inserts/migrations
		// brittle, so we keep the op a pure read round trip.
		return db.WithContext(ctx).Raw("SELECT 1").Scan(&x).Error
	}
	loadtest.Run(ctx, loadtest.Config{Concurrency: *concurrency, Duration: *duration}, op).Print(os.Stdout)
	if !*manual {
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}
}

// init pins the working directory to this file's directory so conf/ resolves.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		if err := os.Chdir(filepath.Dir(filename)); err != nil {
			panic(err)
		}
	}
}
