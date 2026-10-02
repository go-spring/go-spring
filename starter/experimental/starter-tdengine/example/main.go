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
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-tdengine"
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

// runTest exercises the TDengine round trip: create database + super table,
// write one meter row through the super table, read the aggregate back.
func runTest(s *Service) {
	ctx := context.Background()

	stmts := []string{
		"CREATE DATABASE IF NOT EXISTS power",
		"CREATE STABLE IF NOT EXISTS power.meters (ts TIMESTAMP, current FLOAT) TAGS (location BINARY(24))",
		"INSERT INTO power.d001 USING power.meters TAGS('beijing') VALUES (NOW, 10.5)",
	}
	for _, q := range stmts {
		if _, err := s.Client.ExecContext(ctx, q); err != nil {
			log.Errorf(ctx, log.TagAppDef, "EXEC failed (%s): %v", q, err)
			os.Exit(1)
		}
	}

	rows, err := s.Client.QueryContext(ctx, "SELECT COUNT(*) FROM power.meters")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "QUERY failed: %v", err)
		os.Exit(1)
	}
	defer rows.Close()
	if !rows.Next() {
		log.Errorf(ctx, log.TagAppDef, "QUERY failed: no rows")
		os.Exit(1)
	}
	var n int
	if err = rows.Scan(&n); err != nil || n < 1 {
		log.Errorf(ctx, log.TagAppDef, "SCAN failed: n=%d err=%v", n, err)
		os.Exit(1)
	}

	fmt.Println("TDengine round trip OK: rows =", n)
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
