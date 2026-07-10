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

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	clientv3 "go.etcd.io/etcd/client/v3"

	_ "go-spring.org/starter-etcd"
)

type Service struct {
	Etcd *clientv3.Client `autowire:"__default__"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		resp, err := s.Etcd.Get(r.Context(), "key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		if len(resp.Kvs) == 0 {
			_, _ = w.Write([]byte(""))
			return
		}
		_, _ = w.Write(resp.Kvs[0].Value)
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if _, err := s.Etcd.Put(r.Context(), "key", "value"); err != nil {
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
	if _, err := s.Etcd.Put(ctx, "key", "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUT failed: %v", err)
		os.Exit(1)
	}
	resp, err := s.Etcd.Get(ctx, "key")
	if err != nil || len(resp.Kvs) == 0 || string(resp.Kvs[0].Value) != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: err=%v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", string(resp.Kvs[0].Value))
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
