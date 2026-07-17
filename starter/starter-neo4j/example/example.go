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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterNeo4j "go-spring.org/starter-neo4j"
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
	Neo4j neo4j.DriverWithContext `autowire:"graph"`
}

// query runs a Cypher statement and returns the eager result.
func (s *Service) query(ctx context.Context, cypher string, params map[string]any) (*neo4j.EagerResult, error) {
	return neo4j.ExecuteQuery(ctx, s.Neo4j, cypher, params, neo4j.EagerResultTransformer)
}

func main() {
	// You can change the `driver` property in the configuration file
	// and check the used Neo4j driver via logs.

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	// Define a handler to create a Person node.
	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		_, err := s.query(r.Context(),
			"MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
			map[string]any{"name": "alice", "age": 30})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	// Define a handler to read a Person node's age.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		res, err := s.query(r.Context(),
			"MATCH (p:Person {name: $name}) RETURN p.age AS age",
			map[string]any{"name": "alice"})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		if len(res.Records) == 0 {
			_, _ = w.Write([]byte("not found"))
			return
		}
		age, _ := res.Records[0].Get("age")
		_, _ = w.Write([]byte(fmt.Sprintf("%v", age)))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/create
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// 30%
}

func runTest(s *Service) {
	ctx := context.Background()

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
