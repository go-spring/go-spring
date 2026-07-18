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
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterMemcached "go-spring.org/starter-memcached"
)

func init() {
	StarterMemcached.RegisterDriver("AnotherMemcachedDriver", &AnotherMemcachedDriver{})
}

// AnotherMemcachedDriver is a custom implementation of the Driver interface.
type AnotherMemcachedDriver struct{}

func (AnotherMemcachedDriver) CreateClient(c StarterMemcached.Config) (*memcache.Client, error) {
	log.Infof(context.Background(), log.TagAppDef, "AnotherMemcachedDriver::CreateClient")
	return memcache.New(c.Servers...), nil
}

type Service struct {
	Memcached *memcache.Client `autowire:"cache"`
}

func main() {
	// You can change the `driver` property in the configuration file
	// and check the used Memcached driver via logs.

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	// Define a handler to GET a Memcached key value.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		item, err := s.Memcached.Get("key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(item.Value)
	})

	// Define a handler to SET a Memcached key value.
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.Memcached.Set(&memcache.Item{Key: "key", Value: []byte("value")}); err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	// Define a handler to INCR a Memcached counter key.
	http.HandleFunc("/incr", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		n, err := s.Memcached.Increment("counter", 1)
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(strconv.FormatUint(n, 10)))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/get
	// memcache: cache miss%
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: String SET/GET.
	if err := s.Memcached.Set(&memcache.Item{Key: "key", Value: []byte("value")}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
		os.Exit(1)
	}
	item, err := s.Memcached.Get("key")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "GET failed: err=%v", err)
		os.Exit(1)
	}
	if string(item.Value) != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET mismatch: v=%q", string(item.Value))
		os.Exit(1)
	}

	// Feature 2: INCR counter — seed then increment three times.
	if err := s.Memcached.Set(&memcache.Item{Key: "counter", Value: []byte("0")}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET counter failed: %v", err)
		os.Exit(1)
	}
	var n uint64
	for i := 0; i < 3; i++ {
		n, err = s.Memcached.Increment("counter", 1)
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "INCR failed: %v", err)
			os.Exit(1)
		}
	}
	if n != 3 {
		log.Errorf(ctx, log.TagAppDef, "INCR final value expected 3, got %d", n)
		os.Exit(1)
	}

	// Feature 3: DELETE + cache-miss GET.
	if err := s.Memcached.Delete("key"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "DELETE failed: %v", err)
		os.Exit(1)
	}
	if _, err := s.Memcached.Get("key"); !errors.Is(err, memcache.ErrCacheMiss) {
		log.Errorf(ctx, log.TagAppDef, "expected cache miss after delete, got err=%v", err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", string(item.Value), "counter:", n)

	// Feature 4: health check. The client's Ping probes every configured server
	// and is the readiness signal — read straight off the autowired client.
	if err := s.Memcached.Ping(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "health ping failed: %v", err)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
