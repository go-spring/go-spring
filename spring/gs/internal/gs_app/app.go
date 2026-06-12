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

	"github.com/go-spring/log"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_conf"
	"go-spring.org/spring/gs/internal/gs_core"
	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
	"github.com/go-spring/stdlib/goutil"
)

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
//   - Support graceful shutdown via the Stop() method
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
	Stop() error
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
//   - Runner and Server lifecycle management
//   - Graceful shutdown orchestration
type App struct {
	c *gs_core.Container // IoC container
	p *gs_conf.AppConfig // Application configuration

	ctx    context.Context    // Root context for managing cancellation
	cancel context.CancelFunc // Function to cancel the root context
	wg     sync.WaitGroup     // WaitGroup to track running servers

	Runners []Runner `autowire:"${spring.app.runners:=?}"`
	Servers []Server `autowire:"${spring.app.servers:=?}"`

	roots []*gs_bean.BeanDefinition // Root beans for container refresh
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

// Root beans serve as entry points for the dependency injection graph.
//
// Unlike regular Provide(), which only registers a bean definition,
// Root() also marks the bean as a "root" that triggers recursive wiring
// of all its dependencies during container initialization.
func (app *App) Root(obj any) *gs_bean.BeanDefinition {
	b := app.c.Provide(obj).Caller(2)
	app.roots = append(app.roots, b)
	return b
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
	if app.p == nil {
		return errutil.Explain(nil, "app.p is nil")
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
	s := flatten.NewPrefixedStorage(p, loggingKey+".")
	return log.Refresh(s)
}

// Start initializes and launches the application.
// The startup sequence is:
//  1. Register the App, ContextProvider, and PropertiesRefresher beans
//  2. Refresh application properties from all sources
//  3. Initialize logging system
//  4. Refresh the IoC container to wire all beans
//  5. Clear the temporary root bean list after container refresh
//  6. Execute all Runner beans sequentially
//  7. Start all configured servers in separate goroutines
//     - Each server signals readiness via ReadySignal
//     - If a server panics or returns an unexpected error, ReadySignal is intercepted
//     and the application initiates a graceful shutdown
//  8. Wait until all servers signal readiness or intercept occurs
func (app *App) Start() error {

	app.Root(app)
	app.c.Provide(&ContextProvider{app.ctx})
	app.c.Provide(&PropertiesRefresher{app})

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
	if err = app.c.Refresh(p, app.roots); err != nil {
		return err
	}

	app.roots = nil
	// If there are no dynamic fields, clear the configuration
	if app.c.DynamicObjectsCount() == 0 {
		app.p = nil
	}

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
			sig.Add()
			app.wg.Add(1)
			goutil.Go(app.ctx, func(ctx context.Context) {
				defer app.wg.Done()
				defer func() {
					// Recover from server panics and trigger shutdown
					if r := recover(); r != nil {
						sig.Intercept()
						app.ShutDown()
						panic(r) // re-panic so goutil.Go can handle it
					}
				}()
				if err := svr.Run(ctx, sig); err != nil {
					log.Errorf(ctx, log.TagAppDef, "server serve error: %v", err)
					sig.Intercept()
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
//  4. Cleans up and destroys the logging system
func (app *App) WaitForShutdown() {
	// Block until the root context is cancelled
	<-app.ctx.Done()

	// Stop all servers concurrently
	var stopWg sync.WaitGroup
	for _, svr := range app.Servers {
		stopWg.Add(1)
		goutil.Go(app.ctx, func(ctx context.Context) {
			defer stopWg.Done()
			if err := svr.Stop(); err != nil {
				log.Errorf(ctx, log.TagAppDef, "shutdown server failed: %v", err)
			}
		}, goutil.DetachCancel)
	}

	stopWg.Wait()
	app.wg.Wait()
	app.c.Close()
	log.Infof(app.ctx, log.TagAppDef, "shutdown complete")
	log.Destroy()
}

// ShutDown initiates a graceful shutdown of the application.
func (app *App) ShutDown() {
	log.Infof(app.ctx, log.TagAppDef, "shutting down")
	app.cancel()
}
