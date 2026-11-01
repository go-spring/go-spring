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

// factory.go is the stage's vocabulary and seam: the algorithm names a rule
// writes, the [Limiter] contract both call directions charge, the [Provider] one
// implementation contributes, the [Factory] that collects them, and the
// process-local provider this package contributes. Each algorithm in its own
// file.

package ratelimit

import (
	"context"
	"fmt"
)

// Names of the process-local algorithms. They are plain strings because they
// are values a rule writes; a backend adds its own prefix
// ("redis:token-bucket").
const (
	TokenBucketAlgorithm   = "token-bucket"
	SlidingWindowAlgorithm = "sliding-window"
	FixedWindowAlgorithm   = "fixed-window"
)

// Limiter is one rate-limit counter. The [Policy] is passed on every call rather
// than fixed at construction, so a configuration change applies to the counter's
// existing state instead of rebuilding it.
//
// Allow reports whether n units were admitted. admitted=false with a nil error
// means the budget is exhausted; a non-nil error means the store itself could
// not be read or updated. The two are deliberately separate: a store failure is
// not a budget decision, and each direction decides what to do with it (both
// currently allow the call).
type Limiter interface {
	Allow(ctx context.Context, policy Policy, n int) (admitted bool, err error)
}

// Provider is ONE limiter implementation — the process-local algorithms, a
// Redis store, ... Algorithms lists the algorithm names it can create, as the
// plain strings a rule writes; New builds the counter of one of them for scope.
//
// An algorithm may be exported by exactly one provider. [Factory] enforces that
// at construction and routes Create by this set, so bean names are free-form.
//
// Implementations must be safe for concurrent use.
type Provider interface {
	Algorithms() []string
	New(algorithm string, scope string) (Limiter, error)
}

// Factory holds the collected limiter providers and is the one place that
// creates counters. Callers ask it for an algorithm name and a scope; they
// never touch a [Provider].
//
// It indexes the providers by algorithm name once, at construction: Create is a
// map lookup, and an algorithm exported by two providers is a wiring error
// caught then. A provider repeating one of its own names is harmless.
type Factory struct {
	providers map[string]Provider
}

// NewFactory collects providers into the algorithm index. An algorithm exported
// by more than one provider is a wiring error and panics.
func NewFactory(providers []Provider) *Factory {
	byAlgorithm := make(map[string]Provider)
	for _, p := range providers {
		if p == nil {
			continue
		}
		seen := make(map[string]struct{})
		for _, algorithm := range p.Algorithms() {
			if _, duplicate := seen[algorithm]; duplicate {
				continue
			}
			seen[algorithm] = struct{}{}
			if _, taken := byAlgorithm[algorithm]; taken {
				panic(fmt.Errorf("ratelimit: algorithm %q exported by two providers", algorithm))
			}
			byAlgorithm[algorithm] = p
		}
	}
	return &Factory{providers: byAlgorithm}
}

// Create builds the counter of algorithm for scope. An unknown algorithm —
// including the empty name a programmatic caller forgot — is an error; a caller
// that must reject a document before serving it builds the counters the
// document enables while applying it, so construction is the check. A
// configuration document always names its algorithm (the default comes from the
// binding; see [Policy.Limiter]).
func (f *Factory) Create(algorithm string, scope string) (Limiter, error) {
	p, ok := f.providers[algorithm]
	if !ok {
		return nil, fmt.Errorf("ratelimit: unknown algorithm %q", algorithm)
	}
	return p.New(algorithm, scope)
}

// localProvider is the process-local implementation: token bucket, sliding
// window and fixed window, backed by in-memory state. It is unexported because
// callers only ever need the [Provider] value [NewLocalProvider] returns.
type localProvider struct{}

var _ Provider = localProvider{}

// NewLocalProvider returns the process-local provider.
func NewLocalProvider() Provider { return localProvider{} }

// Algorithms exports the process-local algorithms.
func (localProvider) Algorithms() []string {
	return []string{TokenBucketAlgorithm, SlidingWindowAlgorithm, FixedWindowAlgorithm}
}

// New builds the process-local counter for algorithm and scope. The counter
// starts unconfigured and materializes from the first rule its Allow sees.
func (localProvider) New(algorithm, _ string) (Limiter, error) {
	switch algorithm {
	case TokenBucketAlgorithm:
		return &tokenBucket{}, nil
	case SlidingWindowAlgorithm:
		return &slidingWindow{}, nil
	case FixedWindowAlgorithm:
		return &fixedWindow{}, nil
	}
	return nil, fmt.Errorf("ratelimit: local provider does not support %q", algorithm)
}
