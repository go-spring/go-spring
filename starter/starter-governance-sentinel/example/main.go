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

// Command example is a self-contained smoke test for the resilience client-side
// seams that were not already covered by starter-oauth2-client's HTTP
// round-tripper demo: the three composable policies acceptance calls for — rate
// limit + circuit breaker + retry — all carried by the recommended sentinel
// driver.
//
// (The former dialer-seam demo lived here too; it has moved with the seam to
// the mongodb starter, the one client with no per-command hook to protect.)
//
// It asserts every expectation and exits non-zero on failure, so no external
// services (or docker) are required. The blank import of the parent module
// registers "sentinel" as a resilience driver.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/resilience"

	StarterGovernanceSentinel "go-spring.org/starter-governance-sentinel"
)

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")
	driver := StarterGovernanceSentinel.NewSentinelDriver()

	demoComposedRetry(driver)

	fmt.Println("resilience seams smoke: OK")
}

// demoComposedRetry composes rate limit + circuit breaker + retry in one policy
// and proves they cooperate: a flaky upstream that 503s twice before succeeding
// is recovered transparently within the retry budget while the (generous) limit
// and breaker stay closed.
func demoComposedRetry(driver resilience.Driver) {
	exec, err := driver.NewClientExecutor("sentinel:composed", resilience.ClientPolicy{
		RateLimit:      100,
		ErrorThreshold: 10,
		Timeout:        time.Second,
		Retry:          chain.RetryPolicy{MaxRetries: 3},
	})
	if err != nil {
		fail("composed executor: %v", err)
	}

	var hits int32
	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	})
	srv.Handler = mux
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, "sentinel", nil)}
	resp, err := client.Get("http://" + ln.Addr().String() + "/flaky")
	if err != nil {
		fail("composed request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("composed retry did not recover: status %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		fail("composed retry expected 3 attempts, got %d", got)
	}
	fmt.Printf("composed policy: recovered after %d attempts (rate limit + breaker + retry)\n", atomic.LoadInt32(&hits))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
