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

// Package main is the load-test binary for starter-neo4j. It wires a Neo4j
// driver (resilience + fault hot-reloadable) and drives a MERGE + count Cypher
// round-trip through the shared [loadtest] harness, printing throughput /
// latency percentiles / error breakdown. Toggle fault.* in conf/app.properties
// to "set fire" and watch the closed loop.
package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/experimental/loadtest"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-governance"
	StarterNeo4j "go-spring.org/starter-neo4j"
)

// Service is the root bean holding the autowired client instance "load".
type Service struct {
	Neo4j *StarterNeo4j.Client `autowire:"load"`
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
			time.Sleep(500 * time.Millisecond) // let gs wire + the driver verify connectivity
			runLoad(bean.Interface().(*Service))
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad(s *Service) {
	ctx := context.Background()
	op := func(ctx context.Context) error {
		// MERGE is an idempotent write; RETURN count(n) makes the round-trip a
		// write+read in one Cypher call. The node is reused across iterations.
		_, err := neo4j.ExecuteQuery(ctx, s.Neo4j,
			"MERGE (n:LoadNode {id: $id}) RETURN count(n) AS c",
			map[string]any{"id": "load"},
			neo4j.EagerResultTransformer)
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
