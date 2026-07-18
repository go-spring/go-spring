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

	starter "go-spring.org/starter-rabbitmq"
)

const queueName = "hello"

type Service struct {
	Conn *amqp.Connection `autowire:"a"`
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

// publish declares the queue and sends a message onto it. The publish is wrapped
// in an OTel producer span (no-op unless starter-otel is imported), which also
// injects trace context into the message headers.
func (s *Service) publish(ctx context.Context, body string) error {
	ch, err := s.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if _, err = ch.QueueDeclare(queueName, false, false, false, false, nil); err != nil {
		return err
	}
	pub := amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(body),
	}
	ctx, span := starter.StartPublishSpan(ctx, "", queueName, &pub)
	err = ch.PublishWithContext(ctx, "", queueName, false, false, pub)
	starter.EndSpan(span, err)
	return err
}

// consume declares the queue and pulls a single message from it. Each delivery
// starts an OTel consumer span linked to the producer via the propagated trace
// context (no-op unless starter-otel is imported).
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
	_, span := starter.StartConsumeSpan(context.Background(), &msg)
	starter.EndSpan(span, nil)
	return string(msg.Body), nil
}

// getWithTimeout polls ch.Get in a bounded loop so runTest can never hang.
func getWithTimeout(ch *amqp.Channel, queue string, autoAck bool, timeout time.Duration) (amqp.Delivery, bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		msg, ok, err := ch.Get(queue, autoAck)
		if err != nil {
			return amqp.Delivery{}, false, err
		}
		if ok {
			return msg, true, nil
		}
		if time.Now().After(deadline) {
			return amqp.Delivery{}, false, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: Default-exchange publish/consume on queue "hello".
	// Purge first so a leftover message from a previous run cannot poison this test.
	if ch, err := s.Conn.Channel(); err == nil {
		_, _ = ch.QueueDeclare(queueName, false, false, false, false, nil)
		_, _ = ch.QueuePurge(queueName, false)
		_ = ch.Close()
	}
	if err := s.publish(ctx, "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}
	body, err := s.consume()
	if err != nil || body != "value" {
		log.Errorf(ctx, log.TagAppDef, "CONSUME failed: body=%q err=%v", body, err)
		os.Exit(1)
	}

	// Feature 2: Direct exchange + routing key binding.
	// Declare an auto-delete direct exchange, bind an auto-delete queue with
	// routing key "info", publish "routed", and read it back.
	routedBody, err := runDirectExchange(ctx, s)
	if err != nil || routedBody != "routed" {
		log.Errorf(ctx, log.TagAppDef, "DIRECT EXCHANGE failed: body=%q err=%v", routedBody, err)
		os.Exit(1)
	}

	// Feature 3: QoS + manual ack.
	// Set channel prefetch to 1, consume with autoAck=false, then explicitly ack.
	ackedBody, err := runQosManualAck(ctx, s)
	if err != nil || ackedBody != "ack-me" {
		log.Errorf(ctx, log.TagAppDef, "QOS/ACK failed: body=%q err=%v", ackedBody, err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", body, "routed:", routedBody, "acked:", ackedBody)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// runDirectExchange demonstrates a direct exchange bound to a queue with a
// routing key. Both the exchange and the queue are auto-deleted so the
// example stays self-contained.
func runDirectExchange(ctx context.Context, s *Service) (string, error) {
	ch, err := s.Conn.Channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

	const exchange = "logs_direct"
	const routingKey = "info"

	if err := ch.ExchangeDeclare(exchange, "direct", false, true, false, false, nil); err != nil {
		return "", err
	}
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return "", err
	}
	if err := ch.QueueBind(q.Name, routingKey, exchange, false, nil); err != nil {
		return "", err
	}
	if err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("routed"),
	}); err != nil {
		return "", err
	}
	msg, ok, err := getWithTimeout(ch, q.Name, true, 2*time.Second)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("no message on queue %q", q.Name)
	}
	return string(msg.Body), nil
}

// runQosManualAck demonstrates channel prefetch (Qos) combined with manual
// ack: the consumer must explicitly acknowledge each delivery.
func runQosManualAck(ctx context.Context, s *Service) (string, error) {
	ch, err := s.Conn.Channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

	const queue = "hello_ack"
	if _, err := ch.QueueDeclare(queue, false, true, false, false, nil); err != nil {
		return "", err
	}
	// Drain any leftover messages from previous runs.
	if _, err := ch.QueuePurge(queue, false); err != nil {
		return "", err
	}
	// Only one unacknowledged message at a time.
	if err := ch.Qos(1, 0, false); err != nil {
		return "", err
	}
	if err := ch.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("ack-me"),
	}); err != nil {
		return "", err
	}
	msg, ok, err := getWithTimeout(ch, queue, false, 2*time.Second)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("no message on queue %q", queue)
	}
	if err := msg.Ack(false); err != nil {
		return "", fmt.Errorf("ack failed: %w", err)
	}
	return string(msg.Body), nil
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
