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

// entity.go is the "resource entity" concept of this starter: the Client
// wrapper Kafka clients are injected as, hollowed into an InnerKafka chain —
// the identity layer declares each synchronous produce, the governance layer
// runs it under the resilience executor, the adapter layer makes the wire
// call. This wrapper replaces the earlier per-client guard registry: the chain
// travels with the client, not a package-global map.
package StarterKafka

import (
	"context"
	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud"
	"go-spring.org/log"
)

// Client wraps a franz-go client. It holds exactly two exported things: the
// embedded [InnerKafka] chain head the synchronous produces run through —
// reorganized by wrapping the head in a layer of your own — and
// [Client.Kafka], the raw client as a read-only handle (the poll loop, async
// Produce, admin features — franz-go's async paths are intentionally outside
// the chain; see command.go). [NewClient] is the only way to build one.
type Client struct {
	// The embedded InnerKafka is the chain ProduceSync runs through: identity
	// over governance over the raw adapter by default, so the default path is
	// always declared and protected. Reorganize it by wrapping the head (see
	// [InnerKafka]); build-time only.
	InnerKafka

	// Kafka is the raw franz-go client — the original object, not a wrapper. A
	// read-only handle for the API surface outside the chain (PollFetches,
	// async Produce, admin); producing synchronously through it bypasses the
	// chain.
	Kafka *kgo.Client

	// guard is the chain's governance layer, held for the per-record consume
	// path (whose per-delivery pipeline cannot ride the chain — see
	// [Client.execute]). Unaffected by head reorganization.
	guard *GuardKafka
}

// NewClient builds a complete Client — identity, governance and all — over a
// raw franz-go client. brokers is the client's identity; it is what the
// governance label scopes limiter/breaker state by. params carries the
// container's facilities (see [cloud.ClientParams]); the zero bundle degrades
// to the observed-only, loudly-unmanaged executor.
func NewClient(cl *kgo.Client, brokers string, params cloud.ClientParams) *Client {
	guard := NewGuardKafka(NewRawKafka(cl), params.ExecutorFor("kafka", serviceLabelOf(brokers)))
	return &Client{Kafka: cl, InnerKafka: NewObsKafka(guard), guard: guard}
}

// Close tears the client down: the governance layer's executor goes first,
// then any buffered produce records are flushed (so in-flight messages are not
// dropped on shutdown) and the client closes. It is the gs destroy method.
func (c *Client) Close() error {
	if c.guard != nil && c.guard.exec != nil {
		_ = c.guard.exec.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	flushErr := c.Kafka.Flush(ctx)
	c.Kafka.Close()
	if flushErr != nil {
		// A failed flush means buffered produce records were never delivered:
		// those messages are lost, so shutdown must not swallow the error.
		log.Errorf(context.Background(), log.TagAppDef,
			"kafka: flush before close failed, buffered messages may be LOST: %v", flushErr)
		return flushErr
	}
	return nil
}

// execute routes a per-record consume call through the governance layer's
// executor, and otherwise runs it inline. The consume pipeline (declare, run
// under the executor, once per record) is driven by whoever owns the poll
// loop, so it uses this seam instead of the chain.
func (c *Client) execute(ctx context.Context, call func(context.Context) error) error {
	if c.guard == nil || c.guard.exec == nil {
		return call(ctx)
	}
	return runGuarded(ctx, c.guard.exec, call)
}
