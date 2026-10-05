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

// delegate.go re-exposes the raw *nats.Conn methods on Conn. The raw connection
// is a plain field (client.go), so nothing is promoted: every method the
// raw *nats.Conn exposed would otherwise disappear from the injected bean. These
// are plain pass-throughs on purpose — the instrumented surface is the small set
// in command.go (PublishMsg/PublishMsgContext/Consume, plus the opt-in
// PublishGuarded/RequestGuarded); everything here keeps its original,
// unobserved behaviour so existing callers see no change.
//
// PublishMsg is the one raw method NOT delegated here: command.go overwrites it
// with the instrumented version. JetStream is not delegated either — the field
// of the same name on Conn shadows it (as it did before, when Conn embedded the
// raw connection and the field shadowed the promoted method).

package StarterNats

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/nats-io/nats.go"
)

// Publish publishes data on subj. It goes straight to the wire: only
// PublishMsg/PublishMsgContext (command.go) carry the declaration layer.
func (c *Conn) Publish(subj string, data []byte) error { return c.Conn.Publish(subj, data) }

// PublishRequest publishes a request on subj with a reply subject; the caller
// subscribes to reply itself.
func (c *Conn) PublishRequest(subj, reply string, data []byte) error {
	return c.Conn.PublishRequest(subj, reply, data)
}

// Request sends a request on subj and waits up to timeout for a reply.
func (c *Conn) Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return c.Conn.Request(subj, data, timeout)
}

// RequestMsg sends msg as a request and waits up to timeout for a reply.
func (c *Conn) RequestMsg(msg *nats.Msg, timeout time.Duration) (*nats.Msg, error) {
	return c.Conn.RequestMsg(msg, timeout)
}

// RequestWithContext sends a request on subj and waits for a reply, bounded by
// ctx rather than a timeout duration.
func (c *Conn) RequestWithContext(ctx context.Context, subj string, data []byte) (*nats.Msg, error) {
	return c.Conn.RequestWithContext(ctx, subj, data)
}

// RequestMsgWithContext sends msg as a request, bounded by ctx.
func (c *Conn) RequestMsgWithContext(ctx context.Context, msg *nats.Msg) (*nats.Msg, error) {
	return c.Conn.RequestMsgWithContext(ctx, msg)
}

// NewInbox returns a new unique inbox subject.
func (c *Conn) NewInbox() string { return c.Conn.NewInbox() }

// NewRespInbox returns a new inbox subject suitable for replies.
func (c *Conn) NewRespInbox() string { return c.Conn.NewRespInbox() }

// Subscribe subscribes to subj with an async message handler. The instrumented
// subscribe path is [Conn.Consume] (command.go), which declares the consume's
// operation for the executor to emit.
func (c *Conn) Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return c.Conn.Subscribe(subj, cb)
}

// ChanSubscribe subscribes to subj, delivering messages on ch.
func (c *Conn) ChanSubscribe(subj string, ch chan *nats.Msg) (*nats.Subscription, error) {
	return c.Conn.ChanSubscribe(subj, ch)
}

// ChanQueueSubscribe subscribes to subj as a member of group, delivering on ch.
func (c *Conn) ChanQueueSubscribe(subj, group string, ch chan *nats.Msg) (*nats.Subscription, error) {
	return c.Conn.ChanQueueSubscribe(subj, group, ch)
}

// SubscribeSync subscribes to subj with a synchronous subscription.
func (c *Conn) SubscribeSync(subj string) (*nats.Subscription, error) {
	return c.Conn.SubscribeSync(subj)
}

// QueueSubscribe subscribes to subj as a member of queue, so only one member of
// the group receives each message.
func (c *Conn) QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return c.Conn.QueueSubscribe(subj, queue, cb)
}

// QueueSubscribeSync subscribes to subj as a member of queue, synchronously.
func (c *Conn) QueueSubscribeSync(subj, queue string) (*nats.Subscription, error) {
	return c.Conn.QueueSubscribeSync(subj, queue)
}

// QueueSubscribeSyncWithChan subscribes to subj as a member of queue, delivering
// messages on ch.
func (c *Conn) QueueSubscribeSyncWithChan(subj, queue string, ch chan *nats.Msg) (*nats.Subscription, error) {
	return c.Conn.QueueSubscribeSyncWithChan(subj, queue, ch)
}

// NumSubscriptions returns the number of active subscriptions.
func (c *Conn) NumSubscriptions() int { return c.Conn.NumSubscriptions() }

// FlushTimeout flushes the connection's outbound buffer, bounded by timeout.
func (c *Conn) FlushTimeout(timeout time.Duration) error { return c.Conn.FlushTimeout(timeout) }

// Flush flushes the connection's outbound buffer.
func (c *Conn) Flush() error { return c.Conn.Flush() }

// FlushWithContext flushes the connection's outbound buffer, bounded by ctx.
func (c *Conn) FlushWithContext(ctx context.Context) error { return c.Conn.FlushWithContext(ctx) }

// RTT measures the round-trip time to the server.
func (c *Conn) RTT() (time.Duration, error) { return c.Conn.RTT() }

// Buffered returns the number of bytes currently buffered for sending.
func (c *Conn) Buffered() (int, error) { return c.Conn.Buffered() }

// IsClosed reports whether the connection is closed.
func (c *Conn) IsClosed() bool { return c.Conn.IsClosed() }

// IsReconnecting reports whether the connection is currently reconnecting.
func (c *Conn) IsReconnecting() bool { return c.Conn.IsReconnecting() }

// IsConnected reports whether the connection is currently established.
func (c *Conn) IsConnected() bool { return c.Conn.IsConnected() }

// Drain drains the connection, letting in-flight subscriptions finish before the
// socket is closed. It is what [Conn.Close] runs, at the chain's tail.
func (c *Conn) Drain() error { return c.Conn.Drain() }

// IsDraining reports whether the connection is draining.
func (c *Conn) IsDraining() bool { return c.Conn.IsDraining() }

// Servers returns the configured server URLs.
func (c *Conn) Servers() []string { return c.Conn.Servers() }

// DiscoveredServers returns the servers discovered from the cluster.
func (c *Conn) DiscoveredServers() []string { return c.Conn.DiscoveredServers() }

// Status returns the connection's current status.
func (c *Conn) Status() nats.Status { return c.Conn.Status() }

// Stats returns the connection's statistics.
func (c *Conn) Stats() nats.Statistics { return c.Conn.Stats() }

// MaxPayload returns the maximum payload size the server accepts.
func (c *Conn) MaxPayload() int64 { return c.Conn.MaxPayload() }

// HeadersSupported reports whether the server supports message headers.
func (c *Conn) HeadersSupported() bool { return c.Conn.HeadersSupported() }

// AuthRequired reports whether the server requires authentication.
func (c *Conn) AuthRequired() bool { return c.Conn.AuthRequired() }

// TLSRequired reports whether the server requires TLS.
func (c *Conn) TLSRequired() bool { return c.Conn.TLSRequired() }

// Barrier schedules f to run after all prior buffered messages are processed.
func (c *Conn) Barrier(f func()) error { return c.Conn.Barrier(f) }

// GetClientIP returns the client's IP address as seen by the server.
func (c *Conn) GetClientIP() (net.IP, error) { return c.Conn.GetClientIP() }

// GetClientID returns the client's unique ID assigned by the server.
func (c *Conn) GetClientID() (uint64, error) { return c.Conn.GetClientID() }

// StatusChanged returns a channel notified on each of the given status
// transitions.
func (c *Conn) StatusChanged(statuses ...nats.Status) chan nats.Status {
	return c.Conn.StatusChanged(statuses...)
}

// LastError returns the last error reported by the connection.
func (c *Conn) LastError() error { return c.Conn.LastError() }

// ForceReconnect forces the client to reconnect immediately.
func (c *Conn) ForceReconnect() error { return c.Conn.ForceReconnect() }

// TLSConnectionState returns the TLS connection state.
func (c *Conn) TLSConnectionState() (tls.ConnectionState, error) { return c.Conn.TLSConnectionState() }

// ConnectedUrl returns the URL of the server the client is connected to.
func (c *Conn) ConnectedUrl() string { return c.Conn.ConnectedUrl() }

// ConnectedUrlRedacted returns the connected server URL with credentials
// redacted.
func (c *Conn) ConnectedUrlRedacted() string { return c.Conn.ConnectedUrlRedacted() }

// ConnectedAddr returns the address of the server the client is connected to.
func (c *Conn) ConnectedAddr() string { return c.Conn.ConnectedAddr() }

// ConnectedServerId returns the ID of the connected server.
func (c *Conn) ConnectedServerId() string { return c.Conn.ConnectedServerId() }

// ConnectedServerName returns the name of the connected server.
func (c *Conn) ConnectedServerName() string { return c.Conn.ConnectedServerName() }

// ConnectedServerVersion returns the version of the connected server.
func (c *Conn) ConnectedServerVersion() string { return c.Conn.ConnectedServerVersion() }

// ConnectedClusterName returns the cluster name of the connected server.
func (c *Conn) ConnectedClusterName() string { return c.Conn.ConnectedClusterName() }

// SetDisconnectHandler sets the handler invoked on a disconnect.
func (c *Conn) SetDisconnectHandler(dcb nats.ConnHandler) { c.Conn.SetDisconnectHandler(dcb) }

// SetDisconnectErrHandler sets the handler invoked on a disconnect with an
// error.
func (c *Conn) SetDisconnectErrHandler(dcb nats.ConnErrHandler) {
	c.Conn.SetDisconnectErrHandler(dcb)
}

// DisconnectErrHandler returns the disconnect handler currently set.
func (c *Conn) DisconnectErrHandler() nats.ConnErrHandler { return c.Conn.DisconnectErrHandler() }

// SetReconnectHandler sets the handler invoked on a reconnect.
func (c *Conn) SetReconnectHandler(rcb nats.ConnHandler) { c.Conn.SetReconnectHandler(rcb) }

// ReconnectHandler returns the reconnect handler currently set.
func (c *Conn) ReconnectHandler() nats.ConnHandler { return c.Conn.ReconnectHandler() }

// SetDiscoveredServersHandler sets the handler invoked when servers are
// discovered.
func (c *Conn) SetDiscoveredServersHandler(dscb nats.ConnHandler) {
	c.Conn.SetDiscoveredServersHandler(dscb)
}

// DiscoveredServersHandler returns the discovered-servers handler currently set.
func (c *Conn) DiscoveredServersHandler() nats.ConnHandler {
	return c.Conn.DiscoveredServersHandler()
}

// SetClosedHandler sets the handler invoked when the connection is closed.
func (c *Conn) SetClosedHandler(cb nats.ConnHandler) { c.Conn.SetClosedHandler(cb) }

// ClosedHandler returns the closed handler currently set.
func (c *Conn) ClosedHandler() nats.ConnHandler { return c.Conn.ClosedHandler() }

// SetErrorHandler sets the async error handler.
func (c *Conn) SetErrorHandler(cb nats.ErrHandler) { c.Conn.SetErrorHandler(cb) }

// ErrorHandler returns the async error handler currently set.
func (c *Conn) ErrorHandler() nats.ErrHandler { return c.Conn.ErrorHandler() }
