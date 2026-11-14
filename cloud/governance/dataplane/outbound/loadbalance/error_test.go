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

package loadbalance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// ioTimeoutErr is a net.Error that times out WITHOUT wrapping
// os.ErrDeadlineExceeded, pinning the Timeout() branch of [chain.IsIOTimeout].
type ioTimeoutErr struct{}

func (ioTimeoutErr) Error() string   { return "read tcp: i/o timeout" }
func (ioTimeoutErr) Timeout() bool   { return true }
func (ioTimeoutErr) Temporary() bool { return false }

func TestClassifyRound(t *testing.T) {
	cases := []struct {
		err  error
		want errorKind
	}{
		{nil, success},
		{chain.ConnectError(errors.New("refused")), connectFailed},
		{chain.ConnectError(fmt.Errorf("wrapped: %w", net.ErrClosed)), connectFailed},
		{fmt.Errorf("wrapped: %w", chain.ConnectError(errors.New("refused"))), connectFailed},
		{chain.ResourceError(errors.New("pool exhausted")), resourceExhausted},
		{fmt.Errorf("wrapped: %w", chain.ResourceError(errors.New("resource_exhausted"))), resourceExhausted},
		{os.ErrDeadlineExceeded, timeoutFailed},
		{fmt.Errorf("wrapped: %w", os.ErrDeadlineExceeded), timeoutFailed},
		{ioTimeoutErr{}, timeoutFailed},
		{context.Canceled, contextCanceled},
		{context.DeadlineExceeded, contextCanceled},
		{fmt.Errorf("wrapped: %w", context.Canceled), contextCanceled},
		// The context deadline is what IsIOTimeout excludes: a transport
		// deadline riding beside it is not evidence about the peer.
		{fmt.Errorf("%w: %w", context.DeadlineExceeded, ioTimeoutErr{}), contextCanceled},
		// A canceled context carries no exclusion, so a real transport
		// deadline beside it still reads as the peer going silent.
		{fmt.Errorf("%w: %w", context.Canceled, ioTimeoutErr{}), timeoutFailed},
		{errors.New("boom"), unknownFailed},
	}
	for _, c := range cases {
		assert.That(t, classifyError(c.err)).Equal(c.want)
	}
}

func TestRoundOutcomeString(t *testing.T) {
	names := map[errorKind]string{
		success: "success", unknownFailed: "unknown_failed",
		connectFailed: "connect_failed", timeoutFailed: "timeout_failed",
		contextCanceled: "context_canceled", resourceExhausted: "resource_exhausted",
	}
	for v, want := range names {
		assert.That(t, v.String()).Equal(want)
	}
	// A value outside the closed set is named as such, never with a
	// business-sounding word.
	assert.String(t, errorKind(99).String()).Equal("invalid")
}
