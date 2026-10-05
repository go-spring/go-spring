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

	starter "go-spring.org/starter-cassandra"
)

type Service struct {
	Client *starter.Client `autowire:"a"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

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

// runTest exercises the Cassandra round trip: create keyspace and table,
// insert through the resilience-guarded Exec helper, read back through the
// embedded session's query path.
func runTest(s *Service) {
	ctx := context.Background()

	stmts := []string{
		"CREATE KEYSPACE IF NOT EXISTS demo WITH replication = {'class':'SimpleStrategy','replication_factor':1}",
		"CREATE TABLE IF NOT EXISTS demo.greetings (id int PRIMARY KEY, message text)",
	}
	for _, q := range stmts {
		if err := s.Client.Exec(ctx, q); err != nil {
			log.Errorf(ctx, log.TagAppDef, err, "EXEC failed (%s)", q)
			os.Exit(1)
		}
	}
	if err := s.Client.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (1, 'hello') IF NOT EXISTS"); err != nil {
		log.Errorf(ctx, log.TagAppDef, err, "EXEC failed (insert)")
		os.Exit(1)
	}

	var msg string
	if err := s.Client.Query("SELECT message FROM demo.greetings WHERE id = 1").WithContext(ctx).Scan(&msg); err != nil || msg != "hello" {
		log.Errorf(ctx, log.TagAppDef, err, "QUERY failed: msg=%q err", msg)
		os.Exit(1)
	}

	fmt.Println("Cassandra round trip OK:", msg)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

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
