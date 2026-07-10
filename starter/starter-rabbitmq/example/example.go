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

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-rabbitmq"
)

const queueName = "hello"

type Service struct {
	Conn *amqp.Connection `autowire:"__default__"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.publish(r.Context(), "value"); err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	http.HandleFunc("/consume", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		body, err := s.consume()
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(body))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/publish
	// OK%
	// ~ curl http://127.0.0.1:9090/consume
	// value%
}

// publish declares the queue and sends a message onto it.
func (s *Service) publish(ctx context.Context, body string) error {
	ch, err := s.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if _, err = ch.QueueDeclare(queueName, false, false, false, false, nil); err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(body),
	})
}

// consume declares the queue and pulls a single message from it.
func (s *Service) consume() (string, error) {
	ch, err := s.Conn.Channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()
	if _, err = ch.QueueDeclare(queueName, false, false, false, false, nil); err != nil {
		return "", err
	}
	msg, ok, err := ch.Get(queueName, true)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return string(msg.Body), nil
}

func runTest(s *Service) {
	ctx := context.Background()
	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}
	body, err := s.consume()
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
