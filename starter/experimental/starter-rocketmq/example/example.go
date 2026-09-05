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

	"go-spring.org/cloud/messaging"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-rocketmq"
)

const (
	topic = "hello"
	group = "hello-group"
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

// runTest publishes a message through the messaging.Driver and consumes it
// back, proving the whole produce/push-consume round trip works. The driver
// path also exercises trace injection/extraction (a no-op here) and the
// load-test marker propagation.
func runTest(s *Service) {
	ctx := context.Background()

	driver := starter.NewDriver(s.Client)

	// Subscribe before publishing so the message is not missed.
	sub, err := driver.NewSubscriber(ctx, topic, group)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "SUBSCRIBE failed: %v", err)
		os.Exit(1)
	}
	msgs := make(chan *messaging.Message, 1)
	if err = sub.Subscribe(ctx, func(_ context.Context, msg *messaging.Message) error {
		msgs <- msg
		return nil
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SUBSCRIBE failed: %v", err)
		os.Exit(1)
	}

	pub, err := driver.NewPublisher(ctx, topic)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}
	err = pub.Publish(ctx, &messaging.Message{
		Key:     "k1",
		Payload: []byte("value"),
		Headers: map[string]string{"from": "example"},
	})
	_ = pub.Close()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}

	select {
	case msg := <-msgs:
		if string(msg.Payload) != "value" || msg.Headers["from"] != "example" {
			log.Errorf(ctx, log.TagAppDef, "CONSUME failed: body=%q headers=%v", msg.Payload, msg.Headers)
			os.Exit(1)
		}
		fmt.Println("Response from server:", string(msg.Payload))
	case <-time.After(20 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "CONSUME failed: timed out waiting for message")
		os.Exit(1)
	}
	_ = sub.Close()

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
