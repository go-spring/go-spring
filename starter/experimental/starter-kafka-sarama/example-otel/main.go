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

// Package main is the observability example for starter-kafka-sarama.
//
// It publishes a message to a Kafka topic and consumes it back through
// instrumented Sarama producer/consumer calls, verifies traces reach Jaeger,
// then self-exits. Traces are exported via OTLP/gRPC to Jaeger (docker-compose).
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

	"github.com/IBM/sarama"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"

	starter "go-spring.org/starter-kafka-sarama"
	_ "go-spring.org/starter-otel"
)

const topic = "hello"

type Service struct {
	Client sarama.Client `autowire:"a"`
	// Prop is the process's load-test convention, handed to the publish/consume
	// seams so the marker crosses the Kafka hop.
	Prop traffic.Propagator
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	if err != nil {
		panic(err)
	}

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{Prop: prop}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 700)
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

// publish sends a single record to the topic and waits for the broker ack,
// using a SyncProducer derived from the shared client. WrapSyncProducer declares
// the publish's identity and routes the send through the resilience executor,
// which opens the publish span and injects trace context into the record
// headers; a no-op unless starter-otel is imported.
func (s *Service) publish(ctx context.Context, value string) error {
	producer, err := sarama.NewSyncProducerFromClient(s.Client)
	if err != nil {
		return err
	}
	defer producer.Close()
	producer = starter.WrapSyncProducer(s.Client, producer, s.Prop)

	msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(value)}
	_, _, err = producer.SendMessage(msg)
	return err
}

// consume reads the first record from partition 0 starting at the oldest
// offset, using a Consumer derived from the shared client. Consume continues the
// trace carried in the record headers and runs the handler under the resilience
// executor, which emits the consume span; it is a no-op unless starter-otel is
// imported.
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
		var body string
		err := starter.Consume(ctx, s.Client, msg, s.Prop, func(context.Context) error {
			body = string(msg.Value)
			return nil
		})
		return body, err
	case err := <-pc.Errors():
		return "", err
	case <-time.After(timeout):
		return "", errutil.Explain(nil, "consume timed out")
	}
}

func runTest(s *Service) {
	ctx := context.Background()

	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, err, "PUBLISH failed")
		os.Exit(1)
	}

	// Consuming can lag behind producing, so poll with a bounded timeout.
	body, err := s.consume(ctx, 10*time.Second)
	if err != nil || body != "value" {
		log.Errorf(ctx, log.TagAppDef, err, "CONSUME failed: body=%q err", body)
		os.Exit(1)
	}

	fmt.Println("Response from server:", body)

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=kafka-sarama-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'kafka-sarama-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'kafka-sarama-otel-example'")

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

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

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
