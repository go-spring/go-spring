/*
 * Copyright 2024 The Go-Spring Authors.
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

//go:generate gs mock -o=app_mock.go -i=Server

package gs_app

import (
	"context"
	"sync"
	"sync/atomic"

	"go-spring.org/log"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_conf"
	"go-spring.org/spring/gs/internal/gs_core"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/goutil"
)

// Rooter marks a bean as an application graph root.
//
// App collects Rooter values to make those beans reachable for dependency
// injection. It does not invoke them or attach any lifecycle behavior to them.
type Rooter any

// Runner defines an interface for components that need to be executed
// after all beans have been injected but before servers start.
//
// Runners are executed synchronously and sequentially during application startup.
// Each Runner must complete quickly and should NOT block indefinitely, as this
// would prevent the application from starting. If a Runner returns an error,
// the application startup process will be terminated immediately.
//
// Typical use cases include:
//   - Database schema initialization
//   - Cache warming
//   - One-time data migration tasks
//   - Application bootstrap logic
type Runner interface {
	Run(ctx context.Context) error
}

// ReadySignal defines an interface for signaling application readiness.
// Servers can use this to indicate when they are ready to accept requests.
type ReadySignal interface {
	TriggerAndWait() <-chan struct{}
}

// Server defines the lifecycle of application servers (e.g., HTTP, gRPC).
// It provides methods to start and gracefully stop the server.
//
// Servers are started concurrently in separate goroutines when the application
// runs. Each server is a long-running background process that provides services
// externally. The server must:
//   - Support graceful shutdown via the Stop(ctx) method; ctx is the root
//     context WITHOUT cancellation and WITHOUT a deadline - the app imposes
//     no shutdown timeout, so bounding the shutdown is each server's own
//     responsibility (e.g. context.WithTimeout inside Stop); the context
//     only carries values
//   - Respond to context cancellation for timely cleanup
//   - Signal readiness via ReadySignal before accepting requests
//   - Handle errors appropriately and trigger application shutdown if needed
//
// Typical use cases include:
//   - HTTP servers
//   - gRPC servers
//   - WebSocket servers
//   - TCP/UDP service listeners
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop(ctx context.Context) error
}

// PreStopper is an optional interface a Server may implement to participate in
// graceful drain. When shutdown begins, PreStop is invoked on every server that
// implements it — before any server is stopped — so the server can start refusing to advertise itself as ready (for
// example, flip a readiness probe to OUT_OF_SERVICE) while in-flight requests
// keep being served.
//
// This is what lets a Kubernetes rolling update be lossless: on SIGTERM the
// readiness probe goes false, the endpoint controller removes the pod from
// Service endpoints, and — after whatever drain wait the server itself decides
// on — the servers are then actually stopped.
type PreStopper interface {
	PreStop(ctx context.Context)
}

// ContextProvider is a wrapper that provides explicit access to the
// application's root context. It allows users to inject the context into
// their beans without ambiguity.
//
// This wrapper is necessary because:
//   - It distinguishes the app's root context from other context.Context beans
//   - It provides a clear, intentional injection point for context access
//   - It ensures all components use the same unified context hierarchy
type ContextProvider struct {
	Context context.Context
}

// PropertiesRefresher encapsulates the ability to refresh application
// properties at runtime. Components can inject this bean to trigger
// hot configuration updates without restarting the application.
//
// When RefreshProperties() is called:
//  1. Configuration is reloaded from all sources (files, env, cmd args)
//  2. Changes are propagated to the IoC container
//  3. All dynamic fields (gs.Dync[T]) are updated automatically
type PropertiesRefresher struct {
	app *App
}

// Started reports whether the application has finished wiring its IoC
// container. Check it before calling RefreshProperties (which returns an error
// if called before the app is started) — useful in a Runner that injects this
// bean and runs during startup.
func (c *PropertiesRefresher) Started() bool {
	return c.app.Started()
}

// RefreshProperties refreshes application properties and
// propagates the changes to the IoC container.
func (c *PropertiesRefresher) RefreshProperties() error {
	return c.app.RefreshProperties()
}

// App represents the core application, managing its lifecycle,
// configuration, and dependency injection. It serves as the central
// coordinator for:
//   - Bean registration and wiring via the IoC container
//   - Configuration loading and hot-refreshing
//   - Root component collection through Rooter, Runner, and Server
//   - Runner and Server lifecycle management
//   - Graceful shutdown orchestration
type App struct {
	c      *gs_core.Container // IoC container
	p      *gs_conf.AppConfig // Application configuration
	ctx    context.Context    // Root context for managing cancellation
	cancel context.CancelFunc // Function to cancel the root context
	wg     sync.WaitGroup     // WaitGroup to track running servers

	Rooters []Rooter `autowire:"?"`
	Runners []Runner `autowire:"${spring.app.runners:=?}"`
	Servers []Server `autowire:"${spring.app.servers:=?}"`

	// started is set to true after the IoC container has been fully wired
	// (app.c.Refresh returned successfully). RefreshProperties refuses to
	// run before this flag is set to avoid operating on a partially-wired
	// container.
	started atomic.Bool
}

// NewApp creates a new App instance with an initialized root context.
func NewApp() *App {
	// nolint: staticcheck
	ctx := context.WithValue(context.Background(), "app", "")
	ctx, cancel := context.WithCancel(ctx)
	return &App{
		c:      gs_core.New(),
		p:      gs_conf.NewAppConfig(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context returns the root context for the application.
func (app *App) Context() context.Context {
	return app.ctx
}

// Started reports whether the application has finished wiring its IoC container
// (app.c.Refresh returned). Callers of RefreshProperties can check this first to
// avoid the "app not started yet" error, e.g. when a Runner injects a
// PropertiesRefresher and runs during startup, before Started becomes true.
func (app *App) Started() bool {
	return app.started.Load()
}

// Property sets an app-level property in the application's configuration.
// This method allows programmatic configuration during initialization.
func (app *App) Property(key string, val string) {
	app.p.Properties.Set(key, val)
}

// Provide registers a new bean definition in the IoC container.
// The parameter can be either an existing instance or a constructor function.
// Additional arguments can be passed for dependency injection.
func (app *App) Provide(objOrCtor any, args ...gs.Arg) *gs_bean.BeanDefinition {
	return app.c.Provide(objOrCtor, args...).Caller(2)
}

// RefreshProperties reloads application properties from all sources
// and propagates the changes to the IoC container, enabling hot configuration updates.
//
// This method triggers a complete configuration refresh:
//  1. Reloads configuration from all sources (files, env vars, cmd args)
//  2. Merges configurations according to priority rules
//  3. Propagates changes to the IoC container
//  4. Updates all dynamic fields (gs.Dync[T]) automatically
//
// Thread safety:
//   - This method is thread-safe and can be called from any goroutine
//   - All dynamic field updates are atomic
//   - If validation fails, no partial updates are applied
func (app *App) RefreshProperties() error {
	if !app.started.Load() {
		return errutil.Explain(nil, "app not started yet, cannot refresh properties")
	}
	p, err := app.p.Refresh()
	if err != nil {
		return err
	}
	return app.c.RefreshProperties(p)
}

// initLog initializes the application's logging system based on configuration.
// It configures the global logger if the "logging" section exists in the
// provided configuration storage. When no "logging" section is present,
// the application uses the default logging configuration.
func (app *App) initLog(p flatten.Storage) error {
	const loggingKey = "logging"
	if !p.Exists(loggingKey) { // no logging
		return nil
	}
	log.Debugf(app.ctx, log.TagAppDef, "initializing logging system from configuration")
	s := flatten.NewPrefixedStorage(p, loggingKey+".")
	return log.Refresh(s)
}

// Start initializes and launches the application.
// The startup sequence is:
//  1. Register the ContextProvider and PropertiesRefresher beans
//  2. Refresh application properties from all sources
//  3. Initialize logging system
//  4. Refresh the IoC container with App as the graph root, wiring Rooter,
//     Runner, Server, and other dependencies reachable from App
//  5. Execute all Runner beans sequentially
//  6. Start all configured servers in separate goroutines
//     - Each server signals readiness via ReadySignal
//     - If a server panics or returns an unexpected error, ReadySignal is intercepted
//     and the application initiates a graceful shutdown
//  8. Wait until all servers signal readiness or intercept occurs
func (app *App) Start() error {

	app.c.Provide(&PropertiesRefresher{app})
	app.c.Provide(&ContextProvider{app.ctx})

	// Load and refresh application properties
	p, err := app.p.Refresh()
	if err != nil {
		return err
	}

	// Initialize logger
	if err = app.initLog(p); err != nil {
		return err
	}

	// Refresh IoC container to wire all beans
	var roots []*gs_bean.BeanDefinition
	roots = append(roots, gs_bean.NewBean(app))
	if err = app.c.Refresh(p, roots); err != nil {
		return err
	}

	// Mark the app as started so RefreshProperties is allowed.
	app.started.Store(true)

	// Execute all Runner beans sequentially
	for _, r := range app.Runners {
		if err = r.Run(app.ctx); err != nil {
			return err
		}
	}

	// Start all configured servers
	if len(app.Servers) > 0 {
		sig := NewReadySignal() // Coordinate readiness across servers
		for _, svr := range app.Servers {
			app.wg.Add(1)
			svrSig := sig.Add()
			goutil.Go(app.ctx, func(ctx context.Context) {
				defer app.wg.Done()
				defer func() {
					// Recover from server panics and trigger shutdown
					if r := recover(); r != nil {
						svrSig.Intercept()
						app.ShutDown()
						panic(r) // re-panic so goutil.Go can handle it
					}
				}()
				if err := svr.Run(ctx, svrSig); err != nil {
					log.Errorf(ctx, log.TagAppDef, "server serve error: %v", err)
					svrSig.Intercept()
					app.ShutDown()
				} else {
					log.Infof(ctx, log.TagAppDef, "server closed")
				}
			}, goutil.InheritCancel)
		}

		// Wait until all servers signal readiness
		sig.Wait()
		sig.Close()
		if sig.Intercepted() {
			log.Infof(app.ctx, log.TagAppDef, "server intercepted")
			return errutil.Explain(nil, "server intercepted")
		}
		log.Infof(app.ctx, log.TagAppDef, "ready to serve requests")
	}
	return nil
}

// WaitForShutdown blocks until the application is signaled to shut down.
// After shutdown is triggered:
//  1. All servers are stopped concurrently
//  2. Waits for all server goroutines to complete
//  3. Closes the IoC container
//
// Process-global cleanup (gs.RegisterStopper stoppers, which include the logging
// system) is NOT done here: it lives in the top-level defer in Run/RunTest so the
// same cleanup also covers the Start-failure path that never reaches here. The
// defer runs right after this returns, preserving the intended sequence:
// servers stop -> container closes -> stoppers flush (log last).
func (app *App) WaitForShutdown() {
	// Block until the root context is cancelled
	<-app.ctx.Done()

	// Graceful drain: give drain-aware servers a chance to stop advertising
	// readiness (e.g. actuator /readiness -> OUT_OF_SERVICE) before we stop
	// serving. The app imposes no delay and no timeout here: how long to wait
	// for load balancers to de-route, and how long shutdown may take, are
	// decided by each server itself (e.g. inside PreStop / Stop).
	drainCtx := context.WithoutCancel(app.ctx)
	for _, svr := range app.Servers {
		if ps, ok := svr.(PreStopper); ok {
			ps.PreStop(drainCtx)
		}
	}

	// Stop all servers concurrently. The context passed to Stop is the root
	// context without cancellation and without a deadline: it only carries
	// values, never cancellation.
	stopCtx := context.WithoutCancel(app.ctx)
	var stopWg sync.WaitGroup
	for _, svr := range app.Servers {
		stopWg.Add(1)
		goutil.Go(app.ctx, func(ctx context.Context) {
			defer stopWg.Done()
			var err error
			err = svr.Stop(stopCtx)
			if err != nil {
				log.Errorf(ctx, log.TagAppDef, "shutdown server failed: %v", err)
			}
		}, goutil.DetachCancel)
	}

	// Wait indefinitely for all servers and their goroutines to stop. The app
	// imposes no timeout: each server bounds its own shutdown (e.g. inside
	// Stop); a server that hangs hangs the process, by design.
	stopWg.Wait()
	app.wg.Wait()

	app.c.Close()
	log.Infof(app.ctx, log.TagAppDef, "shutdown complete")
}

// ShutDown initiates a graceful shutdown of the application.
func (app *App) ShutDown() {
	log.Infof(app.ctx, log.TagAppDef, "shutting down")
	app.cancel()
}
