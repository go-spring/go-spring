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

package StarterGrpcServer

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestRecoverUnaryInterceptor proves a handler panic surfaces as
// codes.Internal instead of crashing the process — grpc-go recovers nothing
// by itself, so without this interceptor one bad RPC kills the server.
func TestRecoverUnaryInterceptor(t *testing.T) {
	ic := RecoverUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Boom"}

	resp, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { panic("rpc boom") })
	if resp != nil {
		t.Fatalf("no response expected, got %v", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("want codes.Internal, got %v", status.Code(err))
	}

	// The clean path is untouched.
	got, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { return "ok", nil })
	if err != nil || got != "ok" {
		t.Fatalf("clean path broken: %v %v", got, err)
	}
}

// TestRecoverStreamInterceptor is the streaming twin.
func TestRecoverStreamInterceptor(t *testing.T) {
	ic := RecoverStreamInterceptor()
	info := &grpc.StreamServerInfo{FullMethod: "/demo.Service/BoomStream"}

	err := ic(nil, stubServerStream{}, info,
		func(any, grpc.ServerStream) error { panic("stream boom") })
	if status.Code(err) != codes.Internal {
		t.Fatalf("want codes.Internal, got %v", status.Code(err))
	}
}

// stubServerStream satisfies grpc.ServerStream with nothing implemented; the
// recovery interceptor only touches its context.
type stubServerStream struct{}

func (stubServerStream) SetHeader(md metadata.MD) error  { return nil }
func (stubServerStream) SendHeader(md metadata.MD) error { return nil }
func (stubServerStream) SetTrailer(md metadata.MD)       {}
func (stubServerStream) Context() context.Context        { return context.Background() }
func (stubServerStream) SendMsg(any) error               { return nil }
func (stubServerStream) RecvMsg(any) error               { return nil }
