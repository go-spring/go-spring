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

package outbox

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"go-spring.org/cloud/experimental/messaging"
	"go-spring.org/log"
)

// Relay drains a [Store] to a broker through a [messaging.Binder]. Run it on
// one goroutine per relay instance; multiple relays against the same store are
// safe as long as the Store honors its concurrency contract.
//
// Ordering: within a batch records are delivered in ID order, but no global
// ordering is promised across batches, instances or restarts. Businesses that
// need per-entity ordering should set Record.Key so the broker keeps same-key
// messages ordered.
type Relay struct {
	store  Store
	binder messaging.Binder
	cfg    Config
	obs    []Observer
	pubsMu sync.Mutex
	pubs   map[string]messaging.Publisher // destination → publisher, lazily opened
}

// NewRelay assembles a Relay over store and binder with cfg normalized to its
// defaults. Observers receive publish/retry/dead events; they are called
// synchronously from the relay loop.
func NewRelay(store Store, binder messaging.Binder, cfg Config, obs ...Observer) *Relay {
	return &Relay{
		store:  store,
		binder: binder,
		cfg:    cfg.withDefaults(),
		obs:    obs,
	}
}

// publisher returns the (lazily created) publisher for destination.
func (r *Relay) publisher(ctx context.Context, destination string) (messaging.Publisher, error) {
	r.pubsMu.Lock()
	defer r.pubsMu.Unlock()
	if p, ok := r.pubs[destination]; ok {
		return p, nil
	}
	p, err := r.binder.NewPublisher(ctx, destination)
	if err != nil {
		return nil, fmt.Errorf("outbox: open publisher for %q: %w", destination, err)
	}
	if r.pubs == nil {
		r.pubs = make(map[string]messaging.Publisher)
	}
	r.pubs[destination] = p
	return p, nil
}

// Close closes every publisher opened by the relay. Run must have returned.
func (r *Relay) Close() error {
	r.pubsMu.Lock()
	defer r.pubsMu.Unlock()
	var firstErr error
	for dest, p := range r.pubs {
		if err := p.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(r.pubs, dest)
	}
	return firstErr
}

// Run polls the store and delivers due records until ctx is cancelled. On
// cancellation it stops fetching but finishes the record in flight (or marks
// it failed for the next run), then returns ctx.Err().
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if err := r.runBatch(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// runBatch fetches one batch and delivers each record; a failing record never
// aborts the batch — one poisoned message must not starve the rest.
func (r *Relay) runBatch(ctx context.Context) error {
	recs, err := r.store.Fetch(ctx, time.Now(), r.cfg.BatchSize)
	if err != nil {
		// A failing store must not crash the process — the relay keeps
		// polling — but it must not be silent either: a dead database would
		// otherwise be indistinguishable from an idle relay.
		log.Errorf(ctx, log.TagAppDef, "outbox relay: fetch failed (keep polling): %v", err)
		return nil
	}
	for i := range recs {
		if ctx.Err() != nil {
			return nil
		}
		r.deliver(ctx, &recs[i])
	}
	return nil
}

// deliver publishes one record and moves it to sent / retry / dead.
func (r *Relay) deliver(ctx context.Context, rec *Record) {
	err := r.publish(ctx, rec)
	if err == nil {
		if markErr := r.store.MarkSent(ctx, rec.ID, time.Now()); markErr == nil {
			r.notify(func(o Observer) { o.OnPublished(rec) })
			return
		} else {
			// MarkSent failed (store down): leave the record pending — the
			// next run re-delivers it. at-least-once, by design.
			err = markErr
		}
	}
	r.fail(ctx, rec, err)
}

// publish sends the record's message to its destination.
func (r *Relay) publish(ctx context.Context, rec *Record) error {
	p, err := r.publisher(ctx, rec.Destination)
	if err != nil {
		return err
	}
	msg := &messaging.Message{
		Key:     rec.Key,
		Payload: rec.Payload,
		Headers: rec.Headers,
	}
	if err := p.Publish(ctx, msg); err != nil {
		return fmt.Errorf("outbox: publish record %d to %q: %w", rec.ID, rec.Destination, err)
	}
	return nil
}

// fail records a delivery failure: retry with backoff, or dead-letter once
// MaxAttempts is exhausted.
func (r *Relay) fail(ctx context.Context, rec *Record, err error) {
	attempts := rec.Attempts + 1
	if attempts < r.cfg.MaxAttempts {
		next := time.Now().Add(r.cfg.backoff(attempts))
		if markErr := r.store.MarkFailed(ctx, rec.ID, err, next); markErr == nil {
			r.notify(func(o Observer) { o.OnRetry(rec, err, next) })
			return
		}
		// MarkFailed failed: fall through to the dead branch only if the DLQ
		// copy succeeds; otherwise leave pending and try everything next run.
	}
	if r.cfg.DLQSuffix == "" {
		if r.store.MarkDead(ctx, rec.ID, err) == nil {
			r.notify(func(o Observer) { o.OnDead(rec, err) })
		}
		return
	}
	if dlqErr := r.publishDLQ(ctx, rec, attempts, err); dlqErr != nil {
		// Losing a dead letter is worse than redelivering: keep the record
		// pending so the next run retries the whole dead-letter path.
		_ = r.store.MarkFailed(ctx, rec.ID, dlqErr, time.Now().Add(r.cfg.backoff(attempts)))
		return
	}
	if r.store.MarkDead(ctx, rec.ID, err) == nil {
		r.notify(func(o Observer) { o.OnDead(rec, err) })
	}
}

// publishDLQ sends a copy of rec to the dead-letter destination, stamped with
// the [messaging] DLQ header contract.
func (r *Relay) publishDLQ(ctx context.Context, rec *Record, attempts int, err error) error {
	p, perr := r.publisher(ctx, rec.Destination+r.cfg.DLQSuffix)
	if perr != nil {
		return perr
	}
	msg := &messaging.Message{
		Payload:   rec.Payload,
		Headers:   make(map[string]string, len(rec.Headers)+3),
		Timestamp: time.Now(),
	}
	maps.Copy(msg.Headers, rec.Headers)
	msg.Headers[messaging.HeaderDLQError] = err.Error()
	msg.Headers[messaging.HeaderDLQRetries] = fmt.Sprintf("%d", attempts)
	msg.Headers[messaging.HeaderDLQKey] = rec.Key
	if perr = p.Publish(ctx, msg); perr != nil {
		return fmt.Errorf("outbox: dead-letter record %d to %q: %w", rec.ID, rec.Destination+r.cfg.DLQSuffix, perr)
	}
	return nil
}

// notify invokes emit on every observer.
func (r *Relay) notify(emit func(Observer)) {
	for _, o := range r.obs {
		emit(o)
	}
}
