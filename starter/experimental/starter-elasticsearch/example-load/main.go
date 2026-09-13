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

// Package main is the load-test binary for starter-elasticsearch. It wires an
// Elasticsearch client (resilience + fault hot-reloadable) and drives an
// Index + Get round-trip through the shared [loadtest] harness, printing
// throughput / latency percentiles / error breakdown. Toggle fault.* in
// conf/app.properties to "set fire" and watch the closed loop.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/cloud/experimental/loadtest"
	"go-spring.org/spring/gs"

	StarterElasticsearch "go-spring.org/starter-elasticsearch"
	_ "go-spring.org/starter-governance"
)

const indexName = "starter-es-load"

// Service is the root bean holding the autowired client instance "load".
type Service struct {
	ES *StarterElasticsearch.Client `autowire:"load"`
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
			time.Sleep(500 * time.Millisecond) // let gs wire + the client reach the cluster
			runLoad(bean.Interface().(*Service))
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad(s *Service) {
	ctx := context.Background()
	es := s.ES
	doc := `{"v":"v"}`
	op := func(ctx context.Context) error {
		idxRes, err := es.Index(indexName, strings.NewReader(doc),
			es.Index.WithDocumentID("1"),
			es.Index.WithRefresh("false"),
			es.Index.WithContext(ctx))
		if err != nil {
			return err
		}
		if idxRes.IsError() {
			return errors.New("elasticsearch: index " + idxRes.Status())
		}
		if err := idxRes.Body.Close(); err != nil {
			return err
		}
		getRes, err := es.Get(indexName, "1", es.Get.WithContext(ctx))
		if err != nil {
			return err
		}
		defer func() { _ = getRes.Body.Close() }()
		if getRes.IsError() {
			return fmt.Errorf("elasticsearch: get %s", getRes.Status())
		}
		return nil
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
