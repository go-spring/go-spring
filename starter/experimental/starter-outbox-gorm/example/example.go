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

// example drives the transactional outbox end to end on an in-memory sqlite
// database and an in-memory "mem" driver, self-asserting the pattern's three
// core behaviors, then exits non-zero on any failure:
//
//  1. atomicity — a committed transaction delivers its message; a rolled-back
//     one delivers nothing;
//  2. retry — a failing destination is retried with backoff until it succeeds;
//  3. dead-letter — a permanently failing destination exhausts its attempts
//     and lands in the DLQ with the failure headers.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"go-spring.org/cloud/messaging"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	outboxgorm "go-spring.org/starter-outbox-gorm"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// In-memory driver: records published messages per destination; destinations
// listed in failUntil fail the first N publishes (0 = always fail), so the
// example can drive retry and dead-letter paths without a broker.
// ----------------------------------------------------------------------------

// memDriver implements [messaging.Driver] over in-process maps.
type memDriver struct {
	mu        sync.Mutex
	sent      map[string][]*messaging.Message
	attempts  map[string]int
	failUntil map[string]int
}

// demoDriver is the delivery-side messaging.Driver: an in-process fake the
// outbox relay drains into. It is provided as a messaging.Driver bean — the only
// one in this app — so the outbox starter autowires it as the delivery driver,
// the same path a broker starter's exported messaging.Driver bean rides.
var demoDriver = &memDriver{
	sent:      make(map[string][]*messaging.Message),
	attempts:  make(map[string]int),
	failUntil: map[string]int{"orders.flaky": 2, "poison": 0},
}

// delivered returns the messages published to dest so far.
func (b *memDriver) delivered(dest string) []*messaging.Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*messaging.Message(nil), b.sent[dest]...)
}

// NewPublisher implements [messaging.Driver].
func (b *memDriver) NewPublisher(ctx context.Context, destination string) (messaging.Publisher, error) {
	return &memPublisher{b: b, dest: destination}, nil
}

// NewSubscriber implements [messaging.Driver].
func (b *memDriver) NewSubscriber(ctx context.Context, source, group string) (messaging.Subscriber, error) {
	return nil, errors.New("example: subscriber not supported")
}

// memPublisher publishes to one destination of a [memDriver].
type memPublisher struct {
	b    *memDriver
	dest string
}

// Publish implements [messaging.Publisher].
func (p *memPublisher) Publish(ctx context.Context, msg *messaging.Message) error {
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	p.b.attempts[p.dest]++
	if n, ok := p.b.failUntil[p.dest]; ok && (n == 0 || p.b.attempts[p.dest] <= n) {
		return fmt.Errorf("example: simulated failure #%d on %q", p.b.attempts[p.dest], p.dest)
	}
	p.b.sent[p.dest] = append(p.b.sent[p.dest], msg)
	return nil
}

// Close implements [messaging.Publisher].
func (p *memPublisher) Close() error { return nil }

// ----------------------------------------------------------------------------
// Wiring: one sqlite database bean (autowired by the outbox starter), one
// self-asserting runner.
// ----------------------------------------------------------------------------

func init() {
	gs.Provide(newDB)
	// Provide the in-memory driver as a messaging.Driver bean so the outbox
	// starter autowires it as the delivery side (no broker needed).
	gs.Provide(func() messaging.Driver { return demoDriver })
}

// newDB opens the shared in-memory sqlite database. MaxOpenConns=1 keeps the
// in-memory data on a single connection; the outbox starter auto-migrates the
// outbox_message table (auto-migrate=true in conf).
func newDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:outbox-db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	demoDB = db
	return db, nil
}

// demoDB is the database bean instance, captured for the self-assertions.
var demoDB *gorm.DB

func main() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest()
	}()
	gs.Run()
}

// runTest publishes the scenario messages, waits for the relay to settle,
// self-asserts the three behaviors, prints OK, and SIGTERMs the app (which
// check.sh treats as success). Any deviation exits non-zero.
func runTest() {
	ctx := context.Background()
	mem := demoDriver
	db := demoDB
	fail := func(format string, args ...any) {
		log.Errorf(ctx, log.TagAppDef, format, args...)
		os.Exit(1)
	}

	// 1. Atomicity: one committed order, one rolled-back.
	if err := db.Transaction(func(tx *gorm.DB) error {
		return outboxgorm.Publish(tx, "orders", "order-1", []byte(`{"id":1}`), nil)
	}); err != nil {
		fail("publish committed record: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := outboxgorm.Publish(tx, "orders", "order-2", []byte(`{"id":2}`), nil); err != nil {
			return err
		}
		return errors.New("business rollback")
	}); err == nil {
		fail("rollback transaction unexpectedly succeeded")
	}

	// 2. Retry: "orders.flaky" fails twice, then succeeds (failUntil=2,
	// max-attempts=3 — the third attempt is its last before dead-letter).
	if err := db.Transaction(func(tx *gorm.DB) error {
		return outboxgorm.Publish(tx, "orders.flaky", "order-3", []byte(`{"id":3}`), nil)
	}); err != nil {
		fail("publish flaky record: %v", err)
	}

	// 3. Dead-letter: "poison" always fails; max-attempts=3 → DLQ.
	if err := db.Transaction(func(tx *gorm.DB) error {
		return outboxgorm.Publish(tx, "poison", "poison-1", []byte(`{"id":4}`), nil)
	}); err != nil {
		fail("publish poison record: %v", err)
	}

	// Wait for the relay to settle (poll 100ms, backoff 100ms → a few rounds).
	settled := func() bool {
		return len(mem.delivered("orders")) == 1 &&
			len(mem.delivered("orders.flaky")) == 1 &&
			len(mem.delivered("poison.dlq")) == 1
	}
	deadline := time.Now().Add(10 * time.Second)
	for !settled() {
		if time.Now().After(deadline) {
			fail("relay did not settle within 10s (orders=%d flaky=%d dlq=%d)",
				len(mem.delivered("orders")), len(mem.delivered("orders.flaky")), len(mem.delivered("poison.dlq")))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Assert delivered content and headers.
	orders := mem.delivered("orders")
	if got := string(orders[0].Payload); got != `{"id":1}` {
		fail("orders payload: %s", got)
	}
	if orders[0].Key != "order-1" {
		fail("orders key: %s", orders[0].Key)
	}
	dlq := mem.delivered("poison.dlq")
	if got := string(dlq[0].Payload); got != `{"id":4}` {
		fail("dlq payload: %s", got)
	}
	if dlq[0].Headers[messaging.HeaderDLQKey] != "poison-1" ||
		dlq[0].Headers[messaging.HeaderDLQRetries] != "3" {
		fail("dlq headers: %v", dlq[0].Headers)
	}
	if len(mem.delivered("poison")) != 0 {
		fail("poison message was delivered — dead-letter must not deliver")
	}

	// Assert final table states: 2 sent, 1 dead, 0 pending.
	for status, want := range map[string]int64{"sent": 2, "dead": 1, "pending": 0} {
		var n int64
		if err := db.Table("outbox_message").Where("status = ?", status).Count(&n).Error; err != nil {
			fail("count %s: %v", status, err)
		}
		if n != want {
			fail("outbox rows with status %q: got %d, want %d", status, n, want)
		}
	}

	fmt.Println("outbox example OK: atomicity, retry, dead-letter all passed")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
