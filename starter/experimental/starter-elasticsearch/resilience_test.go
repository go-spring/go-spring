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

package StarterElasticsearch

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// mockRT is a distinct concrete http.RoundTripper type (different from
// *http.Transport and from the observe/resilience transports) that records
// calls and returns a canned status.
type mockRT struct {
	mu     sync.Mutex
	status int
	calls  int
}

func (m *mockRT) RoundTrip(*http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return &http.Response{StatusCode: m.status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

// TestDynamicTransportSwapDifferentTypes is the regression test for the
// resilience wiring panic: dynamicTransport used to back its slot with an
// atomic.Value, which panics when two distinct concrete types are stored in a
// row (http.DefaultTransport is *http.Transport, then the observe/resilience
// round-trippers are other types). It now uses an RWMutex, so swapping between
// heterogeneous transports must neither panic nor lose the active transport.
func TestDynamicTransportSwapDifferentTypes(t *testing.T) {
	tr := newDynamicTransport()

	// The slot starts pointing at http.DefaultTransport.
	if tr.cur != http.DefaultTransport {
		t.Fatalf("initial transport: got %T, want http.DefaultTransport", tr.cur)
	}

	// Swap in a first distinct concrete type and exercise it.
	rt1 := &mockRT{status: http.StatusCreated}
	tr.Swap(rt1)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip via swapped transport: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	// Swap in a second, different concrete type — the operation that panicked
	// under atomic.Value — and confirm delegation follows the new transport.
	rt2 := &mockRT{status: http.StatusNotFound}
	tr.Swap(rt2)
	resp, err = tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip after second swap: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status after second swap: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if rt1.calls != 1 || rt2.calls != 1 {
		t.Fatalf("delegation counts: rt1=%d rt2=%d, want 1/1", rt1.calls, rt2.calls)
	}
}

// TestDynamicTransportSwapConcurrent ensures the RWMutex slot is safe under
// concurrent Swap + RoundTrip, so installTransport can hot-swap transports while
// in-flight requests read the active one.
func TestDynamicTransportSwapConcurrent(t *testing.T) {
	tr := newDynamicTransport()
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)

	const n = 64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range n {
			tr.Swap(&mockRT{status: http.StatusOK})
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			if _, err := tr.RoundTrip(req); err != nil {
				t.Errorf("round trip during swap: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}
