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
	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	observability "go-spring.org/cloud/governance/dataplane/outbound/observe"
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

	// obs is the client's observer, held for the per-record consume path (whose
	// per-delivery pipeline cannot ride the chain — see [Client.observe]).
	// Unaffected by head reorganization. A kafka client has no seam for the
	// resilience executor, so this is observation only — no governance.
	obs *observability.OperationObserver

	// prop is the process traffic convention, taken from the runtime when the
	// client is built; each operation records its traffic class.
	prop traffic.Propagator
}

// NewClient builds a complete Client — emitting layer and all — over a raw
// franz-go client. brokers is the client's identity; it is what the observer's
// service label scopes its signals by. A kafka client has no seam for the
// resilience executor, so there is no governance here: the observer emits the
// produce/consume span, metrics and access log, and nothing throttles or
// retries.
func NewClient(cl *kgo.Client, brokers string, prop traffic.Propagator) *Client {
	obs := observability.NewOperationObserver("kafka", serviceLabelOf(brokers))
	return &Client{Kafka: cl, InnerKafka: NewObsKafka(NewRawKafka(cl), obs, prop), obs: obs, prop: prop}
}

// Close tears the client down: any buffered produce records are flushed (so
// in-flight messages are not dropped on shutdown) and the client closes. It is
// the gs destroy method.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	flushErr := c.Kafka.Flush(ctx)
	c.Kafka.Close()
	if flushErr != nil {
		// A failed flush means buffered produce records were never delivered:
		// those messages are lost, so shutdown must not swallow the error.
		log.Error(context.Background(), log.TagAppDef, flushErr, "kafka: flush before close failed, buffered messages may be LOST")
		return flushErr
	}
	return nil
}

// observe runs one per-record consume call under the client's observer. The
// consume pipeline (name, run observed, once per record) is driven by whoever
// owns the poll loop, so it uses this seam instead of the chain.
func (c *Client) observe(ctx context.Context, op chain.Operation, call func(context.Context) error) error {
	if c.obs == nil {
		return call(ctx)
	}
	return c.obs.Observe(ctx, op, nil, call)
}
