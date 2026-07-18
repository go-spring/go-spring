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
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-kafka"
)

const topic = "hello"

type Service struct {
	Client *kgo.Client `autowire:"a"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()
}

// publish sends a single record to the topic and waits for the broker ack.
func (s *Service) publish(ctx context.Context, value string) error {
	rec := &kgo.Record{Topic: topic, Value: []byte(value)}
	return s.Client.ProduceSync(ctx, rec).FirstErr()
}

// consume polls one batch of fetches and returns the first record's value.
func (s *Service) consume(ctx context.Context) (string, error) {
	fetches := s.Client.PollFetches(ctx)
	if err := fetches.Err(); err != nil {
		return "", err
	}
	var body string
	fetches.EachRecord(func(r *kgo.Record) {
		if body == "" {
			body = string(r.Value)
		}
	})
	return body, nil
}

func runTest(s *Service) {
	ctx := context.Background()

	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}

	// Consuming can lag behind producing, so poll with a bounded timeout.
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := s.consume(pollCtx)
	if err != nil || body != "value" {
		log.Errorf(ctx, log.TagAppDef, "CONSUME failed: body=%q err=%v", body, err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", body)
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
