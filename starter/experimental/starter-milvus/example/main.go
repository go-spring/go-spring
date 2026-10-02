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

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-milvus"
)

const coll = "go_spring_example"

type Service struct {
	Client *starter.Client `autowire:"a"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(1 * time.Second)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// Create a collection with a float vector field (dim 8).
	if err := s.Client.NewCollection(ctx, coll, 8); err != nil {
		log.Errorf(ctx, log.TagAppDef, "CREATE failed: %v", err)
		os.Exit(1)
	}

	// Insert one row with an ID and vector.
	idCol := entity.NewColumnInt64("id", []int64{1})
	vecCol := entity.NewColumnFloatVector("vector", 8, [][]float32{
		{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
	})
	if _, err := s.Client.Insert(ctx, coll, "", idCol, vecCol); err != nil {
		log.Errorf(ctx, log.TagAppDef, "INSERT failed: %v", err)
		os.Exit(1)
	}
	_ = s.Client.Flush(ctx, coll, false)

	if err := s.Client.LoadCollection(ctx, coll, false); err != nil {
		log.Errorf(ctx, log.TagAppDef, "LOAD failed: %v", err)
		os.Exit(1)
	}

	vec2search := []entity.Vector{entity.FloatVector{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}}
	sp, _ := entity.NewIndexFlatSearchParam()
	results, err := s.Client.Search(ctx, coll, nil, "", []string{"id"}, vec2search, "vector", entity.L2, 1, sp)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "SEARCH failed: %v", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		log.Errorf(ctx, log.TagAppDef, "SEARCH returned no results")
		os.Exit(1)
	}

	fmt.Println("Milvus round trip OK: hit id=", results[0].IDs.FieldData().GetScalars().GetLongData().Data)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, _ := os.Getwd()
	fmt.Println(workDir)
}
