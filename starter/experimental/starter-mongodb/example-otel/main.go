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

// Package main is the observability example for starter-mongodb.
//
// It runs MongoDB CRUD operations through an OTel-instrumented client,
// verifies client spans reach Jaeger, then self-exits. Traces are exported
// via OTLP/gRPC to Jaeger (docker-compose).
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

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	StarterMongoDB "go-spring.org/starter-mongodb"
	_ "go-spring.org/starter-otel"
)

type Service struct {
	Mongo     *mongo.Client `autowire:"a"`
	DiscMongo *mongo.Client `autowire:"disc"`
}

func (s *Service) coll() *mongo.Collection {
	return s.Mongo.Database("test").Collection("kv")
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		var res bson.M
		err := s.coll().FindOne(r.Context(), bson.M{"key": "key"}).Decode(&res)
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprint(res["value"])))
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		_, err := s.coll().InsertOne(r.Context(), bson.M{"key": "key", "value": "value"})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Open Jaeger at http://localhost:16686")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()
	if err := StarterMongoDB.HealthCheck(ctx, s.Mongo); err != nil {
		log.Errorf(ctx, log.TagAppDef, "HealthCheck failed: %v", err)
		os.Exit(1)
	}

	// Drop the collection first so this smoke test is deterministic
	// and idempotent across repeated runs.
	if err := s.coll().Drop(ctx); err != nil {
		log.Errorf(ctx, log.TagAppDef, "DROP failed: %v", err)
		os.Exit(1)
	}

	// Feature 1: InsertOne.
	insertRes, err := s.coll().InsertOne(ctx, bson.M{"key": "key", "value": "value"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "INSERT failed: %v", err)
		os.Exit(1)
	}
	if insertRes == nil || insertRes.InsertedID == nil {
		log.Errorf(ctx, log.TagAppDef, "INSERT returned no InsertedID")
		os.Exit(1)
	}

	// Feature 2: FindOne — value should equal "value".
	var res bson.M
	if err = s.coll().FindOne(ctx, bson.M{"key": "key"}).Decode(&res); err != nil {
		log.Errorf(ctx, log.TagAppDef, "FIND failed: %v", err)
		os.Exit(1)
	}
	if fmt.Sprint(res["value"]) != "value" {
		log.Errorf(ctx, log.TagAppDef, "FIND value mismatch: got %v", res["value"])
		os.Exit(1)
	}

	// Feature 3: UpdateOne — $set value to "value2", then re-read.
	updateRes, err := s.coll().UpdateOne(ctx,
		bson.M{"key": "key"},
		bson.M{"$set": bson.M{"value": "value2"}},
	)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "UPDATE failed: %v", err)
		os.Exit(1)
	}
	if updateRes.ModifiedCount != 1 {
		log.Errorf(ctx, log.TagAppDef, "UPDATE ModifiedCount expected 1, got %d", updateRes.ModifiedCount)
		os.Exit(1)
	}
	var res2 bson.M
	if err = s.coll().FindOne(ctx, bson.M{"key": "key"}).Decode(&res2); err != nil {
		log.Errorf(ctx, log.TagAppDef, "FIND after UPDATE failed: %v", err)
		os.Exit(1)
	}
	if fmt.Sprint(res2["value"]) != "value2" {
		log.Errorf(ctx, log.TagAppDef, "FIND after UPDATE value mismatch: got %v", res2["value"])
		os.Exit(1)
	}

	fmt.Println("Response from server:", res["value"], "->", res2["value"])

	// Feature 4: the discovery-backed client. Its address came from the
	// registered discovery backend (service-name=mongo-cluster), not from conf's
	// dummy uri, so a successful round-trip proves discovery is wired.
	discColl := s.DiscMongo.Database("test").Collection("disc")
	if err := discColl.Drop(ctx); err != nil {
		log.Errorf(ctx, log.TagAppDef, "discovery DROP failed: %v", err)
		os.Exit(1)
	}
	if _, err := discColl.InsertOne(ctx, bson.M{"key": "key", "value": "disc-value"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "discovery INSERT failed: %v", err)
		os.Exit(1)
	}
	var discRes bson.M
	if err := discColl.FindOne(ctx, bson.M{"key": "key"}).Decode(&discRes); err != nil {
		log.Errorf(ctx, log.TagAppDef, "discovery FIND failed: %v", err)
		os.Exit(1)
	}
	if fmt.Sprint(discRes["value"]) != "disc-value" {
		log.Errorf(ctx, log.TagAppDef, "discovery FIND value mismatch: got %v", discRes["value"])
		os.Exit(1)
	}
	fmt.Println("Response from discovered server:", discRes["value"])

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=mongodb-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'mongodb-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'mongodb-otel-example'")

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
