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

// client.go is the "resource entity + lifecycle" concept of this starter. The
// bean is a Client wrapper rather than a raw SDK object because
// rocketmq-client-go has no unified client entity: producers and consumers
// are constructed independently, each repeating the name server list,
// credentials and instance name. The wrapper holds those common options once,
// reapplies them to everything it creates, and registers every producer and
// consumer so Close can shut them all down in one place.
package StarterRocketmq

import (
	"context"
	"errors"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
)

// errClosedClient is returned by NewProducer / NewPushConsumer after Close.
var errClosedClient = errors.New("rocketmq client is closed")

// credentials builds the SDK credential pair from Config; it is only called
// when AccessKey is set (AccessKey and SecretKey are validated in pairs).
func credentials(c Config) primitive.Credentials {
	return primitive.Credentials{AccessKey: c.AccessKey, SecretKey: c.SecretKey}
}

// Client is the RocketMQ resource entity managed by the container: it holds
// the shared connection settings, the resilience executor attached by the
// starter, and every producer/consumer created through it. Inject it and use
// NewProducer / NewPushConsumer for raw SDK access, or NewDriver for the
// broker-neutral messaging abstraction.
type Client struct {
	nameServers []string
	cfg         Config

	// exec / resource carry the resilience executor attached in newClient;
	// exec is nil (transparent pass-through) when governance is off.
	exec     resilience.Executor
	resource string

	mu        sync.Mutex
	closed    bool
	producers []rocketmq.Producer
	consumers []rocketmq.PushConsumer
}

// producerOptions returns the base producer options derived from Config,
// followed by the caller's overrides.
func (cl *Client) producerOptions(extra []producer.Option) []producer.Option {
	opts := []producer.Option{
		producer.WithNameServer(cl.nameServers),
		producer.WithSendMsgTimeout(cl.cfg.SendTimeout),
		producer.WithRetry(cl.cfg.Retry),
	}
	if cl.cfg.InstanceName != "" {
		opts = append(opts, producer.WithInstanceName(cl.cfg.InstanceName))
	}
	if cl.cfg.AccessKey != "" {
		opts = append(opts, producer.WithCredentials(credentials(cl.cfg)))
	}
	return append(opts, extra...)
}

// consumerOptions returns the base consumer options derived from Config,
// followed by the caller's overrides.
func (cl *Client) consumerOptions(extra []consumer.Option) []consumer.Option {
	opts := []consumer.Option{
		consumer.WithNameServer(cl.nameServers),
	}
	if cl.cfg.InstanceName != "" {
		opts = append(opts, consumer.WithInstance(cl.cfg.InstanceName))
	}
	if cl.cfg.AccessKey != "" {
		opts = append(opts, consumer.WithCredentials(credentials(cl.cfg)))
	}
	return append(opts, extra...)
}

// NewProducer creates a started rocketmq.Producer with the client's common
// options applied. opts (e.g., producer.WithGroupName) are appended after the
// common ones so they win on conflict. The producer is registered on the
// client and shut down by Close; shutting it down earlier yourself is fine.
func (cl *Client) NewProducer(opts ...producer.Option) (rocketmq.Producer, error) {
	cl.mu.Lock()
	if cl.closed {
		cl.mu.Unlock()
		return nil, errClosedClient
	}
	cl.mu.Unlock()

	p, err := rocketmq.NewProducer(cl.producerOptions(opts)...)
	if err != nil {
		return nil, err
	}
	if err = p.Start(); err != nil {
		return nil, err
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.closed { // closed concurrently while we were starting
		_ = p.Shutdown()
		return nil, errClosedClient
	}
	cl.producers = append(cl.producers, p)
	return p, nil
}

// NewPushConsumer creates a rocketmq.PushConsumer with the client's common
// options applied. The consumer is returned unstarted: call Subscribe on it
// and then Start, in that order (the SDK's documented usage). Most
// applications should prefer NewDriver, which performs the whole
// subscribe-and-start dance for a messaging.Handler. The consumer is
// registered on the client and shut down by Close.
func (cl *Client) NewPushConsumer(opts ...consumer.Option) (rocketmq.PushConsumer, error) {
	cl.mu.Lock()
	if cl.closed {
		cl.mu.Unlock()
		return nil, errClosedClient
	}
	cl.mu.Unlock()

	c, err := rocketmq.NewPushConsumer(cl.consumerOptions(opts)...)
	if err != nil {
		return nil, err
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.closed {
		_ = c.Shutdown()
		return nil, errClosedClient
	}
	cl.consumers = append(cl.consumers, c)
	return c, nil
}

// Close shuts down every producer and consumer created through the client and
// releases the resilience executor. It is the bean destroy method; calling it
// twice is a no-op.
func (cl *Client) Close() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.closed {
		return nil
	}
	cl.closed = true

	closeResilience(cl)
	for _, p := range cl.producers {
		if err := p.Shutdown(); err != nil {
			log.Errorf(context.Background(), log.TagAppDef, "rocketmq: shutdown producer failed: %v", err)
		}
	}
	for _, c := range cl.consumers {
		if err := c.Shutdown(); err != nil {
			log.Errorf(context.Background(), log.TagAppDef, "rocketmq: shutdown consumer failed: %v", err)
		}
	}
	cl.producers = nil
	cl.consumers = nil
	return nil
}

// execute routes call through the client's resilience executor when one is
// attached, and otherwise runs it inline.
func (cl *Client) execute(ctx context.Context, call func(context.Context) error) error {
	if cl.exec == nil {
		return call(ctx)
	}
	return cl.exec.Execute(ctx, cl.resource, call)
}
