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

package gs_init

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/gs/internal/gs"
	"github.com/go-spring/spring-core/gs/internal/gs_bean"
	"github.com/go-spring/spring-core/gs/internal/gs_cond"
	"github.com/go-spring/stdlib/flatten"
)

var (
	// modules stores all registered module functions.
	// Modules are used to batch-register beans, enabling a starter mechanism
	// similar to Java Spring's auto-configuration.
	modules []Module

	// beans stores all globally registered bean definitions.
	// In go-spring, beans are divided into two categories:
	//   1. Global beans: registered in the current package (gs_init), accessible across the application
	//   2. App/IOC beans: registered within specific App or IOC container instances
	// This slice maintains the global bean definitions that can be shared and reused.
	beans []*gs_bean.BeanDefinition
)

// BeanProvider defines the API for registering beans in the IoC container.
// It serves as the abstraction for bean registration, allowing different
// container implementations to accept bean definitions uniformly.
type BeanProvider interface {
	Provide(objOrCtor any, args ...gs.Arg) *gs_bean.BeanDefinition
}

// ModuleFunc defines the signature of a module function that registers
// beans using a BeanProvider. The function receives a set of configuration
// properties as input.
//
// ModuleFunc is the core mechanism for implementing starter-like functionality:
// - A single ModuleFunc can register multiple related beans atomically
// - This enables auto-configuration patterns similar to Java Spring Boot starters
// - Modules can be conditionally activated based on configuration properties
type ModuleFunc func(r BeanProvider, p flatten.Storage) error

// Module represents a conditional module that can register beans
// when its Condition is satisfied.
type Module struct {
	ModuleFunc ModuleFunc
	Condition  gs.Condition
	FileLine   string
}

// Modules returns all registered modules.
func Modules() []Module {
	return modules
}

// Beans returns all registered bean definitions.
//
// In test mode, it returns cloned BeanDefinitions
// to avoid mutating shared bean definitions during test execution.
func Beans() []*gs_bean.BeanDefinition {
	if !testing.Testing() {
		return beans
	}
	var ret []*gs_bean.BeanDefinition
	for _, b := range beans {
		ret = append(ret, b.Clone())
	}
	return ret
}

// AddBean registers a new bean definition in the global registry.
// This function adds beans to the global scope (package-level), making them
// available for all App/IOC containers that reference this global registry.
func AddBean(bean *gs_bean.BeanDefinition) {
	beans = append(beans, bean)
}

// AddModule registers a conditional module in the global registry.
// This is called by starter/autoconfiguration code to register a module that
// should only activate when its condition matches the application configuration.
func AddModule(c gs_cond.PropertyCondition, fn ModuleFunc, file string, line int) {
	modules = append(modules, Module{
		ModuleFunc: fn,
		Condition:  c,
		FileLine:   fmt.Sprintf("%s:%d", file, line),
	})
}

// Clear resets all registered beans and modules, effectively emptying
// the global registry. This function is primarily used for testing purposes
// to ensure test isolation by clearing the global state between test runs.
func Clear() {
	beans = nil
	modules = nil
}
