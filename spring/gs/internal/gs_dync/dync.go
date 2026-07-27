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

// Package gs_dync provides dynamic configuration binding and refresh
// capabilities for Go-Spring applications.
//
// It enables hot-reload of configuration in long-running applications through
// a two-phase commit mechanism that ensures system consistency. Components register
// themselves during IOC container initialization and can be batch-refreshed at runtime.
//
// Two-phase refresh:
//  1. Validate (onValid): Bind every value against the new configuration without
//     applying it. On failure, the old configuration is preserved and no changes
//     are applied.
//  2. Commit (onCommit): Atomically apply the validated values.
//
// After commit, onFinish fires the registered change listeners with the previous value.
package gs_dync

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// refreshable represents a dynamically refreshed configuration value.
// Only Value[T] implements this interface; keep it narrow so two-phase refresh can
// rely on validate and commit executing the same binding path without supporting
// generic rollback.
type refreshable interface {
	onValid(prop flatten.Storage, param conf.BindParam) (newVal any, err error)
	onCommit(newVal any) (oldVal any)
	onFinish(newVal, oldVal any)
}

// Value represents a thread-safe container that stores a dynamic configuration value.
// Its value is updated atomically through the two-phase refresh driven by Properties.
//
// Key features:
//   - Type-safe: Generic type parameter ensures compile-time type safety.
//   - Atomic access: Uses atomic.Value for lock-free concurrent reads and writes.
//   - JSON serializable: Implements json.Marshaler for easy debugging and monitoring.
//   - Zero-value safe: Returns zero value when no configuration has been set yet.
//
// Typical usage:
//
//	type Config struct {
//	    Timeout gs_dync.Value[time.Duration] `value:"${server.timeout:=30s}"`
//	}
//
// During IOC initialization, the field is bound to configuration.
// At runtime, calling Properties.Refresh updates all registered Value fields atomically.
type Value[T any] struct {
	v        atomic.Value // T
	listener atomic.Value // func(newVal, oldVal T)
}

// Value returns the current value, or the zero value of T if none has been set.
func (r *Value[T]) Value() T {
	v, ok := r.v.Load().(T)
	if !ok {
		var zero T
		return zero
	}
	return v
}

// OnChanged registers a listener invoked with the new and previous values after a
// refresh commits. At most one listener is supported; calling OnChanged again when
// a listener is already registered is a no-op (TODO: log a warning in that case).
func (r *Value[T]) OnChanged(l func(newVal, oldVal T)) {
	prev, ok := r.listener.Load().(func(newVal, oldVal T))
	if !ok || prev == nil {
		r.listener.Store(l)
		return
	}
	// TODO: log a warning; a listener is already registered and will not be overwritten.
}

// onValid binds a new value from prop without applying it. The returned newVal is
// committed later by onCommit.
func (r *Value[T]) onValid(prop flatten.Storage, param conf.BindParam) (newVal any, err error) {
	t := reflect.TypeFor[T]()
	v := reflect.New(t).Elem()
	if err := conf.BindValue(prop, v, t, param, nil); err != nil {
		return nil, errutil.Explain(err, "bind dynamic value failed")
	}
	return v.Interface(), nil
}

// onCommit atomically stores newVal and returns the value it replaced.
func (r *Value[T]) onCommit(newVal any) (oldVal any) {
	return r.v.Swap(newVal)
}

// onFinish fires the registered change listener with the new and previous values, if any.
func (r *Value[T]) onFinish(newVal, oldVal any) {
	l, ok := r.listener.Load().(func(newVal, oldVal T))
	if !ok || l == nil {
		return
	}
	l(newVal.(T), oldVal.(T))
}

// MarshalJSON serializes the stored value as JSON.
func (r *Value[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.v.Load())
}

// refreshObject pairs a refreshable value with the binding parameters used to refresh it.
type refreshObject struct {
	target refreshable    // the value being refreshed
	param  conf.BindParam // binding parameters (key/path) used to resolve the value
}

// Properties manages dynamic properties and the refreshable values bound to them.
// It serves two distinct phases:
//
// 1. Initialization (IOC container startup):
//   - RefreshField is called for each configuration-bound field
//   - Registers refreshable fields for later batch refresh
//   - Binds and commits initial values immediately
//
// 2. Runtime (dynamic configuration updates):
//   - Refresh is called with new configuration data
//   - Executes two-phase refresh: validate all values first, then commit
//   - On validation failure, automatically restores the previous configuration
//   - Thread-safe: all operations are protected by a RWMutex
type Properties struct {
	prop    flatten.Storage  // current property source
	lock    sync.RWMutex     // guards prop and objects
	objects []*refreshObject // refreshable values registered during IOC init
}

// New creates and returns a new Properties instance backed by p.
func New(p flatten.Storage) *Properties {
	return &Properties{
		prop: p,
	}
}

// Data returns the current properties.
func (p *Properties) Data() flatten.Storage {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.prop
}

// ObjectsCount returns the number of registered refreshable objects.
func (p *Properties) ObjectsCount() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return len(p.objects)
}

// Refresh updates the properties and refreshes all bound values using a two-phase commit.
//
// This method is designed for runtime dynamic configuration updates. It validates all
// values before committing them, so a validation failure applies no partial updates.
// It is thread-safe.
func (p *Properties) Refresh(prop flatten.Storage) (err error) {
	if prop == nil {
		return errutil.Explain(nil, "properties storage cannot be nil")
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	oldProp := p.prop
	p.prop = prop
	defer func() {
		if err != nil {
			p.prop = oldProp
		}
	}()

	if len(p.objects) == 0 {
		return nil
	}

	log.Debugf(context.Background(), log.TagAppDef, "refreshing %d dynamic objects", len(p.objects))

	newValues, err := p.onValid(p.objects)
	if err != nil {
		return errutil.Explain(err, "validate dynamic configuration failed")
	}

	oldValues := make([]any, 0, len(p.objects))
	for i, obj := range p.objects {
		oldVal := obj.target.onCommit(newValues[i])
		oldValues = append(oldValues, oldVal)
	}

	for i, obj := range p.objects {
		obj.target.onFinish(newValues[i], oldValues[i])
	}

	log.Debugf(context.Background(), log.TagAppDef, "dynamic objects refreshed successfully")
	return nil
}

// Errors collects multiple errors and renders them as a single message.
type Errors struct {
	errs []error
}

// Len returns the number of collected errors.
func (e *Errors) Len() int {
	return len(e.errs)
}

// Append adds err to the collection if it is non-nil.
func (e *Errors) Append(err error) {
	if err != nil {
		e.errs = append(e.errs, err)
	}
}

// Error concatenates all collected errors, separated by "; ".
func (e *Errors) Error() string {
	var sb strings.Builder
	for i, err := range e.errs {
		sb.WriteString(err.Error())
		if i < len(e.errs)-1 {
			sb.WriteString("; ")
		}
	}
	return sb.String()
}

// onValid runs the validate phase for every object and collects the resulting new
// values. Errors are aggregated rather than short-circuited so that all binding
// problems are reported in a single pass.
func (p *Properties) onValid(objects []*refreshObject) (newValues []any, _ error) {
	retErr := &Errors{}
	newValues = make([]any, 0, len(objects))
	for _, obj := range objects {
		newVal, err := obj.target.onValid(p.prop, obj.param)
		if err != nil {
			retErr.Append(errutil.Explain(err, "refresh dynamic object %s (key=%s) failed", obj.param.Path, obj.param.Key))
		}
		newValues = append(newValues, newVal)
	}
	if retErr.Len() == 0 {
		return newValues, nil
	}
	return newValues, retErr
}

// filter implements conf.Filter to intercept refreshable values encountered while
// binding a struct during IOC initialization.
type filter struct {
	*Properties
}

// Do attempts to bind i as a refreshable value. If i is refreshable, it validates and
// commits the value immediately and registers it for later batch refresh; otherwise it
// lets the caller fall back to normal binding.
//
// This method is invoked by conf.BindValue while processing struct fields with `value`
// tags. It commits immediately because it runs only during initialization, not during
// runtime refresh - runtime refreshes go through Properties.Refresh.
func (f *filter) Do(i any, param conf.BindParam) (bool, error) {
	v, ok := i.(refreshable)
	if !ok || v == nil {
		return false, nil
	}
	newVal, err := v.onValid(f.prop, param)
	if err != nil {
		return true, err
	}
	v.onCommit(newVal)
	f.objects = append(f.objects, &refreshObject{
		target: v,
		param:  param,
	})
	return true, nil
}

// RefreshField binds a configuration value to v and, when v is (or contains) a
// refreshable field, registers it for future batch refreshes.
//
// This method is used exclusively during IOC container initialization to:
//  1. Bind configuration values to struct fields.
//  2. Register fields that implement refreshable for later Refresh calls.
//
// Parameters:
//   - v: Reflect value of the field (must be a pointer to the target field).
//   - param: Binding parameters, including the configuration key and path.
//
// For runtime configuration updates, use Refresh instead.
func (p *Properties) RefreshField(v reflect.Value, param conf.BindParam) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	f := &filter{Properties: p}
	if v.Kind() == reflect.Pointer {
		ok, err := f.Do(v.Interface(), param)
		if err != nil {
			return errutil.Explain(err, "refresh dynamic field %s (key=%s) failed", param.Path, param.Key)
		}
		if ok {
			return nil
		}
	}
	if err := conf.BindValue(p.prop, v.Elem(), v.Elem().Type(), param, f); err != nil {
		return errutil.Explain(err, "refresh dynamic field %s (key=%s) failed", param.Path, param.Key)
	}
	return nil
}
