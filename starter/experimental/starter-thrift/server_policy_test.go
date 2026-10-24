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

package StarterThrift

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/inbound/admission"

	"github.com/apache/thrift/lib/go/thrift"
)

// countingFunc is a service implementation: it records how often it ran and can
// fail on demand. The middleware seam makes the service a TProcessorFunction,
// not a TProcessor, so the tests drive [admit] directly with one.
type countingFunc struct {
	runs int
	err  thrift.TException
}

func (f *countingFunc) Process(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
	f.runs++
	if f.err != nil {
		return false, f.err
	}
	return true, nil
}

// inboundFor builds the inbound governance object for the thrift test label over
// a guard manager armed with p.
func inboundFor(t *testing.T, p admission.Policy) *runtime.Inbound {
	t.Helper()
	mgr := admission.NewManager()
	_ = mgr.Apply(admission.Settings{Default: p})
	in, err := runtime.InboundOver("thrift", "thrift:test", mgr)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestServerPolicy_PassThroughRunsServiceOnce(t *testing.T) {
	svc := &countingFunc{}
	in := inboundFor(t, admission.Policy{})
	ok, ex := admit(context.Background(), in, "thrift:test", "Ping", 1, nil, nil, svc)
	if !ok || ex != nil {
		t.Fatalf("pass-through should succeed: ok=%v err=%v", ok, ex)
	}
	if svc.runs != 1 {
		t.Fatalf("service runs: got %d, want 1", svc.runs)
	}
}

// A rejection must not reach the service, and must surface as a thrift
// application exception (thrift has no status-code channel).
func TestServerPolicy_RejectionNeverReachesService(t *testing.T) {
	in := inboundFor(t, admission.Policy{RateLimit: 1, Burst: 1})

	// Consume the only token, so the request under test is the refused one.
	ok, ex := admit(context.Background(), in, "thrift:test", "Ping", 1, nil, nil, &countingFunc{})
	if !ok || ex != nil {
		t.Fatalf("the first call should be admitted: ok=%v err=%v", ok, ex)
	}

	svc := &countingFunc{}
	ok, ex = admit(context.Background(), in, "thrift:test", "Ping", 1, nil, nil, svc)
	if ok {
		t.Fatal("a rejected call must not report success")
	}
	appErr, isApp := ex.(thrift.TApplicationException)
	if !isApp {
		t.Fatalf("rejection should be a thrift application exception, got %T (%v)", ex, ex)
	}
	if appErr.TypeId() != thrift.INTERNAL_ERROR {
		t.Fatalf("rejection should carry INTERNAL_ERROR, got id %d", appErr.TypeId())
	}
	if !strings.Contains(ex.Error(), "rate limited") {
		t.Fatalf("the message should name the reason, got %q", ex.Error())
	}
	if svc.runs != 0 {
		t.Fatalf("a rejected call must not reach the service, ran %d times", svc.runs)
	}
}

// A service exception must reach the caller unchanged.
func TestServerPolicy_ServiceExceptionPassesThrough(t *testing.T) {
	want := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "no such method")
	svc := &countingFunc{err: want}
	in := inboundFor(t, admission.Policy{})
	ok, ex := admit(context.Background(), in, "thrift:test", "Ping", 1, nil, nil, svc)
	if ok || ex != want {
		t.Fatalf("service exception must pass through unchanged: ok=%v err=%v", ok, ex)
	}
}

// Inbound serving is not idempotent: the service implementation runs exactly
// once.
func TestServerPolicy_DoesNotReplayService(t *testing.T) {
	svc := &countingFunc{err: thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "boom")}
	in := inboundFor(t, admission.Policy{})
	_, ex := admit(context.Background(), in, "thrift:test", "Ping", 1, nil, nil, svc)
	if svc.runs != 1 {
		t.Fatalf("service must run exactly once, ran %d", svc.runs)
	}
	if ex == nil {
		t.Fatal("the service exception must still surface")
	}
}
