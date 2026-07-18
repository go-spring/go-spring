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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
)

// removed counts entries evicted from any DefaultDriver-built cache. It proves
// the OnRemove hook registered via SetOnRemove is wired through the starter.
var removed int64

func init() {
	// Register a global eviction/expiry callback. It must be set before the
	// container starts, since the callback is captured when each cache is built.
	StarterBigCache.SetOnRemove(func(key string, entry []byte) {
		atomic.AddInt64(&removed, 1)
	})
}

type Service struct {
	Hot   *bigcache.BigCache `autowire:"hot"`
	Cold  *bigcache.BigCache `autowire:"cold"`
	Evict *bigcache.BigCache `autowire:"evict"`
}

func main() {
	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	// Define a handler to GET a value from the hot cache.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		v, err := s.Hot.Get("key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(v)
	})

	// Define a handler to SET a value into the hot cache.
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.Hot.Set("key", []byte("value")); err != nil {
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
	// ~ curl http://127.0.0.1:9090/get
	// Entry not found%
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: SET/GET on the hot cache.
	if err := s.Hot.Set("key", []byte("value")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
		os.Exit(1)
	}
	v, err := s.Hot.Get("key")
	if err != nil || string(v) != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: v=%q err=%v", string(v), err)
		os.Exit(1)
	}

	// Feature 2: DELETE + entry-not-found miss.
	if err := s.Hot.Delete("key"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "DELETE failed: %v", err)
		os.Exit(1)
	}
	if _, err := s.Hot.Get("key"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		log.Errorf(ctx, log.TagAppDef, "expected entry-not-found after delete, got err=%v", err)
		os.Exit(1)
	}

	// Feature 3: the second named instance is fully independent — a write to
	// `cold` must not be visible through `hot`, proving multi-instance wiring.
	if err := s.Cold.Set("only-cold", []byte("cold-value")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "cold SET failed: %v", err)
		os.Exit(1)
	}
	if _, err := s.Hot.Get("only-cold"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		log.Errorf(ctx, log.TagAppDef, "hot must not see cold's key, got err=%v", err)
		os.Exit(1)
	}
	cv, err := s.Cold.Get("only-cold")
	if err != nil || string(cv) != "cold-value" {
		log.Errorf(ctx, log.TagAppDef, "cold GET failed: v=%q err=%v", string(cv), err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", "hot len:", s.Hot.Len(), "cold:", string(cv))

	// Feature 4: hit/miss statistics. The hot cache has stats-enabled, so a hit
	// followed by a miss is reflected in Stats() — the read mechanism for cache
	// effectiveness monitoring.
	_, _ = s.Hot.Get("only-cold") // miss (key lives in cold)
	_ = s.Hot.Set("stat-key", []byte("v"))
	_, _ = s.Hot.Get("stat-key") // hit
	st := s.Hot.Stats()
	fmt.Println("Hot stats:", "hits:", st.Hits, "misses:", st.Misses)

	// Feature 5: OnRemove callback. Overflow the hard-capped `evict` cache so
	// older entries are evicted, firing the callback registered via SetOnRemove.
	big := make([]byte, 900)
	for i := 0; i < 4000; i++ {
		if err := s.Evict.Set(fmt.Sprintf("k-%d", i), big); err != nil {
			log.Errorf(ctx, log.TagAppDef, "evict SET failed: %v", err)
			os.Exit(1)
		}
	}
	if atomic.LoadInt64(&removed) == 0 {
		log.Errorf(ctx, log.TagAppDef, "expected OnRemove to fire on eviction, got 0")
		os.Exit(1)
	}
	fmt.Println("OnRemove fired:", atomic.LoadInt64(&removed), "times")

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
