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

	"github.com/IBM/sarama"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-kafka-sarama"
)

const topic = "hello"

type Service struct {
	Client sarama.Client `autowire:"a"`
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

// publish sends a single record to the topic and waits for the broker ack,
// using a SyncProducer derived from the shared client. StartProducerSpan wraps
// the send in an OTel producer span and injects trace context into the record
// headers; it is a no-op unless starter-otel is imported.
func (s *Service) publish(ctx context.Context, value string) error {
	producer, err := sarama.NewSyncProducerFromClient(s.Client)
	if err != nil {
		return err
	}
	defer producer.Close()
	msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(value)}

	_, span := starter.StartProducerSpan(ctx, msg)
	_, _, err = producer.SendMessage(msg)
	starter.EndSpan(span, err)
	return err
}

// consume reads the first record from partition 0 starting at the oldest
// offset, using a Consumer derived from the shared client. StartConsumerSpan
// continues the trace carried in the record headers; it is a no-op unless
// starter-otel is imported.
func (s *Service) consume(ctx context.Context, timeout time.Duration) (string, error) {
	consumer, err := sarama.NewConsumerFromClient(s.Client)
	if err != nil {
		return "", err
	}
	defer consumer.Close()

	pc, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		return "", err
	}
	defer pc.Close()

	select {
	case msg := <-pc.Messages():
		_, span := starter.StartConsumerSpan(ctx, msg)
		starter.EndSpan(span, nil)
		return string(msg.Value), nil
	case err := <-pc.Errors():
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("consume timed out")
	}
}

func runTest(s *Service) {
	ctx := context.Background()

	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}

	// Consuming can lag behind producing, so poll with a bounded timeout.
	body, err := s.consume(ctx, 10*time.Second)
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
