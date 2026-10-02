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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
)

// Service injects the per-instance wrapped clients. bigcache exposes no hook/
// plugin extension point, so per-operation observability is delivered through
// the *Cache wrapper itself: Get/Set/Delete are the wrapper's own methods, and
// the wrapper emits the operation's span and metrics from its observe step -
// there is no executor chain, so nothing else is involved. Inject that type
// rather than the raw *bigcache.BigCache. The raw client is an unexported field
// of the wrapper; the other raw methods (Stats/Len/...) are re-exposed as plain
// delegations.
//
// Cache injects the same `hot` instance through the cache abstraction, exposed
// as the bean "bigcache:hot" - a second face on one cache, not a second cache.
type Service struct {
	Hot   *StarterBigCache.Cache `autowire:"hot"`
	Cold  *StarterBigCache.Cache `autowire:"cold"`
	Evict *StarterBigCache.Cache `autowire:"evict"`

	// The same 1s life-window under two cleaner settings - see Feature 9.
	Cleaned   *StarterBigCache.Cache `autowire:"cleaned"`
	Uncleaned *StarterBigCache.Cache `autowire:"uncleaned"`

	Cache *cache.Cache `autowire:"bigcache:hot"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Drop the marks the launching shell left in the environment, so the example
	// does not depend on which terminal started it. `_` is the one that matters:
	// single-character variable names are not portable - on Windows the shell
	// sets them, and inheriting one breaks the run.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

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

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
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

	fmt.Println("Two independent instances:", "hot len:", s.Hot.Len(), "cold:", string(cv))

	// Feature 4: hit/miss statistics. hot has stats-enabled (from the default
	// bucket), so a hit and a miss both show up in Stats() - the read side of
	// cache-effectiveness monitoring.
	_ = s.Hot.Set("stat-key", []byte("v"))
	_, _ = s.Hot.Get("stat-key")  // hit
	_, _ = s.Hot.Get("never-set") // miss
	if st := s.Hot.Stats(); st.Hits == 0 || st.Misses == 0 {
		log.Errorf(ctx, log.TagAppDef, "Stats() = %+v, want a hit and a miss recorded", st)
		os.Exit(1)
	} else {
		fmt.Println("Hot stats:", "hits:", st.Hits, "misses:", st.Misses)
	}

	// Feature 5: the cache abstraction. The same `hot` instance is also exposed
	// as a *cache.Cache bean named "bigcache:hot": values are JSON-encoded and a
	// miss comes back as cache.ErrMiss. BigCache expires by the instance's
	// life-window alone, so the per-call TTL is ignored - write with a 1s TTL,
	// sleep past it, and the value is still readable.
	if err := s.Cache.Set(ctx, "abstraction", "value", 1); err != nil {
		log.Errorf(ctx, log.TagAppDef, "façade SET failed: %v", err)
		os.Exit(1)
	}
	time.Sleep(1100 * time.Millisecond)
	var av string
	if err := s.Cache.Get(ctx, "abstraction", &av); err != nil || av != "value" {
		log.Errorf(ctx, log.TagAppDef, "per-call TTL must be ignored: v=%q err=%v", av, err)
		os.Exit(1)
	}
	var absent string
	if err := s.Cache.Get(ctx, "no-such-key", &absent); !errors.Is(err, cache.ErrMiss) {
		log.Errorf(ctx, log.TagAppDef, "expected cache.ErrMiss, got err=%v", err)
		os.Exit(1)
	}

	// Feature 6: eviction under a hard cap, asserted from both sides. `evict` is
	// capped at 1MB, so 4000 entries of 900B cannot all be resident: bigcache
	// drops the oldest as room runs out, and the OnRemove hook installed by the
	// custom Driver (driver.go) fires for each drop.
	big := make([]byte, 900)
	for i := range 4000 {
		if err := s.Evict.Set(fmt.Sprintf("k-%d", i), big); err != nil {
			log.Errorf(ctx, log.TagAppDef, "evict SET failed: %v", err)
			os.Exit(1)
		}
	}
	if n := s.Evict.Len(); n >= 4000 {
		log.Errorf(ctx, log.TagAppDef, "hard cap never evicted: %d entries resident", n)
		os.Exit(1)
	} else {
		fmt.Println("Eviction: 4000 written,", n, "resident")
	}
	if n := removals.Load(); n == 0 {
		log.Errorf(ctx, log.TagAppDef, "OnRemove never fired - the Driver's hook is not wired")
		os.Exit(1)
	} else {
		fmt.Println("OnRemove fired", n, "times")
	}

	// Feature 7: the HTTP surface registered in main. It is served by gs's
	// built-in HTTP server, whose address app.properties declares
	// (spring.http.server.addr). The server starts inside gs.Run, i.e. after this
	// goroutine was launched, so wait for it instead of racing it.
	base := "http://127.0.0.1:9090"
	if err := waitForHTTP(base+"/get", 5*time.Second); err != nil {
		log.Errorf(ctx, log.TagAppDef, "HTTP server never came up: %v", err)
		os.Exit(1)
	}
	if body, err := httpGet(base + "/set"); err != nil || body != "OK" {
		log.Errorf(ctx, log.TagAppDef, "GET /set: body=%q err=%v", body, err)
		os.Exit(1)
	}
	if body, err := httpGet(base + "/get"); err != nil || body != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET /get: body=%q err=%v", body, err)
		os.Exit(1)
	}
	fmt.Println("HTTP: /set then /get round-trip OK")

	// Feature 8: the rest of the cache surface. Only Get/Set/Delete are observed
	// (see USAGE 2.2); everything else the raw cache exposes is re-exposed as a
	// plain pass-through, and these are the ones with real uses.
	for i := range 100 {
		if err := s.Cold.Set(fmt.Sprintf("bulk-%d", i), []byte("v")); err != nil {
			log.Errorf(ctx, log.TagAppDef, "bulk SET failed: %v", err)
			os.Exit(1)
		}
	}
	entries := s.Cold.Len()
	if entries < 100 {
		log.Errorf(ctx, log.TagAppDef, "Len() = %d after 100 writes", entries)
		os.Exit(1)
	}
	// Iterator walks every live entry: SetNext advances, Value returns the entry
	// the iterator now points at.
	seen := 0
	for it := s.Cold.Iterator(); it.SetNext(); {
		if _, err := it.Value(); err != nil {
			log.Errorf(ctx, log.TagAppDef, "iterator Value: %v", err)
			os.Exit(1)
		}
		seen++
	}
	if seen != entries {
		log.Errorf(ctx, log.TagAppDef, "Iterator saw %d entries, Len() says %d", seen, entries)
		os.Exit(1)
	}
	fmt.Println("Surface: Len", entries, "Iterator", seen)

	// Reset empties the instance, and says so through Len.
	if err := s.Cold.Reset(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Reset failed: %v", err)
		os.Exit(1)
	}
	if n := s.Cold.Len(); n != 0 {
		log.Errorf(ctx, log.TagAppDef, "Len() = %d right after Reset", n)
		os.Exit(1)
	}
	fmt.Println("Reset: cold is empty")

	// Feature 9: what life-window does - and does not do. Both instances hold a 1s
	// life-window; only clean-window differs. Get never checks an entry's age:
	// life-window marks it stale, and the cleaner is what removes it. So after
	// waiting past the window, one instance has stopped serving the entry while
	// the other still serves it.
	if err := s.Cleaned.Set("stale", []byte("v")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "cleaned SET failed: %v", err)
		os.Exit(1)
	}
	if err := s.Uncleaned.Set("stale", []byte("v")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "uncleaned SET failed: %v", err)
		os.Exit(1)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := s.Cleaned.Get("stale"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		log.Errorf(ctx, log.TagAppDef, "cleaned: an entry past its life-window is still served (err=%v)", err)
		os.Exit(1)
	}
	if v, err := s.Uncleaned.Get("stale"); err != nil || string(v) != "v" {
		log.Errorf(ctx, log.TagAppDef, "uncleaned: want the stale value still served, got v=%q err=%v", string(v), err)
		os.Exit(1)
	}
	fmt.Println("life-window: with a cleaner the stale entry is gone; without one Get still serves it")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// httpGet performs one GET and returns the body. The handlers in main never set
// a status code, so the body is the whole answer.
func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// waitForHTTP retries until the server answers or the budget runs out.
func waitForHTTP(url string, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	var err error
	for time.Now().Before(deadline) {
		if _, err = httpGet(url); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
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
