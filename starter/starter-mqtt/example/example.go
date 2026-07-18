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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-mqtt"
)

const topic = "go-spring/hello"

type Service struct {
	Client mqtt.Client `autowire:"a"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.publish("value"); err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
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
}

// publish sends a message to the topic at QoS 1.
func (s *Service) publish(payload string) error {
	token := s.Client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

// subscribeOnce subscribes to the topic and returns the first message body,
// or times out after the given duration.
func (s *Service) subscribeOnce(timeout time.Duration) (string, error) {
	received := make(chan string, 1)
	token := s.Client.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		select {
		case received <- string(msg.Payload()):
		default:
		}
	})
	token.Wait()
	if err := token.Error(); err != nil {
		return "", err
	}
	defer s.Client.Unsubscribe(topic)

	select {
	case body := <-received:
		return body, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out waiting for message on %q", topic)
	}
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature: pub/sub round-trip on a single topic at QoS 1.
	// Subscribe first so the retained-free message is delivered live,
	// then publish and assert the payload comes back.
	result := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		body, err := s.subscribeOnce(3 * time.Second)
		if err != nil {
			errCh <- err
			return
		}
		result <- body
	}()

	// Give the subscription a moment to register on the broker.
	time.Sleep(300 * time.Millisecond)
	if err := s.publish("value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUBLISH failed: %v", err)
		os.Exit(1)
	}

	select {
	case body := <-result:
		if body != "value" {
			log.Errorf(ctx, log.TagAppDef, "SUBSCRIBE failed: body=%q", body)
			os.Exit(1)
		}
		fmt.Println("Response from server:", body)
	case err := <-errCh:
		log.Errorf(ctx, log.TagAppDef, "SUBSCRIBE failed: %v", err)
		os.Exit(1)
	}

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
