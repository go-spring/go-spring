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

package gs_core

import (
	"testing"

	"github.com/go-spring/spring-core/gs/internal/gs_bean"
	"github.com/go-spring/spring-core/gs/internal/gs_core/injecting"
	"github.com/go-spring/spring-core/gs/internal/gs_core/resolving"
	"github.com/go-spring/spring-core/gs/internal/gs_init"
	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
)

// RefreshState represents the lifecycle state of the container.
type RefreshState int

const (
	RefreshDefault = RefreshState(iota)
	Refreshing
	Refreshed
)

// Container is the core IoC container of the Go-Spring framework.
type Container struct {
	*resolving.Resolving
	*injecting.Injecting
	State RefreshState
}

// New creates a new IoC container instance.
func New() *Container {
	return &Container{
		Resolving: resolving.New(),
		State:     RefreshDefault,
	}
}

// Refresh performs the full container lifecycle startup.
// The container processes application beans in two phases:
//
//  1. Resolving phase:
//     Bean definitions are registered, configuration beans are scanned,
//     conditions are evaluated, and inactive beans are filtered out.
//
//  2. Injecting phase:
//     Dependencies are resolved and injected, and the final bean graph
//     is constructed starting from the specified root beans.
//
// After a successful refresh, resolving-phase metadata is discarded to
// reduce memory usage, and the container transitions to the Refreshed state.
//
// Parameters:
//   - p: configuration storage used for property resolution.
//   - roots: root bean definitions that act as entry points for dependency injection.
//
// Refresh should only be called once per container instance.
func (c *Container) Refresh(p flatten.Storage, roots []*gs_bean.BeanDefinition) error {
	if c.State != RefreshDefault {
		return errutil.Explain(nil, "container already refreshed")
	}
	c.State = Refreshing

	// Step 1: Resolve and prepare all bean definitions.
	if err := c.Resolving.Refresh(p); err != nil {
		return errutil.Explain(err, "container resolving error")
	}

	// Step 2: Run the injecting phase and perform dependency wiring.
	c.Injecting = injecting.New(p)
	if err := c.Injecting.Refresh(roots, c.Beans()); err != nil {
		return errutil.Explain(err, "container injecting error")
	}

	// Step 3: Clear the initialization cache.
	if !testing.Testing() {
		gs_init.Clear()
	}

	c.State = Refreshed
	c.Resolving = nil
	return nil
}
