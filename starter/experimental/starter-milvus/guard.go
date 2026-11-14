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

// guard.go is the "command seam" concept of this starter: the per-RPC
// resilience guard, wired as gRPC client interceptors on the SDK's dial
// options. It mirrors starter-tdengine's guardedConn and starter-elasticsearch's
// resilience round-tripper — the guard rides the transport the SDK actually
// uses, so every Milvus RPC (collection, index, search, insert, ...) is
// protected without any opt-in at the call site.
//
// The interceptors are also this starter's naming seam: each one builds the
// RPC's semantic identity (see observe.go) and passes it EXPLICITLY as the
// operation argument to [chain.Executor.Execute], so the resilience layer inside
// the executor — the single emitter — names the span, the db.client.* metrics
// and the access log from it. It emits nothing itself.

package StarterMilvus

import (
	"context"
	"errors"
	"sync"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/traffic"

	"google.golang.org/grpc"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/merr"
)

// guardSlot carries the executor [NewClient] applies. The interceptors consult
// it on every RPC; before that it is transparent (nil exec) — the same shape as
// starter-tdengine's clientSlot.
type guardSlot struct {
	mu      sync.RWMutex
	exec    chain.Executor
	service string // the governance label the executor was built for
	prop    traffic.Propagator
}

// apply installs the executor the interceptors route through, with the service
// label it was built for (each call states it on its operation so a layer
// outside the executor — the fault layer — matches the same service) and the
// process traffic convention each call's operation records.
func (s *guardSlot) apply(exec chain.Executor, service string, prop traffic.Propagator) {
	s.mu.Lock()
	s.exec = exec
	s.service = service
	s.prop = prop
	s.mu.Unlock()
}

// load returns the applied executor and its service label, or nil before
// [NewClient] applies it, along with the process traffic convention.
func (s *guardSlot) load() (chain.Executor, string, traffic.Propagator) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exec, s.service, s.prop
}

// guardDialOptions returns the gRPC dial options that route every RPC through
// the slot's executor once applied. The SDK skips its own DefaultGrpcOpts when
// custom DialOptions are supplied, so they are re-added first (keepalive,
// connect backoff, 2GB recv limit) — the starter's options are additive, not a
// replacement.
func guardDialOptions(slot *guardSlot) []grpc.DialOption {
	opts := append([]grpc.DialOption{}, client.DefaultGrpcOpts...)
	opts = append(opts,
		grpc.WithChainUnaryInterceptor(unaryGuard(slot)),
		grpc.WithChainStreamInterceptor(streamGuard(slot)),
	)
	return opts
}

// unaryGuard returns the unary interceptor: each call runs inside
// exec.Execute, so a rate limiter rejects or an open circuit short-circuits
// the RPC before it is attempted.
//
// The identity is built here, where the gRPC method is in hand, and passed
// EXPLICITLY as the operation argument to exec.Execute: the executor reads the
// operation at Execute entry, so one call's signals cover every attempt,
// retries included.
func unaryGuard(slot *guardSlot) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if exec, service, prop := slot.load(); exec != nil {
			op := operation(ctx, prop, method)
			op.Service = service
			_, err := resilience.Run(ctx, exec, op, func(ctx context.Context, _ chain.Peer) (struct{}, error) {
				return struct{}{}, invoker(ctx, method, req, reply, cc, opts...)
			}, milvusOptions...)
			return err
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// milvusOptions is the call-outcome declaration every RPC shares: a request
// Milvus definitively rejected is normal, and a condition the SDK marks
// transient is retry-eligible. Both are returned verbatim.
var milvusOptions = []resilience.Option{
	resilience.TolerateFunc(isClientError),
	resilience.RetryOn(isRetryableError),
}

// retryableSentinels are the SDK's errors carrying retriable=true: a dependency
// that is not ready / unavailable / throttled, or a collection still loading.
// They are listed as sentinels rather than read through merr.IsRetryableErr,
// which only recognises an unwrapped milvusError.
var retryableSentinels = []error{
	merr.ErrServiceNotReady,
	merr.ErrServiceUnavailable,
	merr.ErrServiceRequestLimitExceeded,
	merr.ErrServiceRateLimit,
	merr.ErrCollectionNotFullyLoaded,
	merr.ErrPartitionNotFullyLoaded,
}

// isRetryableError reports whether the SDK classifies the failure as transient.
func isRetryableError(err error) bool {
	for _, sentinel := range retryableSentinels {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// isClientError reports whether Milvus gave a DEFINITE verdict on the request —
// an unknown collection/partition/database/field, an invalid parameter, a
// privilege decision — rather than a condition of the cluster. The SDK exposes
// these as sentinel values (unwrapped from its cockroachdb/errors wrapper by
// errors.Is), not as one class, so the set is listed explicitly.
//
// The list is deliberately conservative: a sentinel absent from it falls back to
// the default and counts as a downstream fault. That direction is the safe one —
// an unlisted client error is over-counted, never an unhealthy dependency
// silently tolerated. Service/cluster conditions (unavailable, not ready, rate
// limited, node/segment/IO failures, segcore) are intentionally NOT listed.
func isClientError(err error) bool {
	for _, sentinel := range clientErrorSentinels {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

var clientErrorSentinels = []error{
	merr.ErrCollectionNotFound,
	merr.ErrPartitionNotFound,
	merr.ErrDatabaseNotFound,
	merr.ErrDatabaseInvalidName,
	merr.ErrIndexNotFound,
	merr.ErrIndexNotSupported,
	merr.ErrFieldNotFound,
	merr.ErrFieldInvalidName,
	merr.ErrAliasNotFound,
	merr.ErrAliasCollectionNameConfilct,
	merr.ErrAliasAlreadyExist,
	merr.ErrParameterInvalid,
	merr.ErrIncorrectParameterFormat,
	merr.ErrMissingRequiredParameters,
	merr.ErrInvalidInsertData,
	merr.ErrCheckPrimaryKey,
	merr.ErrNeedAuthenticate,
	merr.ErrPrivilegeNotAuthenticated,
	merr.ErrPrivilegeNotPermitted,
}

// streamGuard returns the stream interceptor: the stream's open (which sends
// the initial request) runs inside exec.Execute. Per-message reads afterwards
// are driven by the caller and not additionally guarded, matching how the
// other client starters guard the request, not the response body.
//
// The identity is built here and passed explicitly to exec.Execute, exactly as
// in [unaryGuard]: the executor reads it at Execute entry, so the stream open's
// signals are named like any other RPC's.
func streamGuard(slot *guardSlot) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		if exec, service, prop := slot.load(); exec != nil {
			op := operation(ctx, prop, method)
			op.Service = service
			cs, err := resilience.Run(ctx, exec, op, func(ctx context.Context, _ chain.Peer) (grpc.ClientStream, error) {
				return streamer(ctx, desc, cc, method, opts...)
			}, milvusOptions...)
			return cs, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
