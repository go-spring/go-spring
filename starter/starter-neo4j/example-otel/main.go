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

// Package main is the observability example for starter-neo4j.
//
// It runs Cypher queries through a neo4j-otel-instrumented client, verifies
// client spans reach Jaeger, then self-exits. Traces are exported via OTLP/gRPC
// to Jaeger (docker-compose).
//
// Run with -manual to keep the server running for interactive exploration.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterNeo4j "go-spring.org/starter-neo4j"
	_ "go-spring.org/starter-otel"
)

func init() {
	StarterNeo4j.RegisterDriver("AnotherNeo4jDriver", &AnotherNeo4jDriver{})
}

// AnotherNeo4jDriver is a custom implementation of the Driver interface.
type AnotherNeo4jDriver struct{}

func (AnotherNeo4jDriver) CreateClient(c StarterNeo4j.Config) (neo4j.DriverWithContext, error) {
	log.Infof(context.Background(), log.TagAppDef, "AnotherNeo4jDriver::CreateClient")
	return neo4j.NewDriverWithContext(c.URI, neo4j.BasicAuth(c.Username, c.Password, c.Realm))
}

type Service struct {
	Neo4j     neo4j.DriverWithContext `autowire:"graph"`
	DiscNeo4j neo4j.DriverWithContext `autowire:"disc"`
}

// query runs a Cypher statement and returns the eager result.
func (s *Service) query(ctx context.Context, cypher string, params map[string]any) (*neo4j.EagerResult, error) {
	return neo4j.ExecuteQuery(ctx, s.Neo4j, cypher, params, neo4j.EagerResultTransformer)
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Open Jaeger at http://localhost:16686")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 0: readiness probe — verify connectivity before exercising queries.
	if err := StarterNeo4j.HealthCheck(ctx, s.Neo4j); err != nil {
		log.Errorf(ctx, log.TagAppDef, "HealthCheck failed: %v", err)
		os.Exit(1)
	}

	// Feature 1: CREATE / MERGE a node with properties.
	if _, err := s.query(ctx,
		"MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
		map[string]any{"name": "alice", "age": 30}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "MERGE failed: %v", err)
		os.Exit(1)
	}

	// Feature 2: MATCH the node back and verify its property.
	res, err := s.query(ctx,
		"MATCH (p:Person {name: $name}) RETURN p.age AS age",
		map[string]any{"name": "alice"})
	if err != nil || len(res.Records) == 0 {
		log.Errorf(ctx, log.TagAppDef, "MATCH failed: err=%v records=%d", err, len(res.Records))
		os.Exit(1)
	}
	age, _ := res.Records[0].Get("age")
	if age != int64(30) {
		log.Errorf(ctx, log.TagAppDef, "age expected 30, got %v", age)
		os.Exit(1)
	}

	// Feature 3: create a relationship then count it.
	if _, err := s.query(ctx,
		"MERGE (a:Person {name: $a}) MERGE (b:Person {name: $b}) MERGE (a)-[:KNOWS]->(b)",
		map[string]any{"a": "alice", "b": "bob"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "relationship MERGE failed: %v", err)
		os.Exit(1)
	}
	res, err = s.query(ctx,
		"MATCH (:Person {name: $a})-[r:KNOWS]->(:Person {name: $b}) RETURN count(r) AS c",
		map[string]any{"a": "alice", "b": "bob"})
	if err != nil || len(res.Records) == 0 {
		log.Errorf(ctx, log.TagAppDef, "relationship count failed: err=%v", err)
		os.Exit(1)
	}
	count, _ := res.Records[0].Get("c")
	if count != int64(1) {
		log.Errorf(ctx, log.TagAppDef, "relationship count expected 1, got %v", count)
		os.Exit(1)
	}

	// Cleanup: remove the nodes created by this test.
	if _, err := s.query(ctx,
		"MATCH (p:Person) WHERE p.name IN $names DETACH DELETE p",
		map[string]any{"names": []any{"alice", "bob"}}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "cleanup failed: %v", err)
		os.Exit(1)
	}

	fmt.Println("Response from server: age:", age, "knows:", count)

	// Feature 4: the discovery-backed client. Its URI host was spliced in from
	// the registered discovery backend (service-name=neo4j-cluster) at startup,
	// not taken from conf's dummy host, so a successful query proves discovery is
	// wired.
	discRes, err := neo4j.ExecuteQuery(ctx, s.DiscNeo4j,
		"RETURN 1 AS ok", nil, neo4j.EagerResultTransformer)
	if err != nil || len(discRes.Records) == 0 {
		log.Errorf(ctx, log.TagAppDef, "discovery query failed: err=%v", err)
		os.Exit(1)
	}
	ok, _ := discRes.Records[0].Get("ok")
	if ok != int64(1) {
		log.Errorf(ctx, log.TagAppDef, "discovery query expected 1, got %v", ok)
		os.Exit(1)
	}
	fmt.Println("Response from discovered server: ok:", ok)

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=neo4j-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'neo4j-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'neo4j-otel-example'")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

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