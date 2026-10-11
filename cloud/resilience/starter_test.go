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

package resilience

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/spring/gs"
)

// closingDriver is a Driver whose executors count their Close, so a test can see
// whether the manager released them.
type closingDriver struct{ closed *atomic.Int32 }

func (d closingDriver) NewClientExecutor(string, ClientPolicy) (chain.Executor, error) {
	return closingExecutor{closed: d.closed}, nil
}

func (d closingDriver) NewServerExecutor(string, ServerPolicy) (chain.Executor, error) {
	return closingExecutor{closed: d.closed}, nil
}

// managerProbe exists only to make gs instantiate the manager bean and hand it
// to the test.
type managerProbe struct{}

// closingExecutor is a do-nothing executor that reports every Close.
type closingExecutor struct{ closed *atomic.Int32 }

func (e closingExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
	return fn(ctx)
}

func (e closingExecutor) Close() error {
	e.closed.Add(1)
	return nil
}

// TestStarter_ManagerBeanClosesOnShutdown pins the manager bean's destroy hook:
// it is the ONLY caller of [Manager.Close], and Close is the only thing that
// releases the executors the manager registered — and, with them, whatever
// background resources their driver holds (metric pumps, in a production
// engine). Without the hook, Close is unreachable and those executors outlive
// the container.
func TestStarter_ManagerBeanClosesOnShutdown(t *testing.T) {
	var closed atomic.Int32

	var m *Manager

	gs.Web(false).Configure(func(app gs.App) {
		// Select the closing driver at startup, the way a deployment selects a
		// backend: ${spring.resilience.driver} names the Driver bean's name.
		app.Property(DriverProperty, "closing")
		app.Provide(func() *closingDriver { return &closingDriver{closed: &closed} }).
			Name("closing").Export(gs.As[Driver]())
		// A stand-in for a client starter: it injects the manager bean so gs
		// instantiates it, and captures it for the test. Exported as a Rooter
		// because nothing autowires it.
		app.Provide(func(mgr *Manager) *managerProbe {
			m = mgr
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if err := m.Apply(Settings{
			Client: ClientSettings{Default: ClientPolicy{AttemptTimeout: time.Second}},
		}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		// Build one executor, so there is something for Close to release.
		exec := m.ClientExecutorFor("sys", "res:1")
		if err := exec.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil }); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if got := closed.Load(); got != 0 {
			t.Fatalf("precondition: nothing closes while the container is up, got %d", got)
		}
	})

	// The container has torn down, so the manager bean is gone.
	if got := closed.Load(); got != 1 {
		t.Fatalf("the manager bean must release its executors on shutdown: got %d closes, want 1", got)
	}
}
