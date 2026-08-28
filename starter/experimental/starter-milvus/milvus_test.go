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

package StarterMilvus

import (
	"context"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
)

// TestConfigDefaults pins the database default and that addr is required
// (validated by the expr tag at bind time).
func TestConfigDefaults(t *testing.T) {
	var c Config
	assert.That(t, c.Database).Equal("") // zero value; the := tag fills "default" at bind
	assert.That(t, c.Addr).Equal("")
}

// --- guard (per-RPC resilience via gRPC interceptors) ---

// newGuardedSlot builds a slot armed with a real executor from the default
// resilience driver, for driving the interceptors directly (no live Milvus
// server is needed).
func newGuardedSlot(t *testing.T, p resilience.Policy) *guardSlot {
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	s := &guardSlot{}
	s.arm(exec, "milvus:test")
	return s
}

// TestUnaryGuardPassThrough proves the unarmed slot (before Init) passes RPCs
// straight through — the fail-fast probe in newClient relies on this.
func TestUnaryGuardPassThrough(t *testing.T) {
	var ran int
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		ran++
		return nil
	}
	err := unaryGuard(&guardSlot{})(context.Background(), "/milvus.proto/Flush", nil, nil, nil, invoker)
	assert.Error(t, err).Nil()
	assert.That(t, ran).Equal(1)
}

// TestUnaryGuardRateLimit confirms the flow-control path: once the burst is
// spent, the RPC is rejected without reaching the invoker.
func TestUnaryGuardRateLimit(t *testing.T) {
	slot := newGuardedSlot(t, resilience.Policy{RateLimit: 1, Burst: 1})
	var ran int
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		ran++
		return nil
	}
	err := unaryGuard(slot)(context.Background(), "/milvus.proto/HasCollection", nil, nil, nil, invoker)
	assert.Error(t, err).Nil()
	err = unaryGuard(slot)(context.Background(), "/milvus.proto/ListCollections", nil, nil, nil, invoker)
	assert.Error(t, err).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(1) // the rejected call never reached the wire
}

// TestStreamGuardRateLimit confirms the stream path short-circuits the same
// way: the stream open is rejected without calling the streamer.
func TestStreamGuardRateLimit(t *testing.T) {
	slot := newGuardedSlot(t, resilience.Policy{RateLimit: 1, Burst: 1})
	var ran int
	streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		ran++
		return nil, nil
	}
	_, err := streamGuard(slot)(context.Background(), &grpc.StreamDesc{}, nil, "/milvus.proto/Search", streamer)
	assert.Error(t, err).Nil()
	_, err = streamGuard(slot)(context.Background(), &grpc.StreamDesc{}, nil, "/milvus.proto/Query", streamer)
	assert.Error(t, err).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(1)
}

// TestGuardDialOptionsKeepsDefaults pins that supplying custom dial options
// does not silently drop the SDK's default keepalive/backoff/recv-size options.
func TestGuardDialOptionsKeepsDefaults(t *testing.T) {
	opts := guardDialOptions(&guardSlot{})
	assert.That(t, len(opts)).Equal(len(client.DefaultGrpcOpts) + 2)
}
