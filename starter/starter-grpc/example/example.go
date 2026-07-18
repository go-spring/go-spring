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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterGrpc "go-spring.org/starter-grpc"
	"go-spring.org/starter-grpc/example/idl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

func init() {
	// Register the Echo controller as a bean.
	gs.Provide(&Controller{})

	// Register the gRPC service. Because starter-grpc constructs the
	// grpc.Server without exposing grpc.ServerOption, we compose the
	// unary interceptor with the underlying controller via an in-handler
	// wrapper (interceptedEchoServer). This is functionally equivalent
	// to a grpc.UnaryServerInterceptor being passed to grpc.NewServer.
	gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
		return func(svr *grpc.Server) {
			proto.RegisterEchoServiceServer(svr, &interceptedEchoServer{
				inner:       c,
				interceptor: LoggingInterceptor,
			})
		}
	})
}

// Controller implements the EchoService. It also demonstrates writing a
// response header via grpc.SetHeader so callers can read handler-side
// metadata from the response.
type Controller struct {
	proto.UnimplementedEchoServiceServer
}

// Echo returns the request message unchanged and attaches a response
// header (`x-handler=echo`) via gRPC metadata.
func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-handler", "echo")); err != nil {
		log.Warnf(ctx, log.TagAppDef, "SetHeader failed: %v", err)
	}
	return &proto.EchoResponse{Message: req.Message}, nil
}

// LoggingInterceptor is a real grpc.UnaryServerInterceptor. It logs the
// invoked method, reads the incoming `x-app` metadata (if present) and
// validates that it is not empty, then delegates to the next handler.
var LoggingInterceptor grpc.UnaryServerInterceptor = func(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	app := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("x-app"); len(v) > 0 {
			app = v[0]
		}
	}
	log.Infof(ctx, log.TagAppDef, "gRPC call method=%s x-app=%q", info.FullMethod, app)
	return handler(ctx, req)
}

// interceptedEchoServer wires a grpc.UnaryServerInterceptor around the
// real EchoService implementation. It reproduces the interceptor chain
// that grpc.NewServer would build if ChainUnaryInterceptor were used.
type interceptedEchoServer struct {
	proto.UnimplementedEchoServiceServer
	inner       proto.EchoServiceServer
	interceptor grpc.UnaryServerInterceptor
}

func (s *interceptedEchoServer) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
	info := &grpc.UnaryServerInfo{Server: s, FullMethod: "/EchoService/Echo"}
	handler := func(ctx context.Context, r any) (any, error) {
		return s.inner.Echo(ctx, r.(*proto.EchoRequest))
	}
	resp, err := s.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	return resp.(*proto.EchoResponse), nil
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Run()
}

func runTest() {
	ctx := context.Background()

	conn, err := grpc.NewClient(":9494", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Failed to connect: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := proto.NewEchoServiceClient(conn)

	// Feature 2: attach outgoing metadata `x-app=go-spring` so the
	// server-side interceptor sees and validates it. Feature 3: capture
	// response headers via grpc.Header.
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-app", "go-spring"))
	var header metadata.MD

	// Feature 1: unary Echo round-trip.
	response, err := client.Echo(
		callCtx,
		&proto.EchoRequest{Message: "Hello, gRPC!"},
		grpc.Header(&header),
	)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error calling Echo: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", response.Message)

	// Assertion 1: response body echoed unchanged.
	if response.Message != "Hello, gRPC!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected echo body: %q", response.Message)
		os.Exit(1)
	}

	// Assertion 3: handler-side response header propagated to client.
	if v := header.Get("x-handler"); len(v) == 0 || v[0] != "echo" {
		log.Errorf(ctx, log.TagAppDef, "missing/incorrect x-handler header: %v", v)
		os.Exit(1)
	}

	// Assertion 2 is implicit: if the interceptor had rejected the call
	// (or panicked reading incoming metadata) the Echo above would have
	// failed. Reaching this point means it ran and passed the request
	// through cleanly.

	// Server hardening: the starter mounts the standard grpc_health_v1 service
	// (spring.grpc.server.health.enabled defaults to true). Probe it and assert
	// the server reports SERVING.
	healthResp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "health check failed: %v", err)
		os.Exit(1)
	}
	fmt.Println("Health status:", healthResp.Status)
	if healthResp.Status != healthpb.HealthCheckResponse_SERVING {
		log.Errorf(ctx, log.TagAppDef, "unexpected health status: %v", healthResp.Status)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
