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

// Command example demonstrates cloud/loadtest standalone: an open-loop ramp
// against an in-process HTTP server, a verdict made of QPS / latency / error
// assertions, and a custom error classifier turning HTTP 5xx responses into
// their own bucket. It self-asserts both runs and exits non-zero on failure,
// so it doubles as the package's smoke test. No external services are
// required; the starter example-load binaries show the same harness wired to
// real middleware clients.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/loadtest"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("smoke ok: both load runs passed their verdicts")
}

func run() error {
	svr := newTarget()
	defer svr.Close()

	// One client for both runs, tuned for high request rates against a single
	// host: the default transport keeps only 2 idle conns per host, which
	// churns connections (and exhausts ephemeral ports via TIME_WAIT) under
	// thousands of ops.
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 256,
	}}

	if err := healthyRun(svr, client); err != nil {
		return err
	}
	return flakyRun(svr, client)
}

// newTarget starts the in-process system-under-test: /fast answers after a
// fixed 2ms of work; /flaky fails every 10th request with a 500 so the custom
// classifier has something to bucket.
func newTarget() *httptest.Server {
	var n atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/fast", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1)%10 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	return httptest.NewServer(mux)
}

// healthyRun ramps 200→1000 RPS over 3 seconds against /fast and expects the
// run to hold: throughput floor, p99 ceiling, near-zero errors.
func healthyRun(svr *httptest.Server, client *http.Client) error {
	fmt.Println("=== run 1: ramp against /fast, expect PASSED ===")
	op := func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, svr.URL+"/fast", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		// Drain the body so the transport can reuse the connection.
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	}

	res := loadtest.New().
		Driver(loadtest.Ramp(200, 1000, 3*time.Second, 200)).
		Duration(3*time.Second).
		CaptureGC(true).
		Assert("qps-floor", loadtest.AssertMinQPS(150)).
		Assert("p99-ceiling", loadtest.AssertP99Below(500*time.Millisecond)).
		Assert("zero-errors", loadtest.AssertErrorRateBelow(0.02)).
		Run(context.Background(), op)
	res.Print(os.Stdout)
	if !res.Passed() {
		return errors.New("healthy run verdict FAILED")
	}
	return nil
}

// flakyRun drives /flaky with the legacy closed-loop one-liner and proves the
// custom classifier: HTTP 5xx errors land in their own "http-5xx" bucket
// instead of "other", and the run's verdict is built from a custom assertion
// reading that bucket off the Result.
func flakyRun(svr *httptest.Server, client *http.Client) error {
	fmt.Println("=== run 2: closed-loop against /flaky, expect bucketed 5xx ===")
	op := func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, svr.URL+"/flaky", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		// Drain the body so the transport can reuse the connection.
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			// Wrapped so the classifier can recognize it.
			return &httpError{status: resp.StatusCode}
		}
		return nil
	}

	res := loadtest.New().
		Driver(loadtest.ClosedLoop{Concurrency: 50}).
		Duration(2*time.Second).
		Classify(func(err error) string {
			var he *httpError
			if errors.As(err, &he) && he.status >= 500 {
				return "http-5xx"
			}
			if errors.Is(err, context.Canceled) {
				// Ops still in flight when the run's duration expires are
				// cancelled — expected tail noise, not a 5xx classification miss.
				return "cancelled"
			}
			return loadtest.DefaultClassify(err)
		}).
		Assert("5xx-bucketed", func(_ context.Context, r *loadtest.Result) error {
			if r.Buckets["http-5xx"] == 0 {
				return errors.New("no http-5xx bucket recorded")
			}
			return nil
		}).
		Run(context.Background(), op)
	res.Print(os.Stdout)
	if !res.Passed() {
		return errors.New("flaky run verdict FAILED")
	}
	return nil
}

// httpError marks an op failure caused by an HTTP response status.
type httpError struct{ status int }

func (e *httpError) Error() string { return fmt.Sprintf("http status %d", e.status) }
