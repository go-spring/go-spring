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

// Tests for app.go: app bootstrapping, Configure/Web, RunTest and its
// signature validation, and the Start-failure shutdown path.

package gs

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/testing/assert"
)

func init() {
	Provide(func(ctx *ContextProvider) *GlobalService {
		return &GlobalService{}
	})
}

type GlobalService struct {
	Name string `value:"${name:=global}"`
}

// closeRecorderProvider is a config provider that only counts its Close calls,
// used to prove the runtime releases providers on shutdown.
type closeRecorderProvider struct {
	closes atomic.Int32
}

func (p *closeRecorderProvider) Load(optional bool, source string) (map[string]string, error) {
	return nil, nil
}

func (p *closeRecorderProvider) Close(context.Context) error {
	p.closes.Add(1)
	return nil
}

// TestRunTestClosesConfigProviders proves the shutdown path reaches the config
// providers: a provider registered in init() is closed when the application
// stops, so its watchers and listeners do not outlive the app (which matters
// when a process runs several instances, e.g. sequential gs.RunTest runs).
//
// Two closes are expected, not one: the graceful shutdown path closes the
// providers while the container is still alive, and the deferred safety net
// (which covers a failed start) closes them again. Closing twice must stay
// harmless — that is part of the Provider contract.
func TestRunTestClosesConfigProviders(t *testing.T) {
	p := &closeRecorderProvider{}
	conf.RegisterProvider("close-recorder", p)

	Web(false).RunTest(t, func(s *struct {
		Name string `value:"${name:=app}"`
	}) {
		// Nothing to assert inside the app: the interesting moment is shutdown.
		assert.That(t, s.Name).Equal("app")
	})

	assert.Number(t, p.closes.Load()).Equal(2)
}

type App1Service struct {
	Name string         `value:"${name:=app1}"`
	Svr  *GlobalService `autowire:""`
}

func TestApp1(t *testing.T) {
	Web(false).RunTest(t, func(s *App1Service) {
		assert.That(t, s.Name).Equal("app1")
		assert.That(t, s.Svr).NotNil()
		assert.That(t, s.Svr.Name).Equal("global")
	})
}

func TestApp2(t *testing.T) {
	Web(false).Configure(func(g App) {
		g.Property("name", "myapp2")
	}).RunTest(t, func(s *struct {
		Name string         `value:"${name:=app2}"`
		Svr  *GlobalService `autowire:""`
		App1 *App1Service   `autowire:"?"`
	}) {
		assert.That(t, s.Name).Equal("myapp2")
		assert.That(t, s.Svr).NotNil()
		assert.That(t, s.Svr.Name).Equal("myapp2")
		assert.That(t, s.App1).Nil() // no App1Service bean in this app
	})
}

// TestWeb proves Web(false) keeps the built-in HTTP server module from
// registering its beans, and that the server is provided by default
// (on an ephemeral port) when not disabled.
func TestWeb(t *testing.T) {

	t.Run("server disabled", func(t *testing.T) {
		Web(false).RunTest(t, func(s *struct {
			Svr *SimpleHttpServer `autowire:"?"`
		}) {
			assert.That(t, s.Svr).Nil()
		})
	})

	t.Run("server enabled", func(t *testing.T) {
		Configure(func(app App) {
			app.Property("spring.http.server.addr", ":0")
		}).RunTest(t, func(s *struct {
			Svr *SimpleHttpServer `autowire:""`
		}) {
			assert.That(t, s.Svr).NotNil()
		})
	})
}

// failingRunner is a gs.Runner whose Run always errors, so App.Start fails after
// the IoC container is wired (and after any module setup has registered stoppers).
type failingRunner struct{}

func (failingRunner) Run(context.Context) error {
	return errFailingRunner
}

var errFailingRunner = errors.New("failing runner")

// TestStopperFlushedOnStartFailure proves that when App.Start fails (so Run
// never reaches WaitForShutdown), the top-level defer in Run still flushes
// registered stoppers. Without that defer a stopper registered during setup
// would leak its buffered data on a failed boot.
func TestStopperFlushedOnStartFailure(t *testing.T) {
	resetStoppersForTest()

	// Run() blocks until startApp fails (failingRunner) then returns; it never
	// enters the signal-wait phase. The defer flushes the stopper registered here.
	var ran atomic.Bool
	RegisterStopper("test-start-failure", func(context.Context) error {
		ran.Store(true)
		return nil
	})

	Configure(func(g App) {
		g.Provide(func() Runner { return failingRunner{} })
	}).Run()

	if !ran.Load() {
		t.Fatal("stopper was not flushed on Start failure; the failure-path defer is missing")
	}
}

// runTestTarget is a plain struct used to exercise RunTest's signature checks.
type runTestTarget struct{}

func TestValidateRunTestFunc(t *testing.T) {

	t.Run("invalid", func(t *testing.T) {
		var nilFn func(*runTestTarget)
		cases := []struct {
			name string
			fn   any
			err  string
		}{
			{
				name: "nil",
				fn:   nil,
				err:  `RunTest requires func\(\*Struct\), got <nil>`,
			},
			{
				name: "non function",
				fn:   1,
				err:  `RunTest requires func\(\*Struct\), got int`,
			},
			{
				name: "nil function",
				fn:   nilFn,
				err:  `RunTest requires non-nil func\(\*Struct\)`,
			},
			{
				name: "no arguments",
				fn:   func() {},
				err:  `RunTest requires exactly one argument, got 0`,
			},
			{
				name: "too many arguments",
				fn:   func(*runTestTarget, int) {},
				err:  `RunTest requires exactly one argument, got 2`,
			},
			{
				name: "non pointer",
				fn:   func(runTestTarget) {},
				err:  `RunTest argument must be pointer to struct, got .*runTestTarget`,
			},
			{
				name: "non struct pointer",
				fn:   func(*int) {},
				err:  `RunTest argument must be pointer to struct, got \*int`,
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				_, _, err := validateRunTestFunc(c.fn)
				assert.Error(t, err).Matches(c.err)
			})
		}
	})

	t.Run("valid", func(t *testing.T) {
		fn := func(*runTestTarget) {}
		ft, fv, err := validateRunTestFunc(fn)
		assert.That(t, err).Nil()
		assert.That(t, ft).Equal(reflect.TypeFor[func(*runTestTarget)]())
		assert.That(t, fv.IsValid()).True()
	})
}
