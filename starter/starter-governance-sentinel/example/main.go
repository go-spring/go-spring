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

// Command example is a self-contained smoke test for the FUSED path: sentinel
// is selected as a rate-limit algorithm inside the resilience engine, so the
// same pipeline (matching, timeout, retry, metrics) runs with sentinel's flow
// control in the limiter slot.
//
// It asserts its expectations and exits non-zero on failure, so no external
// services (or docker) are required.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"

	StarterGovernanceSentinel "go-spring.org/starter-governance-sentinel"
)

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	demoRateLimitAlgorithm()

	fmt.Println("resilience seams smoke: OK")
}

// demoRateLimitAlgorithm proves the fusion: a resilience policy naming the
// sentinel limiter algorithm gets its admission decision from sentinel, while
// the call still runs through the framework's engine (so timeout, retry and the
// neutral ErrRateLimited are the engine's).
func demoRateLimitAlgorithm() {
	// Both providers feed one factory; the rule picks the algorithm by name.
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{
		ratelimit.NewLocalProvider(),
		StarterGovernanceSentinel.Provider{},
	})
	if err != nil {
		fail("ratelimit factory: %v", err)
	}

	mgr, err := resilience.NewManager(fact)
	if err != nil {
		fail("resilience manager: %v", err)
	}
	if err := mgr.ApplyRateLimit(resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
		Service: "sentinel:example",
		Rules: []resilience.RateLimitRule{{
			ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			Policy: ratelimit.Policy{
				Limiter:   StarterGovernanceSentinel.LimiterAlgorithm,
				RateLimit: 2, // two units per second
			},
		}},
	}}}); err != nil {
		fail("rate limit policy: %v", err)
	}
	exec := mgr.Wrap("sentinel:example", nil)

	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	srv.Handler = mux
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{Transport: resilience.NewRoundTripper(
		http.DefaultTransport, exec, "sentinel:example", nil)}

	var admitted, limited int
	for range 3 {
		resp, err := client.Get("http://" + ln.Addr().String() + "/ping")
		if err != nil {
			limited++
			continue
		}
		admitted++
		_ = resp.Body.Close()
	}
	if admitted != 2 || limited != 1 {
		fail("sentinel flow: got %d admitted / %d limited, want 2 / 1", admitted, limited)
	}
	fmt.Printf("sentinel limiter: %d admitted, %d rate-limited (budget 2/s)\n", admitted, limited)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
