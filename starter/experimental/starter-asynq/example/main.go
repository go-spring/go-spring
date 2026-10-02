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

// Package main is the example for starter-asynq: one instance plays both
// roles — the producer Client enqueues a task, the worker Server (enabled
// in config) runs it. The handler signals completion on a channel; the test
// asserts the round trip and self-exits.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-asynq"
)

const taskType = "example:greet"

// Service holds the producer and worker beans.
type Service struct {
	Client *starter.Client `autowire:"a"`
	Server *starter.Server `autowire:"a:server"`
}

// Init runs after gs field-injects both beans, so the handler registration
// sees a non-nil Server. It is the correct seam for "register handlers before
// the worker starts", which gs.Runner starts after Init.
func (s *Service) Init() error {
	s.Server.RegisterHandler(taskType, handleGreet)
	return nil
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// completed signals a processed task payload back to runTest.
var completed = make(chan string, 1)

func handleGreet(ctx context.Context, task *asynq.Task) error {
	var payload map[string]string
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	select {
	case completed <- payload["msg"]:
	default:
	}
	return nil
}

func runTest(s *Service) {
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"msg": "hello asynq"})
	info, err := s.Client.Enqueue(ctx, asynq.NewTask(taskType, payload), asynq.Queue("default"))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "ENQUEUE failed: %v", err)
		os.Exit(1)
	}
	_ = info

	select {
	case msg := <-completed:
		if msg != "hello asynq" {
			log.Errorf(ctx, log.TagAppDef, "HANDLER mismatch: %q", msg)
			os.Exit(1)
		}
		fmt.Println("Asynq round trip OK:", msg)
	case <-time.After(20 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "HANDLER timed out")
		os.Exit(1)
	}

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
