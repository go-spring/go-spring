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

//go:generate gs mock -o=gs_mock.go -i=ConditionContext,ArgContext

package gs

import (
	"reflect"
	"strings"
)

// anyType is the [reflect.Type] of the [any] type.
var anyType = reflect.TypeFor[any]()

// As returns the [reflect.Type] of the given generic interface type T.
// It ensures that T is an interface type; otherwise, it panics.
func As[T any]() reflect.Type {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Interface {
		panic("T must be interface")
	}
	return t
}

// BeanID uniquely identifies a bean in the IoC container by type and name.
type BeanID struct {
	Type reflect.Type // The bean's type
	Name string       // The bean's name
}

// BeanIDFor creates a BeanID for a specific type T.
// If a name is provided, it will be associated with the selector;
// otherwise, only the type is used to identify the bean.
func BeanIDFor[T any](name ...string) BeanID {
	if len(name) == 0 {
		return BeanID{Type: reflect.TypeFor[T]()}
	}
	return BeanID{Type: reflect.TypeFor[T](), Name: name[0]}
}

// String returns a human-readable string representation of the selector.
// Example: "{Type:*mypkg.MyBean,Name:myBeanInstance}"
func (s BeanID) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	if s.Type != nil {
		sb.WriteString("Type:")
		if s.Type == anyType {
			sb.WriteString("any")
		} else {
			sb.WriteString(s.Type.String())
		}
	}
	if s.Name != "" {
		if sb.Len() > 1 {
			sb.WriteString(",")
		}
		sb.WriteString("Name:")
		sb.WriteString(s.Name)
	}
	sb.WriteString("}")
	return sb.String()
}

/************************************ cond ***********************************/

// ConditionBean represents a bean in the IoC container that can be queried by conditions.
type ConditionBean interface {
	GetName() string       // Returns the bean's name
	GetType() reflect.Type // Returns the bean's type
}

// ConditionContext provides access to the IoC container for conditions.
// Conditions can query properties or find beans in the container.
type ConditionContext interface {
	// Has checks if a property with the given key exists in the IoC container.
	Has(key string) bool
	// Prop retrieves a property value from the IoC container with an optional default.
	Prop(key string) (string, bool)
	// Find searches for beans that match the given BeanSelector.
	Find(beanID BeanID) ([]ConditionBean, error)
}

// Condition defines a contract for conditional bean registration.
// A Condition can decide at runtime whether a particular bean should be registered.
type Condition interface {
	// Matches evaluates the condition against the given ConditionContext.
	// It returns true if the condition is satisfied.
	Matches(ctx ConditionContext) (bool, error)
}

/************************************* arg ***********************************/

// ArgContext provides the runtime context for resolving arguments.
// It allows checking conditions, binding properties, and wiring dependencies.
type ArgContext interface {
	// Check evaluates whether a given condition is satisfied.
	Check(c Condition) (bool, error)
	// Bind binds configuration or property values into the provided [reflect.Value].
	// Used for primitive types and structs with value tags.
	Bind(v reflect.Value, tag string) error
	// Wire injects dependencies (beans) into the provided [reflect.Value].
	// Used for struct and interface type bean references.
	Wire(v reflect.Value, tag string) error
}

// Arg defines an interface for resolving arguments used in dependency injection.
// It determines how to obtain values for function or method parameters.
type Arg interface {
	// GetArgValue retrieves the argument value for the given type
	// using the provided ArgContext.
	GetArgValue(ctx ArgContext, t reflect.Type) (reflect.Value, error)
}
