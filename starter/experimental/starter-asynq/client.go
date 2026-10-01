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

// client.go is the "resource entity + lifecycle" of this starter: the Client
// wrapper bean (enqueue producer) and the Server wrapper bean (worker), plus
// their Destroy. Both are fully assembled by their constructors — there is no
// Init hook — and share the Config-derived RedisConnOpt; the server
// additionally holds the handler registry the app populates before Run.
package StarterAsynq

import (
	"context"

	"github.com/hibiken/asynq"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
)

// Client is the producer bean: it enqueues tasks into one Asynq queue. It
// embeds *asynq.Client so every enqueue method promotes unchanged; the
// executor guards the synchronous Enqueue path (the overload-sensitive
// operation, since enqueue touches Redis).
type Client struct {
	*asynq.Client

	// serviceLabel is the resilience service key ("asynq:<addr>") the executor
	// scopes limiter/breaker state by; it is fixed by [NewClient].
	serviceLabel string

	// exec is the resilience executor protecting the synchronous Enqueue path,
	// fixed by [NewClient]; a no-op when governance is off.
	exec resilience.ClientExecutor
}

// NewClient builds a complete Client — identity and governance both applied —
// over the asynq producer built from connOpt. addr is the Redis host:port the
// instance addresses (the config entry's addr); the governance service label
// ("asynq:<addr>") is derived from it.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client passes the zero [cloud.ClientParams]; its executor then
// degrades to resilience.Unmanaged — observed, with a one-time warning that no
// protection applies — rather than silently running bare.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(connOpt asynq.RedisConnOpt, addr string, params cloud.ClientParams) *Client {
	o := &Client{
		Client:       asynq.NewClient(connOpt),
		serviceLabel: resilience.ServiceLabel("asynq", addr),
	}
	o.exec = params.ExecutorFor("asynq", o.serviceLabel)
	return o
}

// Destroy closes the producer and the resilience executor.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Client.Close()
}

// Enqueue is the guarded, observed enqueue path. It behaves like
// Client.Enqueue but declares the call's identity on the context and routes it
// through the resilience executor, which is also where the span, the metrics
// and the access log are emitted; a rejection (rate-limit / open circuit) never
// reaches Redis.
func (o *Client) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	ctx = observability.WithOperation(ctx, operation("enqueue", task.Type()))
	info, err := resilience.Run(ctx, o.exec, func(attemptCtx context.Context) (*asynq.TaskInfo, error) {
		return o.Client.EnqueueContext(attemptCtx, task, opts...)
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Server is the worker bean. It holds the handler mux the application
// populates (RegisterHandler) before the container runs the server, plus the
// asynq server built from Config. Destroy calls Shutdown, which drains
// in-flight tasks up to ShutdownTimeout.
//
// Unlike the producer Client, the worker declares no operation of its own
// (per-task handling is asynq's domain — handler errors/panics are recovered
// and retried by asynq), so it carries no observability config.
type Server struct {
	mux *asynq.ServeMux
	srv *asynq.Server
}

// RegisterHandler registers fn as the handler for task pattern. It may be
// called before or after the container wires the bean (handlers are fixed
// once the server starts consuming): the mux is created lazily so app code
// can register handlers against a freshly-constructed Server in its wiring
// without racing the constructor.
//
// pattern is the task type name, with ":" as the group separator for
// middleware scoping.
func (o *Server) RegisterHandler(pattern string, fn asynq.HandlerFunc) {
	o.muxLazy().HandleFunc(pattern, fn)
}

// muxLazy returns the shared mux, creating it on first use.
func (o *Server) muxLazy() *asynq.ServeMux {
	if o.mux == nil {
		o.mux = asynq.NewServeMux()
	}
	return o.mux
}

// Handler exposes the built mux, for a custom MiddlewareFunc chain or a
// manual Run.
func (o *Server) Handler() asynq.Handler { return o.muxLazy() }

// Run implements gs.Server: start the worker, block until ctx is cancelled or
// Stop is called, then shut down.
//
// We deliberately do NOT use asynq.Server.Run: that helper installs its own
// signal handler (waitForSignals) which would race gs's graceful-shutdown
// signal handling. Start + wait-on-ctx keeps signal handling in gs's hands.
func (o *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	if err := o.srv.Start(o.muxLazy()); err != nil {
		return err
	}
	if sig != nil {
		sig.TriggerAndWait()
	}
	<-ctx.Done()
	o.srv.Shutdown()
	return nil
}

// Stop shuts the worker down, draining in-flight tasks. asynq's Shutdown takes
// no context, so ctx is unused here - the drain is bounded by the configured
// ShutdownTimeout.
func (o *Server) Stop(ctx context.Context) error {
	o.srv.Shutdown()
	return nil
}

// Destroy shuts the worker down, draining in-flight tasks (bean destroy path).
func (o *Server) Destroy() error {
	return o.Stop(context.Background())
}
