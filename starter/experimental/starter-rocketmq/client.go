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
	"go-spring.org/cloud/chain"
	"strings"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
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
// the shared connection settings, the resilience executor [NewClient] attaches,
// and every producer/consumer created through it. Inject it and use
// NewProducer / NewPushConsumer for raw SDK access, or NewDriver for the
// broker-neutral messaging abstraction.
type Client struct {
	nameServers []string
	cfg         Config

	// exec / serviceLabel carry the resilience executor [NewClient] applies from
	// the governance bundle it is handed; exec is never nil — a hand-built client
	// degrades to an observed-only, loudly-unmanaged executor.
	exec         chain.Executor
	serviceLabel string

	mu        sync.Mutex
	closed    bool
	producers []rocketmq.Producer
	consumers []rocketmq.PushConsumer
}

// NewClient builds a complete Client — identity, name server list and governance
// executor all applied — over the given name server list. It fixes the identity
// the wrapper reapplies to every producer and consumer it creates (the name
// server list) and derives from the Config. There is no Init step — building a
// Client and initializing it are the same act, so the container has no lifecycle
// hook to register and no way to hand out a half-built client.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: the executor comes from
// params.ExecutorFor, with a hand-built client (the zero [cloud.ClientParams])
// degrading to [resilience.Unmanaged] — observed, with a one-time warning that no
// protection applies. It is the constructor the bundled [DefaultDriver] uses; a
// company Driver may call it too.
func NewClient(nameServers []string, c Config, params cloud.ClientParams) *Client {
	cl := &Client{nameServers: primitive.NamesrvAddr(nameServers), cfg: c}
	cl.serviceLabel = resilience.ServiceLabel("rocketmq", strings.Join(nameServers, ","))
	cl.exec = params.ExecutorFor("rocketmq", cl.serviceLabel)
	return cl
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

	if cl.exec != nil {
		_ = cl.exec.Close()
		cl.exec = nil
	}
	for _, p := range cl.producers {
		if err := p.Shutdown(); err != nil {
			log.Error(context.Background(), log.TagAppDef, err, log.Msg("rocketmq: shutdown producer failed"))
		}
	}
	for _, c := range cl.consumers {
		if err := c.Shutdown(); err != nil {
			log.Error(context.Background(), log.TagAppDef, err, log.Msg("rocketmq: shutdown consumer failed"))
		}
	}
	cl.producers = nil
	cl.consumers = nil
	return nil
}

// GuardedProducer returns the chain producer's synchronous sends run through:
// identity over governance over the raw adapter, governance from this client's
// executor. Wrap the returned head in your own [InnerProducer] layer to modify
// what a send does — the topics a layer rewrites are what gets declared.
func (cl *Client) GuardedProducer(p rocketmq.Producer) InnerProducer {
	return NewObsProducer(NewGuardProducer(NewRawProducer(p), cl.exec))
}

// execute routes call through the client's resilience executor, and otherwise
// runs it inline. It serves the per-delivery consume path (whose pipeline
// inverts the chain's composition, like starter-nats's Consume) and a bare
// Client (tests, or a caller assembling one by hand).
func (cl *Client) execute(ctx context.Context, call func(context.Context) error) error {
	if cl.exec == nil {
		return call(ctx)
	}
	return cl.exec.Execute(ctx, call)
}
