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

// Command example-lb is a self-contained smoke test for the gRPC client-side
// load-balancing adapter (starter-grpc/balancer.go) built on
// go-spring.org/spring/loadbalance. It needs no external services or docker: it
// starts three in-process Echo backends, adapts them through a static discovery
// backend, and drives one gRPC client that dials "gsdiscovery:///echo".
//
// It asserts the three acceptance behaviours end to end:
//
//  1. Even distribution — round-robin spreads requests evenly across instances.
//  2. Breaker eviction + recovery — a backend that starts failing is ejected
//     within a few requests (traffic stops reaching it), then readmitted after
//     it recovers and the cool-down elapses.
//  3. Kill an instance — removing a backend from discovery and stopping it makes
//     traffic drop it within seconds, with no request errors.
//
// The process exits 0 only if every assertion holds.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/loadbalance"
	StarterGrpc "go-spring.org/starter-grpc"
	"go-spring.org/starter-grpc/example/idl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// smokeBalancer is a round-robin balancer with a short-fused ejection tracker so
// the breaker phases finish quickly (3 failures evict, 2s cool-down).
const smokeBalancer = "gs_smoke"

func fatalf(format string, args ...any) {
	fmt.Printf("FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Echo backend
// ---------------------------------------------------------------------------

type echoServer struct {
	proto.UnimplementedEchoServiceServer
	id   string
	fail atomic.Bool
}

func (s *echoServer) Echo(_ context.Context, _ *proto.EchoRequest) (*proto.EchoResponse, error) {
	if s.fail.Load() {
		return nil, status.Errorf(codes.Unavailable, "backend %s is failing", s.id)
	}
	// The response carries the backend id so the client can tally which
	// instance served each request.
	return &proto.EchoResponse{Message: s.id}, nil
}

type backend struct {
	id   string
	addr string
	h    *echoServer
	srv  *grpc.Server
}

func startBackend(id string) (*backend, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	h := &echoServer{id: id}
	srv := grpc.NewServer()
	proto.RegisterEchoServiceServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	return &backend{id: id, addr: lis.Addr().String(), h: h, srv: srv}, nil
}

// ---------------------------------------------------------------------------
// Dynamic in-memory discovery backend
// ---------------------------------------------------------------------------

type disco struct {
	mu  sync.Mutex
	eps []discovery.Endpoint
	ws  map[*watcher]struct{}
}

func newDisco(eps []discovery.Endpoint) *disco {
	return &disco{eps: eps, ws: map[*watcher]struct{}{}}
}

func (d *disco) set(eps []discovery.Endpoint) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.eps = eps
	for w := range d.ws {
		w.notify(eps)
	}
}

func (d *disco) Resolve(_ context.Context, _ string) ([]discovery.Endpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]discovery.Endpoint(nil), d.eps...), nil
}

func (d *disco) Watch(_ context.Context, _ string) (discovery.Watcher, error) {
	w := &watcher{d: d, ch: make(chan []discovery.Endpoint, 1), done: make(chan struct{})}
	d.mu.Lock()
	d.ws[w] = struct{}{}
	d.mu.Unlock()
	return w, nil
}

type watcher struct {
	d    *disco
	ch   chan []discovery.Endpoint
	done chan struct{}
	once sync.Once
}

func (w *watcher) notify(eps []discovery.Endpoint) {
	snap := append([]discovery.Endpoint(nil), eps...)
	// Coalesce: keep only the latest snapshot in the 1-slot channel.
	select {
	case <-w.ch:
	default:
	}
	select {
	case w.ch <- snap:
	default:
	}
}

func (w *watcher) Next() ([]discovery.Endpoint, error) {
	select {
	case eps := <-w.ch:
		return eps, nil
	case <-w.done:
		return nil, fmt.Errorf("watcher stopped")
	}
}

func (w *watcher) Stop() error {
	w.once.Do(func() {
		w.d.mu.Lock()
		delete(w.d.ws, w)
		w.d.mu.Unlock()
		close(w.done)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Client driver
// ---------------------------------------------------------------------------

// send fires n unary Echoes and tallies successful responses by backend id,
// returning the tally and the number of failed calls.
func send(client proto.EchoServiceClient, n int) (map[string]int, int) {
	hits := map[string]int{}
	errs := 0
	for range n {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.Echo(ctx, &proto.EchoRequest{Message: "hi"})
		cancel()
		if err != nil {
			errs++
			continue
		}
		hits[resp.Message]++
	}
	return hits, errs
}

// waitReady sends probe requests until responses have been seen from `want`
// distinct backends (all SubConns established) or the deadline passes.
func waitReady(client proto.EchoServiceClient, want int) {
	seen := map[string]bool{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		resp, err := client.Echo(ctx, &proto.EchoRequest{Message: "warmup"})
		cancel()
		if err == nil {
			seen[resp.Message] = true
		}
		if len(seen) >= want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	fatalf("only %d/%d backends became reachable during warmup", len(seen), want)
}

func main() {
	// Start three Echo backends on ephemeral ports.
	backends := map[string]*backend{}
	var eps []discovery.Endpoint
	for _, id := range []string{"A", "B", "C"} {
		b, err := startBackend(id)
		if err != nil {
			fatalf("start backend %s: %v", id, err)
		}
		backends[id] = b
		eps = append(eps, discovery.Endpoint{Addr: b.addr, Healthy: true})
	}

	// Register the backends under a discovery name and a short-fused balancer.
	d := newDisco(eps)
	discovery.Register("default", d)
	StarterGrpc.RegisterBalancer(smokeBalancer, loadbalance.RoundRobin,
		loadbalance.TrackerConfig{Threshold: 3, EjectFor: 2 * time.Second})

	conn, err := grpc.NewClient(
		StarterGrpc.Scheme+":///echo",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingConfig":[{%q:{}}]}`, smokeBalancer)),
	)
	if err != nil {
		fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := proto.NewEchoServiceClient(conn)

	waitReady(client, 3)

	// -----------------------------------------------------------------
	// Phase 1: even round-robin distribution across three instances.
	// -----------------------------------------------------------------
	hits, errs := send(client, 30)
	if errs != 0 {
		fatalf("phase 1: unexpected errors %d", errs)
	}
	for _, id := range []string{"A", "B", "C"} {
		if hits[id] < 8 || hits[id] > 12 {
			fatalf("phase 1: uneven distribution: %v", hits)
		}
	}
	fmt.Printf("phase 1 OK: even distribution %v\n", hits)

	// -----------------------------------------------------------------
	// Phase 2: a backend that starts failing is evicted; then recovers.
	// -----------------------------------------------------------------
	backends["A"].h.fail.Store(true)

	// Warm-up burst: A returns errors until three consecutive failures eject it.
	_, errs = send(client, 20)
	if errs == 0 {
		fatalf("phase 2: expected some failures from A before eviction")
	}
	// Once evicted, a clean burst must not touch A at all — zero errors.
	hits, errs = send(client, 15)
	if errs != 0 || hits["A"] != 0 {
		fatalf("phase 2: A not evicted (hits=%v errs=%d)", hits, errs)
	}
	fmt.Printf("phase 2a OK: failing A evicted, traffic on %v\n", hits)

	// Recover A and wait past the ejection cool-down; the half-open trial should
	// readmit it.
	backends["A"].h.fail.Store(false)
	time.Sleep(2500 * time.Millisecond)
	hits, errs = send(client, 30)
	if errs != 0 {
		fatalf("phase 2b: unexpected errors after recovery: %d", errs)
	}
	if hits["A"] == 0 {
		fatalf("phase 2b: recovered A not readmitted (hits=%v)", hits)
	}
	fmt.Printf("phase 2b OK: recovered A readmitted %v\n", hits)

	// -----------------------------------------------------------------
	// Phase 3: kill an instance — drop B from discovery and stop it.
	// -----------------------------------------------------------------
	d.set([]discovery.Endpoint{
		{Addr: backends["A"].addr, Healthy: true},
		{Addr: backends["C"].addr, Healthy: true},
	})
	backends["B"].srv.Stop()
	time.Sleep(2 * time.Second) // let the resolver update and the SubConn drop

	hits, errs = send(client, 20)
	if errs != 0 {
		fatalf("phase 3: unexpected errors after kill: %d", errs)
	}
	if hits["B"] != 0 {
		fatalf("phase 3: killed B still receiving traffic (hits=%v)", hits)
	}
	if hits["A"] == 0 || hits["C"] == 0 {
		fatalf("phase 3: surviving instances not both used (hits=%v)", hits)
	}
	fmt.Printf("phase 3 OK: killed B dropped, traffic on %v\n", hits)

	fmt.Println("all load-balancing smoke assertions passed")
}
