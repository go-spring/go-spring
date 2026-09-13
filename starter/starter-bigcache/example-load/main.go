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

// Package main is the load-test binary for starter-bigcache. It wires a BigCache
// instance (resilience + fault hot-reloadable) and drives SET/GET through the
// shared [loadtest] harness, printing throughput / latency / error breakdown.
// BigCache is in-process so there is no external service to bring up; toggle
// fault.* in conf/app.properties to "set fire" and watch the closed loop.
package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/experimental/loadtest"
	"go-spring.org/spring/gs"

	StarterBigCache "go-spring.org/starter-bigcache"
	_ "go-spring.org/starter-governance" // registers the centralized governance center
)

// Service is the root bean holding the autowired client instance "load".
type Service struct {
	BigCache *StarterBigCache.Cache `autowire:"load"`
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
			time.Sleep(500 * time.Millisecond) // let gs wire + the cache init
			runLoad(bean.Interface().(*Service))
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad(s *Service) {
	ctx := context.Background()
	bc := s.BigCache
	op := func(ctx context.Context) error {
		if err := bc.Set("load", []byte("v")); err != nil {
			return err
		}
		_, err := bc.Get("load")
		return err
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
