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

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-pulsar"
)

const (
	topic        = "hello"
	subscription = "hello-sub"
)

type Service struct {
	Client pulsar.Client `autowire:"a"`
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

// publish creates a producer and sends a single message to the topic.
// StartProducerSpan wraps the send in an OTel producer span and injects trace
// context into the message properties; it is a no-op unless starter-otel is
// imported.
func (s *Service) publish(ctx context.Context, value string) error {
	producer, err := s.Client.CreateProducer(pulsar.ProducerOptions{Topic: topic})
	if err != nil {
		return err
	}
	defer producer.Close()
	msg := &pulsar.ProducerMessage{Payload: []byte(value)}

	ctx, span := starter.StartProducerSpan(ctx, msg)
	_, err = producer.Send(ctx, msg)
	starter.EndSpan(span, err)
	return err
}

// consume subscribes to the topic and reads a single message. StartConsumerSpan
// continues the trace carried in the message properties; it is a no-op unless
// starter-otel is imported.
func (s *Service) consume(ctx context.Context) (string, error) {
	consumer, err := s.Client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscription,
		Type:             pulsar.Shared,
	})
	if err != nil {
		return "", err
	}
	defer consumer.Close()
	msg, err := consumer.Receive(ctx)
	if err != nil {
		return "", err
	}
	_, span := starter.StartConsumerSpan(ctx, msg)
	consumer.Ack(msg)
	starter.EndSpan(span, nil)
	return string(msg.Payload()), nil
}

func runTest(s *Service) {
	ctx := context.Background()

	// Subscribe before publishing so the message is not missed.
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}
	body, err := s.consume(recvCtx)
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
