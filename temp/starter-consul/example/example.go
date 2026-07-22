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

	"github.com/hashicorp/consul/api"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-consul"
)

type Service struct {
	Consul *api.Client `autowire:"a"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		pair, _, err := s.Consul.KV().Get("key", nil)
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		if pair == nil {
			_, _ = w.Write([]byte(""))
			return
		}
		_, _ = w.Write(pair.Value)
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		_, err := s.Consul.KV().Put(&api.KVPair{Key: "key", Value: []byte("value")}, nil)
		if err != nil {
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
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: KV put/get.
	if _, err := s.Consul.KV().Put(&api.KVPair{Key: "key", Value: []byte("value")}, nil); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUT failed: %v", err)
		os.Exit(1)
	}
	pair, _, err := s.Consul.KV().Get("key", nil)
	if err != nil || pair == nil || string(pair.Value) != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: err=%v", err)
		os.Exit(1)
	}

	// Feature 2: Service registration + discovery.
	// Deregister a possible leftover from a previous run to keep this idempotent,
	// then register without a health check so the service appears immediately.
	const svcID, svcName = "echo-1", "echo"
	_ = s.Consul.Agent().ServiceDeregister(svcID)
	if err = s.Consul.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   svcID,
		Name: svcName,
		Port: 9090,
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "ServiceRegister failed: %v", err)
		os.Exit(1)
	}
	services, err := s.Consul.Agent().Services()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Agent().Services() failed: %v", err)
		os.Exit(1)
	}
	registered, ok := services[svcID]
	if !ok || registered.Service != svcName {
		log.Errorf(ctx, log.TagAppDef, "expected service %q with id %q to be registered", svcName, svcID)
		os.Exit(1)
	}

	// Feature 3: Deregister.
	if err = s.Consul.Agent().ServiceDeregister(svcID); err != nil {
		log.Errorf(ctx, log.TagAppDef, "ServiceDeregister failed: %v", err)
		os.Exit(1)
	}
	services, err = s.Consul.Agent().Services()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Agent().Services() after deregister failed: %v", err)
		os.Exit(1)
	}
	if _, still := services[svcID]; still {
		log.Errorf(ctx, log.TagAppDef, "service %q should be gone after deregister", svcID)
		os.Exit(1)
	}

	fmt.Println("Response from server:", string(pair.Value), "registered:", registered.Service, "deregistered:", svcID)
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
