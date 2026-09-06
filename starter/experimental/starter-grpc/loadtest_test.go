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

package StarterGrpc

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestLoadTestUnaryInterceptor_TagsFromMetadata(t *testing.T) {
	hits := 0
	var saw bool
	handler := func(ctx context.Context, _ any) (any, error) {
		hits++
		saw = traffic.IsLoadTest(ctx)
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	intc := LoadTestUnaryInterceptor()

	// With the marker metadata: handler ctx is tagged.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(canonical.MetaKeyLoadTest, "1"))
	_, _ = intc(ctx, nil, info, handler)
	assert.That(t, hits).Equal(1)
	assert.That(t, saw).True()

	// Without it: plain ctx.
	saw = false
	ctx2 := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other", "1"))
	_, _ = intc(ctx2, nil, info, handler)
	assert.That(t, saw).False()

	// No metadata at all: still no panic, plain ctx.
	saw = false
	_, _ = intc(context.Background(), nil, info, handler)
	assert.That(t, saw).False()
}

// fakeStream is a minimal grpc.ServerStream for the stream-interceptor test.
type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func TestLoadTestStreamInterceptor_TagsFromMetadata(t *testing.T) {
	var saw bool
	handler := func(_ any, ss grpc.ServerStream) error {
		saw = traffic.IsLoadTest(ss.Context())
		return nil
	}
	info := &grpc.StreamServerInfo{}

	// Marker present in inbound metadata => stream ctx tagged.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(canonical.MetaKeyLoadTest, "1"))
	err := LoadTestStreamInterceptor()(nil, &fakeStream{ctx: ctx}, info, handler)
	assert.Error(t, err).Nil()
	assert.That(t, saw).True()

	// Absent => untagged.
	saw = false
	ctx2 := metadata.NewIncomingContext(context.Background(), nil)
	_ = LoadTestStreamInterceptor()(nil, &fakeStream{ctx: ctx2}, info, handler)
	assert.That(t, saw).False()
}

func TestUserInterceptorRegistrySnapshotAndNil(t *testing.T) {
	// Snapshot isolation: registering more after a snapshot must not mutate it.
	extMu.Lock()
	userUnary = nil
	userStream = nil
	extMu.Unlock()
	t.Cleanup(func() {
		extMu.Lock()
		userUnary = nil
		userStream = nil
		extMu.Unlock()
	})

	assert.That(t, len(currentUserUnary())).Equal(0)

	var order []string
	UseUnaryInterceptor(func(ctx context.Context, _ any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		order = append(order, "first")
		return h(ctx, nil)
	})
	UseUnaryInterceptor(func(ctx context.Context, _ any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		order = append(order, "second")
		return h(ctx, nil)
	})
	UseUnaryInterceptor(nil) // nil is ignored

	snap := currentUserUnary()
	assert.That(t, len(snap)).Equal(2)

	// First-registered is outermost: runs first when composed as buildOptions
	// does (prepended in order).
	info := &grpc.UnaryServerInfo{FullMethod: "/s/m"}
	var rt grpc.UnaryHandler = func(ctx context.Context, _ any) (any, error) { return "ok", nil }
	// Compose in registration order (snap[0] outermost).
	chain := snap[len(snap)-1]
	for i := len(snap) - 2; i >= 0; i-- {
		prev := snap[i]
		next := chain
		chain = func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			return prev(ctx, req, info, func(ctx context.Context, req any) (any, error) { return next(ctx, req, info, h) })
		}
	}
	_, _ = chain(context.Background(), nil, info, rt)
	assert.That(t, order).Equal([]string{"first", "second"})

	// Snapshot is a copy.
	UseUnaryInterceptor(func(ctx context.Context, _ any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(ctx, nil)
	})
	assert.That(t, len(snap)).Equal(2)
}
