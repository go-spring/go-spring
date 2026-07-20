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

// Command example is a self-contained smoke test for the resilience seams that
// were not already covered by starter-oauth2-client's HTTP round-tripper demo:
//
//   - the server-side inbound Handler seam (resilience.NewHandler), exercising
//     rate limiting at admission;
//   - the client-side dialer seam (resilience.NewDialer), exercising the circuit
//     breaker at connection establishment;
//   - the three composable policies acceptance calls for — rate limit + circuit
//     breaker + retry — all carried by the recommended sentinel driver.
//
// It asserts every expectation and exits non-zero on failure, so no external
// services (or docker) are required. The blank import of the parent module
// registers "sentinel" as a resilience driver.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"go-spring.org/spring/resilience"

	_ "go-spring.org/starter-resilience"
)

func main() {
	driver, err := resilience.MustGetDriver("sentinel")
	if err != nil {
		fail("driver: %v", err)
	}

	demoInboundHandler(driver)
	demoClientDialer(driver)
	demoComposedRetry(driver)

	fmt.Println("resilience seams smoke: OK")
}

// demoInboundHandler wraps a business handler with the server-side seam under a
// 5 QPS rate limit, fires a burst of requests and asserts that admission both
// serves and sheds load (some 200, some 429) without the handler ever seeing the
// rejected requests.
func demoInboundHandler(driver resilience.Driver) {
	exec, err := driver.NewExecutor(resilience.Policy{RateLimit: 5})
	if err != nil {
		fail("inbound executor: %v", err)
	}

	var served int32
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&served, 1)
		_, _ = w.Write([]byte("ok"))
	})
	handler := resilience.NewHandler(mux, exec, func(*http.Request) string { return "inbound-hello" })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("listen: %v", err)
	}
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	url := "http://" + ln.Addr().String() + "/hello"
	var ok, limited int
	for range 20 {
		resp, err := http.Get(url)
		if err != nil {
			fail("inbound request: %v", err)
		}
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			limited++
		default:
			fail("inbound unexpected status %d", resp.StatusCode)
		}
	}

	if ok == 0 || limited == 0 {
		fail("inbound rate limit ineffective: ok=%d limited=%d", ok, limited)
	}
	if int(atomic.LoadInt32(&served)) != ok {
		fail("inbound handler saw %d calls but %d were admitted", served, ok)
	}
	fmt.Printf("inbound Handler: %d served, %d rejected with 429\n", ok, limited)
}

// demoClientDialer drives the dialer seam against an address with no listener.
// A breaker with threshold 3 trips after three refused dials; the fourth is
// short-circuited with the neutral ErrCircuitOpen before touching the network.
func demoClientDialer(driver resilience.Driver) {
	exec, err := driver.NewExecutor(resilience.Policy{ErrorThreshold: 3, OpenDuration: time.Minute})
	if err != nil {
		fail("dialer executor: %v", err)
	}

	// Reserve a port, then close it so dials are refused deterministically.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("reserve port: %v", err)
	}
	deadAddr := ln.Addr().String()
	_ = ln.Close()

	base := resilience.DialFunc((&net.Dialer{Timeout: time.Second}).DialContext)
	dial := resilience.NewDialer(base, exec, "dead-service")

	for i := 1; i <= 3; i++ {
		if _, err := dial(context.Background(), "tcp", deadAddr); err == nil {
			fail("dial %d unexpectedly succeeded", i)
		} else if errors.Is(err, resilience.ErrCircuitOpen) {
			fail("dial %d opened breaker too early", i)
		}
	}

	_, err = dial(context.Background(), "tcp", deadAddr)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		fail("breaker did not open after 3 failures: %v", err)
	}
	fmt.Println("client Dialer: circuit opened after 3 refused dials")
}

// demoComposedRetry composes rate limit + circuit breaker + retry in one policy
// and proves they cooperate: a flaky upstream that 503s twice before succeeding
// is recovered transparently within the retry budget while the (generous) limit
// and breaker stay closed.
func demoComposedRetry(driver resilience.Driver) {
	exec, err := driver.NewExecutor(resilience.Policy{
		RateLimit:      100,
		ErrorThreshold: 10,
		MaxRetries:     3,
		Timeout:        time.Second,
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

	client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}
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
