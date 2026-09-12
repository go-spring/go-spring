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

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/cloud/governance/resilience"
)

// admissionProcessor wraps a thrift.TProcessor so every call runs through the
// resource's governance executor before it reaches the service implementation:
// inbound rate limiting, bulkhead isolation and circuit breaking for the whole
// server. It is the thrift counterpart of gin's and echo's admission middleware.
type admissionProcessor struct {
	inner    thrift.TProcessor
	exec     resilience.Executor
	resource string
}

// WrapAdmission returns a TProcessor that admits each call through the
// resilience executor for label. The executor is resolved through the NEUTRAL
// provider seam [resilience.ExecutorFor]: starter-govern registers a provider
// backed by the governance center, so this server gets its policy WITHOUT
// injecting *governance.Center or even importing cloud/governance. When
// governance is not configured the seam yields a transparent no-op executor, so
// the wrapper runs and never rejects. Hot-reload is driven on the backing
// executor by the provider, so inbound admission can be tightened without a
// restart, the same way every outbound client's policy is tuned.
//
// Call it inside [WrapProcessor] (i.e. wrap with admission first, then with the
// observing processor) so a rejection is still traced and counted.
func WrapAdmission(inner thrift.TProcessor, label string, system string) thrift.TProcessor {
	return &admissionProcessor{
		inner:    inner,
		exec:     resilience.ExecutorFor(system, label),
		resource: label,
	}
}

// Process runs the call through the executor. A rejection (rate limit, bulkhead
// full, open circuit) is surfaced as a TApplicationException with INTERNAL_ERROR
// — thrift has no status code channel, so the caller sees a failed call and the
// message names the reason. A service exception counts as a failure for the
// breaker, which is what lets the breaker trip on server-side errors.
//
// Inbound admission must NOT retry — a handler that has already produced side
// effects cannot be replayed — so leave Policy.MaxRetries at 0. The served
// guard makes a retrying policy harmless anyway: the service implementation
// still runs exactly once.
func (p *admissionProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	var (
		ok     bool
		svcErr thrift.TException
		served bool
	)
	err := p.exec.Execute(ctx, p.resource, func(ctx context.Context) error {
		if served {
			return nil // reentry guard: the service already ran this call
		}
		served = true
		ok, svcErr = p.inner.Process(ctx, in, out)
		if svcErr != nil {
			return svcErr
		}
		return nil
	})

	if svcErr != nil {
		return ok, svcErr
	}
	if err != nil {
		return false, thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "rejected by inbound admission: "+err.Error())
	}
	return ok, nil
}

// ProcessorMap delegates, as thrift's TProcessor contract requires.
func (p *admissionProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.inner.ProcessorMap()
}

// AddToProcessorMap delegates, as thrift's TProcessor contract requires.
func (p *admissionProcessor) AddToProcessorMap(key string, fn thrift.TProcessorFunction) {
	p.inner.AddToProcessorMap(key, fn)
}
