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

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	_ "go-spring.org/starter-mongodb"
)

type Service struct {
	Mongo *mongo.Client `autowire:"a"`
}

func (s *Service) coll() *mongo.Collection {
	return s.Mongo.Database("test").Collection("kv")
}

func main() {

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

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
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
	if err := s.Mongo.Ping(ctx, nil); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PING failed: %v", err)
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
