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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	StarterNats "go-spring.org/starter-nats"
)

// Service wires two named NATS connections. `main` has JetStream enabled; `work`
// is an independent core connection, proving multi-instance wiring.
type Service struct {
	Main *StarterNats.Conn `autowire:"main"`
	Work *StarterNats.Conn `autowire:"work"`
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

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

func runTest(s *Service) {
	// Feature 0: health — both connections report a live status via Healthy().
	if !s.Main.Healthy() || !s.Work.Healthy() {
		fail("connection not healthy: main=%v work=%v", s.Main.Healthy(), s.Work.Healthy())
	}

	// Feature 1: core pub/sub round-trip. The publish uses PublishMsg with an
	// OTel producer span that injects trace context into the message header, and
	// the subscriber starts a consumer span linked back to it (both no-ops unless
	// starter-otel is imported).
	{
		received := make(chan string, 1)
		sub, err := s.Main.Subscribe("demo.pubsub", func(m *nats.Msg) {
			_, span := StarterNats.StartConsumeSpan(context.Background(), m)
			defer StarterNats.EndSpan(span, nil)
			select {
			case received <- string(m.Data):
			default:
			}
		})
		if err != nil {
			fail("subscribe failed: %v", err)
		}
		_ = s.Main.Flush()
		msg := &nats.Msg{Subject: "demo.pubsub", Data: []byte("hello")}
		_, span := StarterNats.StartPublishSpan(context.Background(), msg)
		err = s.Main.PublishMsg(msg)
		StarterNats.EndSpan(span, err)
		if err != nil {
			fail("publish failed: %v", err)
		}
		select {
		case body := <-received:
			if body != "hello" {
				fail("pub/sub payload mismatch: %q", body)
			}
		case <-time.After(3 * time.Second):
			fail("pub/sub timed out")
		}
		_ = sub.Unsubscribe()
	}

	// Feature 2: request-reply.
	{
		sub, err := s.Main.Subscribe("demo.rpc", func(m *nats.Msg) {
			_ = m.Respond([]byte("pong:" + string(m.Data)))
		})
		if err != nil {
			fail("responder subscribe failed: %v", err)
		}
		_ = s.Main.Flush()
		reply, err := s.Main.Request("demo.rpc", []byte("ping"), 3*time.Second)
		if err != nil {
			fail("request failed: %v", err)
		}
		if string(reply.Data) != "pong:ping" {
			fail("request-reply payload mismatch: %q", string(reply.Data))
		}
		_ = sub.Unsubscribe()
	}

	// Feature 3: queue group — each message is delivered to exactly one member,
	// so the two subscribers' counts must sum to the number published.
	{
		var got int32
		for i := 0; i < 2; i++ {
			sub, err := s.Work.QueueSubscribe("demo.queue", "workers", func(_ *nats.Msg) {
				atomic.AddInt32(&got, 1)
			})
			if err != nil {
				fail("queue subscribe failed: %v", err)
			}
			defer sub.Unsubscribe()
		}
		_ = s.Work.Flush()
		const n = 10
		for i := 0; i < n; i++ {
			if err := s.Work.Publish("demo.queue", []byte("job")); err != nil {
				fail("queue publish failed: %v", err)
			}
		}
		_ = s.Work.Flush()
		deadline := time.After(3 * time.Second)
		for atomic.LoadInt32(&got) < n {
			select {
			case <-deadline:
				fail("queue group delivered %d/%d", atomic.LoadInt32(&got), n)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}

	// Feature 4: JetStream — publish to a stream, then pull it back.
	{
		if s.Main.JetStream == nil {
			fail("JetStream context is nil on `main`")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		js := s.Main.JetStream
		stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
			Name:     "EVENTS",
			Subjects: []string{"events.>"},
		})
		if err != nil {
			fail("create stream failed: %v", err)
		}
		if _, err := js.Publish(ctx, "events.signup", []byte("user-42")); err != nil {
			fail("jetstream publish failed: %v", err)
		}
		cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Durable:   "processor",
			AckPolicy: jetstream.AckExplicitPolicy,
		})
		if err != nil {
			fail("create consumer failed: %v", err)
		}
		msg, err := cons.Next(jetstream.FetchMaxWait(3 * time.Second))
		if err != nil {
			fail("jetstream fetch failed: %v", err)
		}
		if string(msg.Data()) != "user-42" {
			fail("jetstream payload mismatch: %q", string(msg.Data()))
		}
		_ = msg.Ack()
	}

	fmt.Println("Response from server: all nats features OK")
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
