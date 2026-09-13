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

// Package main is the load-test binary for starter-redigo. It wires a redigo
// pool (resilience + fault hot-reloadable) and drives SET/GET through the shared
// [loadtest] harness, printing throughput / latency / error breakdown. Toggle
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

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/experimental/loadtest"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-governance" // registers the centralized governance center
	StarterRedigo "go-spring.org/starter-redigo"
)

// Service is the root bean holding the autowired pool instance "load".
type Service struct {
	Redis *StarterRedigo.Pool `autowire:"load"`
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
			time.Sleep(500 * time.Millisecond) // let gs wire + the pool dial
			runLoad(bean.Interface().(*Service))
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad(s *Service) {
	ctx := context.Background()
	pool := s.Redis
	op := func(ctx context.Context) error {
		c := pool.Get()
		defer func() { _ = c.Close() }()
		if _, err := c.Do("SET", "load", "v"); err != nil {
			return err
		}
		_, err := redis.String(c.Do("GET", "load"))
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
