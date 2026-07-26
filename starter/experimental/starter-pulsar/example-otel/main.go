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

// Package main is the observability example for starter-pulsar.
//
// It publishes a message to Pulsar and consumes it back through an
// OTel-instrumented client, verifies producer/consumer spans reach Jaeger,
// then self-exits. Traces are exported via OTLP/gRPC to Jaeger
// (docker-compose).
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

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"

	starter "go-spring.org/starter-pulsar"
)

const (
	topic        = "hello"
	subscription = "hello-sub"
)

type Service struct {
	Client pulsar.Client `autowire:"a"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Open Jaeger at http://localhost:16686")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// publish creates a producer and sends a single message to the topic.
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

// consume subscribes to the topic and reads a single message.
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

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=pulsar-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'pulsar-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'pulsar-otel-example'")

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
