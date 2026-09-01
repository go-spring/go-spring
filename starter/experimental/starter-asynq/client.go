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
// their Init/Destroy. Both share the Config-derived RedisConnOpt; the server
// additionally holds the handler registry the app populates before Run.
package StarterAsynq

import (
	"context"

	"github.com/hibiken/asynq"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	"go-spring.org/spring/gs"
)

// Client is the producer bean: it enqueues tasks into one Asynq queue. It
// embeds *asynq.Client so every enqueue method promotes unchanged; the
// executor guards the synchronous Enqueue path (the overload-sensitive
// operation, since enqueue touches Redis).
type Client struct {
	*asynq.Client
	// Observability is field-injected by gs from the top-level
	// "observability.*" keys. The instance-prefixed
	// "spring.asynq.<name>.observability.*" keys land in cfg.Observability
	// and take precedence — see resolveObservability.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	cfg      Config
	resource string
	exec     resilience.Executor
	obs      *observe.Observer
}

// resolveObservability merges the two observability config surfaces into the
// effective policy Init arms:
//
//   - the instance-prefixed "spring.asynq.<name>.observability.*" keys, bound
//     into Config (cfg.Observability) by conf.BindEach;
//   - the top-level "observability.*" keys, field-injected into the wrapper's
//     Observability field (an absolute property reference, kept for backward
//     compatibility with configs that predate the instance keys).
//
// Instance-level keys override top-level ones per field. Because binding
// fills the defaults ("brief" / 512 / no skips) even when no instance key is
// present, an instance value is only detectable as "set" when it differs
// from those defaults. Configure one surface only and this caveat
// disappears.
func (o *Client) resolveObservability() observe.ObserveConfig {
	c := o.Observability // top-level fallback
	i := o.cfg.Observability
	if i.Level != "" && i.Level != observe.DefaultBrief {
		c.Level = i.Level
	}
	if i.MaxArgBytes != 0 && i.MaxArgBytes != 512 {
		c.MaxArgBytes = i.MaxArgBytes
	}
	if len(i.SkipOps) > 0 {
		c.SkipOps = i.SkipOps
	}
	return c
}

// Init arms the observe + resilience executor after field injection.
func (o *Client) Init() error {
	obsCfg := o.resolveObservability()
	o.obs = observe.NewProducer("asynq", obsCfg)
	o.resource = resilience.ResourceLabel("asynq", o.cfg.Addr)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, "asynq", obsCfg)
	o.exec = exec
	return nil
}

// Destroy closes the producer and the resilience executor.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Client.Close()
}

// Enqueue is the guarded, observed enqueue path. It behaves like
// Client.Enqueue but routes the call through the resilience executor and
// emits a producer observation; a rejection (rate-limit / open circuit)
// never reaches Redis.
func (o *Client) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	var info *asynq.TaskInfo
	call := func(ctx context.Context) error {
		var err error
		info, err = o.Client.EnqueueContext(ctx, task, opts...)
		return err
	}
	if o.obs != nil {
		inner := call
		call = func(ctx context.Context) error {
			ctx, sp := o.obs.Start(ctx, "enqueue", task.Type())
			err := inner(ctx)
			sp.End(err)
			return err
		}
	}
	if o.exec == nil {
		return info, call(ctx)
	}
	if err := o.exec.Execute(ctx, o.resource, call); err != nil {
		return nil, err
	}
	return info, nil
}

// Server is the worker bean. It holds the handler mux the application
// populates (RegisterHandler) before the container runs the server, plus the
// asynq server built from Config. Destroy calls Shutdown, which drains
// in-flight tasks up to ShutdownTimeout.
//
// Unlike the producer Client, the worker emits no observations of its own
// (per-task handling is asynq's domain — handler errors/panics are recovered
// and retried by asynq), so it carries no observability config.
type Server struct {
	cfg      Config
	resource string
	mux      *asynq.ServeMux
	srv      *asynq.Server
}

// Init builds the asynq server (reusing any mux the app already created via
// RegisterHandler).
func (o *Server) Init() error {
	o.resource = resilience.ResourceLabel("asynq", o.cfg.Addr)
	connOpt, err := newRedisConnOpt(context.Background(), o.cfg)
	if err != nil {
		return err
	}
	o.srv = asynq.NewServer(connOpt, asynq.Config{
		Concurrency:     o.cfg.Concurrency,
		Queues:          o.cfg.Queues,
		ShutdownTimeout: o.cfg.ShutdownTimeout,
		// Errors and panics inside a handler are asynq's to recover and
		// retry; a handler error is reported via ErrorHandler and a panic is
		// recovered by asynq's own guard. We keep our own reporting out of
		// the hot path — see DESIGN for the boundary.
	})
	return nil
}

// RegisterHandler registers fn as the handler for task pattern. It may be
// called before or after the container wires the bean (handlers are fixed
// once the server starts consuming): the mux is created lazily so app code
// can register handlers against a freshly-constructed Server in its wiring
// without racing Init.
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

// Stop implements gs.Server: shut the worker down, draining in-flight tasks.
func (o *Server) Stop() error {
	return o.StopContext(context.Background())
}

// StopContext implements gs.Stopper: shut the worker down, draining in-flight
// tasks. asynq's Shutdown takes no context, so ctx is unused here - the drain
// is bounded by the configured ShutdownTimeout.
func (o *Server) StopContext(ctx context.Context) error {
	o.srv.Shutdown()
	return nil
}

// Destroy shuts the worker down, draining in-flight tasks (bean destroy path).
func (o *Server) Destroy() error {
	return o.Stop()
}

// newRedisConnOpt builds the RedisConnOpt from Config via the selected
// driver, shared by the client and server roles.
func newRedisConnOpt(ctx context.Context, c Config) (asynq.RedisConnOpt, error) {
	d, err := lookupDriver(c.Driver)
	if err != nil {
		return nil, err
	}
	return d.RedisConnOpt(ctx, c)
}
