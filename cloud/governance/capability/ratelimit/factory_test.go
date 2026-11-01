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

package ratelimit

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// repeatingProvider exports one algorithm more than once. Only two providers
// owning the same algorithm is a collision.
type repeatingProvider struct{ algorithms []string }

func (p repeatingProvider) Algorithms() []string { return p.algorithms }

func (repeatingProvider) New(string, string) (Limiter, error) { return &tokenBucket{}, nil }

func TestFactoryToleratesOneProvidersRepeats(t *testing.T) {
	f := NewFactory([]Provider{repeatingProvider{algorithms: []string{TokenBucketAlgorithm, TokenBucketAlgorithm}}})
	_, err := f.Create(TokenBucketAlgorithm, "svc")
	assert.Error(t, err).Nil()
}

func TestFactoryRejectsTwoProvidersOwningOneAlgorithm(t *testing.T) {
	assert.Panic(t, func() {
		NewFactory([]Provider{
			NewLocalProvider(),
			repeatingProvider{algorithms: []string{TokenBucketAlgorithm}},
		})
	}, "exported by two providers")
}

// TestCreateRejectsUnknownAlgorithm pins that an unregistered or empty name is
// an error, not a silent default.
func TestCreateRejectsUnknownAlgorithm(t *testing.T) {
	f := NewFactory([]Provider{NewLocalProvider()})
	_, err := f.Create("no-such-algorithm", "svc")
	assert.Error(t, err).NotNil()
	_, err = f.Create("", "svc")
	assert.Error(t, err).NotNil()
}

// TestLocalProviderExportsEveryAlgorithm pins the process-local provider's set:
// it answers all three algorithms and builds each of them.
func TestLocalProviderExportsEveryAlgorithm(t *testing.T) {
	f := NewLocalProvider()
	assert.That(t, len(f.Algorithms())).Equal(3)
	for _, name := range f.Algorithms() {
		l, err := f.New(name, "svc")
		assert.Error(t, err).Nil()
		assert.That(t, l != nil).True()
	}
}
