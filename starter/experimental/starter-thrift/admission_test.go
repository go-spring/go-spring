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

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/cloud/governance/resilience"
)

// stubAdmissionExecutor is an Executor whose outcome the test dictates. reject
// simulates a pre-call rejection; attempts > 1 simulates a policy with retries;
// fnErr records what the wrapper fed back as the call's outcome.
type stubAdmissionExecutor struct {
	reject   error
	attempts int
	runs     int
	fnErr    error
}

func (s *stubAdmissionExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	if s.reject != nil {
		return s.reject
	}
	var err error
	for range max(s.attempts, 1) {
		s.runs++
		err = fn(ctx)
		s.fnErr = err
		if err == nil {
			return nil
		}
	}
	return err
}

func (s *stubAdmissionExecutor) Close() error                    { return nil }
func (s *stubAdmissionExecutor) Refresh(resilience.Policy) error { return nil }

// countingProcessor records how often the service implementation ran, and can
// fail on demand.
type countingProcessor struct {
	runs int
	err  thrift.TException
}

func (p *countingProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	p.runs++
	if p.err != nil {
		return false, p.err
	}
	return true, nil
}

func (p *countingProcessor) ProcessorMap() map[string]thrift.TProcessorFunction { return nil }
func (p *countingProcessor) AddToProcessorMap(string, thrift.TProcessorFunction) {}

func newAdmission(inner thrift.TProcessor, exec resilience.Executor) thrift.TProcessor {
	return &admissionProcessor{inner: inner, exec: exec, resource: "thrift:test"}
}

func TestAdmission_PassThroughRunsServiceOnce(t *testing.T) {
	svc := &countingProcessor{}
	exec := &stubAdmissionExecutor{}
	ok, ex := newAdmission(svc, exec).Process(context.Background(), nil, nil)
	if !ok || ex != nil {
		t.Fatalf("pass-through should succeed: ok=%v err=%v", ok, ex)
	}
	if svc.runs != 1 || exec.runs != 1 {
		t.Fatalf("service/executor runs: got %d/%d, want 1/1", svc.runs, exec.runs)
	}
}

// A rejection must not reach the service, and must surface as a thrift
// application exception (thrift has no status-code channel).
func TestAdmission_RejectionNeverReachesService(t *testing.T) {
	for name, reject := range map[string]error{
		"rate limited":  resilience.ErrRateLimited,
		"bulkhead full": resilience.ErrBulkheadFull,
		"circuit open":  resilience.ErrCircuitOpen,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &countingProcessor{}
			ok, ex := newAdmission(svc, &stubAdmissionExecutor{reject: reject}).Process(context.Background(), nil, nil)
			if ok {
				t.Fatal("a rejected call must not report success")
			}
			appErr, isApp := ex.(thrift.TApplicationException)
			if !isApp {
				t.Fatalf("rejection should be a thrift application exception, got %T (%v)", ex, ex)
			}
			// TExceptionType() reports the exception KIND (application vs protocol);
			// TypeId() is the application error code.
			if appErr.TypeId() != thrift.INTERNAL_ERROR {
				t.Fatalf("rejection should carry INTERNAL_ERROR, got id %d", appErr.TypeId())
			}
			if !strings.Contains(ex.Error(), reject.Error()) {
				t.Fatalf("the message should name the reason, got %q", ex.Error())
			}
			if svc.runs != 0 {
				t.Fatalf("a rejected call must not reach the service, ran %d times", svc.runs)
			}
		})
	}
}

// A service exception must reach the caller unchanged AND be reported to the
// executor, or the breaker would never see server-side errors.
func TestAdmission_ServiceExceptionPassesThrough(t *testing.T) {
	want := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "no such method")
	svc := &countingProcessor{err: want}
	exec := &stubAdmissionExecutor{}
	ok, ex := newAdmission(svc, exec).Process(context.Background(), nil, nil)
	if ok || ex != want {
		t.Fatalf("service exception must pass through unchanged: ok=%v err=%v", ok, ex)
	}
	if exec.fnErr == nil {
		t.Fatal("a service exception must be fed back to the executor as a failure")
	}
}

// Inbound serving is not idempotent: a retrying policy must not replay the
// service implementation. The retry sees no error (the guard returns nil) and
// the service still runs exactly once.
func TestAdmission_DoesNotReplayService(t *testing.T) {
	svc := &countingProcessor{err: thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "boom")}
	exec := &stubAdmissionExecutor{attempts: 3}
	_, ex := newAdmission(svc, exec).Process(context.Background(), nil, nil)
	if svc.runs != 1 {
		t.Fatalf("service must run exactly once despite retries, ran %d", svc.runs)
	}
	if exec.runs != 2 {
		t.Fatalf("guard should stop the retry loop at the second call: got %d, want 2", exec.runs)
	}
	if ex == nil {
		t.Fatal("the service exception must still surface")
	}
}
