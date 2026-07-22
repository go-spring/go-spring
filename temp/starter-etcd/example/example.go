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
	Etcd *clientv3.Client `autowire:"a"`
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

	// Inspect TTL for the leased key.
	http.HandleFunc("/ttl", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		resp, err := s.Etcd.Get(r.Context(), "lease-key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		if len(resp.Kvs) == 0 {
			_, _ = w.Write([]byte("no lease-key"))
			return
		}
		ttlResp, err := s.Etcd.TimeToLive(r.Context(), clientv3.LeaseID(resp.Kvs[0].Lease))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf("%ds", ttlResp.TTL)))
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

	// Feature 1: Put/Get.
	if _, err := s.Etcd.Put(ctx, "key", "value"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUT failed: %v", err)
		os.Exit(1)
	}
	resp, err := s.Etcd.Get(ctx, "key")
	if err != nil || len(resp.Kvs) == 0 || string(resp.Kvs[0].Value) != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: err=%v", err)
		os.Exit(1)
	}

	// Feature 2: Watch — start the watcher before the mutation, then wait bounded.
	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	wch := s.Etcd.Watch(watchCtx, "watch-key")
	if _, err := s.Etcd.Put(ctx, "watch-key", "changed"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUT watch-key failed: %v", err)
		os.Exit(1)
	}
	var gotWatchValue string
	select {
	case wresp, ok := <-wch:
		if !ok {
			log.Errorf(ctx, log.TagAppDef, "watch channel closed unexpectedly")
			os.Exit(1)
		}
		if err := wresp.Err(); err != nil {
			log.Errorf(ctx, log.TagAppDef, "watch error: %v", err)
			os.Exit(1)
		}
		if len(wresp.Events) == 0 {
			log.Errorf(ctx, log.TagAppDef, "watch returned no events")
			os.Exit(1)
		}
		gotWatchValue = string(wresp.Events[0].Kv.Value)
	case <-time.After(3 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "watch timed out waiting for event")
		os.Exit(1)
	}
	if gotWatchValue != "changed" {
		log.Errorf(ctx, log.TagAppDef, "watch got value %q, want %q", gotWatchValue, "changed")
		os.Exit(1)
	}

	// Feature 3: Lease + TTL — grant, attach to a key, then verify remaining TTL.
	lease, err := s.Etcd.Grant(ctx, 30)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Grant failed: %v", err)
		os.Exit(1)
	}
	if _, err := s.Etcd.Put(ctx, "lease-key", "x", clientv3.WithLease(lease.ID)); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PUT lease-key failed: %v", err)
		os.Exit(1)
	}
	ttlResp, err := s.Etcd.TimeToLive(ctx, lease.ID)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "TimeToLive failed: err=%v", err)
		os.Exit(1)
	}
	if ttlResp.TTL <= 0 {
		log.Errorf(ctx, log.TagAppDef, "TimeToLive TTL non-positive: ttl=%d", ttlResp.TTL)
		os.Exit(1)
	}

	fmt.Println("Response from server:", string(resp.Kvs[0].Value), "watch:", gotWatchValue, "ttl:", ttlResp.TTL)
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
