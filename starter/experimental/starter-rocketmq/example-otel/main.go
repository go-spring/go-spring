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

// Package main is the observability example for starter-rocketmq.
//
// It publishes a message to RocketMQ and consumes it back through the manual
// OTel span helpers, verifies producer/consumer spans reach Jaeger, then
// self-exits. Traces are exported via OTLP/gRPC to Jaeger (docker-compose).
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

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"

	starter "go-spring.org/starter-rocketmq"
)

const (
	topic = "hello-otel"
	group = "hello-otel-group"
)

type Service struct {
	Client *starter.Client `autowire:"a"`
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

// publish sends a single message on a raw producer. StartProducerSpan wraps
// the send in an OTel producer span and injects trace context into the
// message user properties; it is a no-op unless starter-otel is imported.
func (s *Service) publish(ctx context.Context, value string) error {
	producer, err := s.Client.NewProducer()
	if err != nil {
		return err
	}
	defer producer.Shutdown()
	msg := primitive.NewMessage(topic, []byte(value))

	ctx, span := starter.StartProducerSpan(ctx, msg)
	_, err = producer.SendSync(ctx, msg)
	starter.EndSpan(span, err)
	return err
}

// consume subscribes a raw push consumer and reads a single message. The
// handler continues the trace carried in the message user properties via
// StartConsumerSpan; it is a no-op unless starter-otel is imported.
//
// Subscribe retries because the topic is auto-created by the concurrent first
// produce and may not be visible on the name server yet; the consumer reads
// from the first offset so the already-published message is still delivered.
func (s *Service) consume(ctx context.Context) (string, error) {
	c, err := s.Client.NewPushConsumer(consumer.WithGroupName(group),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromFirstOffset))
	if err != nil {
		return "", err
	}
	msgs := make(chan string, 1)
	for range 20 {
		err = c.Subscribe(topic, consumer.MessageSelector{Type: consumer.TAG, Expression: "*"},
			func(ctx context.Context, exts ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
				for _, ext := range exts {
					_, span := starter.StartConsumerSpan(ctx, ext)
					span.End()
					select {
					case msgs <- string(ext.Body):
					default:
					}
				}
				return consumer.ConsumeSuccess, nil
			})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		return "", err
	}
	if err = c.Start(); err != nil {
		return "", err
	}
	defer c.Shutdown()

	select {
	case body := <-msgs:
		return body, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func runTest(s *Service) {
	ctx := context.Background()

	// Subscribe before publishing so the message is not missed.
	recvCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	go func() {
		if err := s.publish(ctx, "value"); err != nil {
			log.Errorf(ctx, log.TagAppDef, err, "PUBLISH failed")
			os.Exit(1)
		}
	}()
	body, err := s.consume(recvCtx)
	if err != nil || body != "value" {
		log.Errorf(ctx, log.TagAppDef, err, "CONSUME failed: body=%q err", body)
		os.Exit(1)
	}

	fmt.Println("Response from server:", body)

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=rocketmq-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'rocketmq-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'rocketmq-otel-example'")

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

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
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
