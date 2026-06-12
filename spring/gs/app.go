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

package gs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"testing"

	"github.com/go-spring/log"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_app"
	"go-spring.org/spring/gs/internal/gs_bean"
	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/goutil"
)

// inited indicates whether the application has been initialized.
// Once set to true, it prevents further gs.Provide() calls during runtime
// to ensure all bean definitions are registered during package initialization phase.
var inited bool

// App defines the configuration interface of a Go-Spring application.
// Methods on App are only valid during application configuration
// and must not be called after the application has started.
type App interface {
	// Property sets a key-value property in the application configuration.
	Property(key string, val string)
	// Root marks a bean as the root bean.
	Root(obj any) *gs_bean.BeanDefinition
	// Provide registers an object or constructor as a bean in the application.
	Provide(objOrCtor any, args ...gs.Arg) *gs_bean.BeanDefinition
}

// AppStarter wraps a gs_app.App and manages its lifecycle.
// It provides methods for initialization, configuration, starting,
// stopping, running, and testing the application.
type AppStarter struct {
	app *gs_app.App
	cfg func(App)
}

// newApp creates a new application instance.
func newApp() *AppStarter {
	inited = true
	return &AppStarter{app: gs_app.NewApp()}
}

// Web creates a new application with web server enabled.
func Web(enable bool) *AppStarter {
	return Configure(func(app App) {
		if !enable {
			app.Property("spring.http.server.enabled", "false")
		}
	})
}

// Configure creates a new application and registers a configuration
// function that will be applied before the application starts.
//
// Example:
//
//	gs.Configure(func(app gs.App) {
//	    app.Property("server.port", "8080")
//	    app.Provide(&MyService{})
//	}).Run()
func Configure(cfg func(App)) *AppStarter {
	return newApp().Configure(cfg)
}

// Configure allows you to modify the application configuration.
// Accumulates configuration functions - each call adds to the chain.
// Configuration functions execute in order of registration during startApp().
//
// Important:
//   - Multiple Configure() calls accumulate, not replace
//   - Order matters: earlier configs execute before later ones
//   - All configs run before app.Start() is called
func (s *AppStarter) Configure(cfg func(App)) *AppStarter {
	prev := s.cfg
	s.cfg = func(app App) {
		if prev != nil {
			prev(app)
		}
		cfg(app)
	}
	return s
}

// startApp starts the application lifecycle by printing the banner,
// applying the configuration function, and starting the underlying gs_app.App.
// Returns an error if the application fails to start.
func (s *AppStarter) startApp() error {

	// Print banner
	printBanner()

	// Apply user configuration
	if s.cfg != nil {
		s.cfg(s.app)
	}

	// Start application
	if err := s.app.Start(); err != nil {
		err = errutil.Explain(err, "start app failed")
		log.Errorf(s.app.Context(), log.TagAppDef, "%s", err)
		return err
	}

	return nil
}

// Run creates and starts a new application using default settings.
// Blocks until termination signal is received (SIGINT/SIGTERM).
func Run() {
	newApp().Run()
}

// Run starts the application, applies configuration, and waits for
// termination signals (e.g., SIGTERM, Ctrl+C) to trigger a graceful shutdown.
func (s *AppStarter) Run() {

	// Error has already been logged
	if err := s.startApp(); err != nil {
		return
	}

	// Listen for termination signals in a separate goroutine
	// Handles SIGINT (Ctrl+C) and SIGTERM for graceful shutdown
	goutil.Go(s.app.Context(), func(ctx context.Context) {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		signal.Stop(ch)
		close(ch)
		log.Infof(ctx, log.TagAppDef, "Received signal: %v", sig)
		s.app.ShutDown()
	}, goutil.InheritCancel)

	// Wait for shutdown to complete
	// Blocks until all servers have stopped and cleanup is done
	s.app.WaitForShutdown()
}

// RunAsync runs the application asynchronously and
// returns a function to stop the application.
// Convenience wrapper that creates a new AppStarter.
//
// Returns:
//   - stop: Function to gracefully shutdown the application
//   - err: Error if application failed to start (nil on success)
//
// Usage:
//
//	stop, err := gs.RunAsync()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stop() // Ensure cleanup
//	// ... do work ...
func RunAsync() (stop func(), err error) {
	return newApp().RunAsync()
}

// RunAsync runs the application asynchronously and
// returns a function to stop the application.
//
// Returns:
//   - stop: Closure that calls ShutDown() and WaitForShutdown()
//   - err: Startup error if startApp() fails
//
// Behavior:
//   - Calls startApp() synchronously to initialize application
//   - On success: Returns stop function for manual shutdown control
//   - On error: Returns no-op function and error
//
// Caller Responsibility:
//   - Must call stop() to ensure graceful shutdown
//   - Should handle startup errors appropriately
//   - Can use defer for guaranteed cleanup
func (s *AppStarter) RunAsync() (stop func(), err error) {

	if err = s.startApp(); err != nil {
		return func() {}, err
	}

	return func() {
		s.app.ShutDown()
		s.app.WaitForShutdown()
	}, nil
}

// RunTest runs a test function using a new application instance.
// Convenience wrapper that creates a fresh AppStarter for testing.
//
// Parameters:
//   - t: Testing.T instance for test reporting and failure handling
//   - f: Test function accepting exactly one pointer-to-struct argument
//
// Requirements:
//   - Test function signature: func(*TestStruct)
//   - TestStruct fields can use autowire/value tags for injection
//   - Struct is registered as root bean with full dependency injection
//
// Example:
//
//	func TestExample(t *testing.T) {
//	    gs.RunTest(t, func(ts *struct {
//	        DB *Database `autowire:""`
//	    }) {
//	        // ts.DB is auto-injected
//	        result := ts.DB.Query(...)
//	        assert.NotNil(t, result)
//	    })
//	}
func RunTest(t *testing.T, f any) {
	newApp().RunTest(t, f)
}

func validateRunTestFunc(f any) (reflect.Type, reflect.Value, error) {
	if f == nil {
		return nil, reflect.Value{}, fmt.Errorf("RunTest requires func(*TestStruct), got <nil>")
	}

	fv := reflect.ValueOf(f)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, reflect.Value{}, fmt.Errorf("RunTest requires func(*TestStruct), got %s", ft)
	}
	if fv.IsNil() {
		return nil, reflect.Value{}, fmt.Errorf("RunTest requires non-nil func(*TestStruct)")
	}
	if ft.NumIn() != 1 {
		return nil, reflect.Value{}, fmt.Errorf("RunTest requires exactly one argument, got %d", ft.NumIn())
	}

	argType := ft.In(0)
	if argType.Kind() != reflect.Pointer || argType.Elem().Kind() != reflect.Struct {
		return nil, reflect.Value{}, fmt.Errorf("RunTest argument must be pointer to struct, got %s", argType)
	}

	return ft, fv, nil
}

// RunTest runs a user-defined test function with an auto-created test object.
// It extracts the test object type from the test function parameter, creates
// the test object, registers it as a root bean, initializes the application,
// starts the application, executes the test, and ensures graceful shutdown.
func (s *AppStarter) RunTest(t *testing.T, f any) {
	ft, fv, err := validateRunTestFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	obj := reflect.New(ft.In(0).Elem())

	// Register the root bean
	s.app.Root(obj.Interface())

	// Force autowire to be nullable
	s.app.Property("spring.force-autowire-is-nullable", "true")

	stop, err := s.RunAsync()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { stop() }()

	// Execute the test function
	fv.Call([]reflect.Value{obj})
}
