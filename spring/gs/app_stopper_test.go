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

package gs_test

import (
	"context"
	"sync/atomic"
	"testing"

	"go-spring.org/spring/gs"
)

// failingRunner is a gs.Runner whose Run always errors, so App.Start fails after
// the IoC container is wired (and after any module setup has registered stoppers).
type failingRunner struct{}

func (failingRunner) Run(context.Context) error {
	return errFailingRunner
}

var errFailingRunner = errString("failing runner")

type errString string

func (e errString) Error() string { return string(e) }

// TestStopperFlushedOnStartFailure proves that when App.Start fails (so Run
// never reaches WaitForShutdown), the top-level defer in Run still flushes
// registered stoppers. Without that defer a stopper registered during setup
// would leak its buffered data on a failed boot.
func TestStopperFlushedOnStartFailure(t *testing.T) {
	// Run() blocks until startApp fails (failingRunner) then returns; it never
	// enters the signal-wait phase. The defer flushes the stopper registered here.
	var ran atomic.Bool
	gs.RegisterStopper("test-start-failure", func(context.Context) error {
		ran.Store(true)
		return nil
	})

	gs.Configure(func(g gs.App) {
		g.Provide(func() gs.Runner { return failingRunner{} })
	}).Run()

	if !ran.Load() {
		t.Fatal("stopper was not flushed on Start failure; the failure-path defer is missing")
	}
}
