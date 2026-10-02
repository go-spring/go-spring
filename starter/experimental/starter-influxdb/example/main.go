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
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-influxdb"
)

type Service struct {
	Client *starter.Client `autowire:"a"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest writes a point through the blocking (resilience-guarded) path and
// reads it back through a Flux query, proving the round trip.
func runTest(s *Service) {
	ctx := context.Background()

	p := influxdb2.NewPointWithMeasurement("cpu").
		AddTag("host", "server-01").
		AddField("usage_idle", 42.5)
	if err := s.Client.WritePoints(ctx, p); err != nil {
		log.Errorf(ctx, log.TagAppDef, "WRITE failed: %v", err)
		os.Exit(1)
	}

	// The write is queryable once the bucket's write path settles.
	var table string
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for range 20 {
		raw, err := s.Client.QueryAPI(s.Client.Org()).
			QueryRaw(qctx, `from(bucket:"example") |> range(start: -1m) |> filter(fn: (r) => r._measurement == "cpu")`, influxdb2.DefaultDialect())
		if err == nil {
			table = raw
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !strings.Contains(table, "usage_idle") || !strings.Contains(table, "42.5") {
		log.Errorf(ctx, log.TagAppDef, "QUERY failed: table=%q", table)
		os.Exit(1)
	}

	fmt.Println("InfluxDB round trip OK:", strings.TrimSpace(strings.SplitN(table, "\n", 2)[0]))
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
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
