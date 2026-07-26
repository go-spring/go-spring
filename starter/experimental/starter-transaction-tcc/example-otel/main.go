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

// Package main is the observability example for starter-transaction-tcc.
//
// It runs TCC commit and rollback scenarios through an in-process coordinator,
// verifies the coordinator spans reach Jaeger, then self-exits. Traces are
// exported via OTLP/gRPC to Jaeger (docker-compose).
//
// Run with -manual to keep the server running for interactive exploration.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/experimental/cloud/transaction/tcc"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"
	_ "go-spring.org/starter-transaction-tcc"
)

// ----------------------------------------------------------------------------
// Two toy TCC resources: a stock ledger and an account balance. Each keeps an
// `available` pool and a per-transaction `frozen` reservation. Try moves value
// into frozen (reserved but not yet business-visible as spent); Confirm drops
// the frozen amount (the spend is final); Cancel returns it to available. This
// is the reserve/commit/release shape TCC exists for — the reserved value is
// never seen as committed by other readers between Try and Confirm.
// ----------------------------------------------------------------------------

type ledger struct {
	mu        sync.Mutex
	available int
	frozen    map[string]int // keyed by transaction id, so Cancel is idempotent
}

func newLedger(initial int) *ledger { return &ledger{available: initial, frozen: map[string]int{}} }

// try reserves n, failing if there is not enough available. Keying by txID makes
// a repeated try a no-op (anti-hanging) rather than a double reservation.
func (l *ledger) try(txID string, n int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, done := l.frozen[txID]; done {
		return nil
	}
	if l.available < n {
		return errors.New("insufficient")
	}
	l.available -= n
	l.frozen[txID] = n
	return nil
}

// confirm finalizes the reservation: the frozen amount is simply dropped.
// Idempotent — a missing reservation means it was already confirmed.
func (l *ledger) confirm(txID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.frozen, txID)
	return nil
}

// cancel returns the reserved amount to available. Idempotent and safe as an
// empty rollback: a missing reservation (Try never ran or already cancelled)
// does nothing.
func (l *ledger) cancel(txID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n, ok := l.frozen[txID]; ok {
		l.available += n
		delete(l.frozen, txID)
	}
	return nil
}

func (l *ledger) snapshot() (available, frozen int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, n := range l.frozen {
		frozen += n
	}
	return l.available, frozen
}

// ----------------------------------------------------------------------------
// Order service: places an order by reserving stock and balance in one TCC
// transaction driven by the coordinator the starter contributes.
// ----------------------------------------------------------------------------

type OrderService struct {
	Coord tcc.Coordinator `autowire:""`

	stock   *ledger
	balance *ledger
}

func newOrderService() *OrderService {
	return &OrderService{stock: newLedger(10), balance: newLedger(100)}
}

// place runs a two-participant TCC transaction: reserve `qty` stock and freeze
// `cost` balance. If either Try fails, the coordinator cancels whatever was
// tried, leaving both ledgers untouched.
func (s *OrderService) place(ctx context.Context, txID string, qty, cost int) (tcc.Result, error) {
	return s.Coord.Execute(ctx, tcc.Transaction{
		ID: txID,
		Participants: []tcc.Participant{
			{
				Name:    "ReserveStock",
				Try:     func(context.Context) (any, error) { return nil, s.stock.try(txID, qty) },
				Confirm: func(context.Context, any) error { return s.stock.confirm(txID) },
				Cancel:  func(context.Context, any) error { return s.stock.cancel(txID) },
			},
			{
				Name:    "FreezeBalance",
				Try:     func(context.Context) (any, error) { return nil, s.balance.try(txID, cost) },
				Confirm: func(context.Context, any) error { return s.balance.confirm(txID) },
				Cancel:  func(context.Context, any) error { return s.balance.cancel(txID) },
			},
		},
	})
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	svc := newOrderService()
	svcBean := gs.Provide(svc).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svcBean.Interface().(*OrderService))
		}()
	} else {

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest exercises the commit and rollback paths and asserts the ledgers end in
// the states the TCC protocol guarantees, then signals shutdown. Any deviation
// exits non-zero so check.sh fails.
func runTest(s *OrderService) {
	ctx := context.Background()

	// Path 1 — commit: 3 stock + 60 balance both fit, so both confirm. The
	// reserved amounts are finalized: available drops, nothing stays frozen.
	res, err := s.place(ctx, "order-commit", 3, 60)
	if err != nil || res.Status != tcc.StatusCommitted {
		log.Errorf(ctx, log.TagAppDef, "commit path: status=%s err=%v", res.Status, err)
		os.Exit(1)
	}
	if a, f := s.stock.snapshot(); a != 7 || f != 0 {
		log.Errorf(ctx, log.TagAppDef, "commit path stock: available=%d frozen=%d, want 7/0", a, f)
		os.Exit(1)
	}
	if a, f := s.balance.snapshot(); a != 40 || f != 0 {
		log.Errorf(ctx, log.TagAppDef, "commit path balance: available=%d frozen=%d, want 40/0", a, f)
		os.Exit(1)
	}
	fmt.Println("commit path OK:", res.Status)

	// Path 2 — rollback: 2 stock fits but 999 balance does not, so ReserveStock's
	// Try succeeds, FreezeBalance's Try fails, and the coordinator cancels the
	// stock reservation. Both ledgers must be exactly as they were after path 1.
	res, err = s.place(ctx, "order-rollback", 2, 999)
	if err == nil || res.Status != tcc.StatusCancelled {
		log.Errorf(ctx, log.TagAppDef, "rollback path: status=%s err=%v (want Cancelled + error)", res.Status, err)
		os.Exit(1)
	}
	if a, f := s.stock.snapshot(); a != 7 || f != 0 {
		log.Errorf(ctx, log.TagAppDef, "rollback path stock: available=%d frozen=%d, want 7/0 (reservation released)", a, f)
		os.Exit(1)
	}
	if a, f := s.balance.snapshot(); a != 40 || f != 0 {
		log.Errorf(ctx, log.TagAppDef, "rollback path balance: available=%d frozen=%d, want 40/0", a, f)
		os.Exit(1)
	}
	fmt.Println("rollback path OK:", res.Status, "-", res.Errors[0].Error())

	// Wait for collector to flush, then verify traces appear in Jaeger.
	time.Sleep(3 * time.Second)

	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=transaction-tcc-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'transaction-tcc-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'transaction-tcc-otel-example'")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where this
// source file resides, so relative config paths resolve regardless of the launch
// path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
