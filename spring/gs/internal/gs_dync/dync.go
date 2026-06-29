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
//  1. Pre-refresh (commit=false): Validates all values against new configuration.
//     On failure, the old configuration is preserved and no changes are applied.
//  2. Commit (commit=true): Atomically applies validated configuration to all values.
package gs_dync

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// refreshable represents a dynamically refreshed configuration value.
// Only Value[T] implements this interface; keep it narrow so two-phase refresh can rely on
// pre-refresh and commit executing the same binding path without supporting generic rollback.
type refreshable interface {
	onRefresh(prop flatten.Storage, param conf.BindParam, commit bool) error
}

// Value represents a thread-safe container that stores a dynamic configuration value.
// Its value can be updated atomically via onRefresh.
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
// At runtime, calling Properties.Refresh() updates all registered Value fields atomically.
type Value[T any] struct {
	v atomic.Value
}

// Value retrieves the current value stored in the object.
// If no value is set, it returns the zero value for the type T.
func (r *Value[T]) Value() T {
	v, ok := r.v.Load().(T)
	if !ok {
		var zero T
		return zero
	}
	return v
}

// onRefresh updates the stored value with new properties if commit is true.
func (r *Value[T]) onRefresh(prop flatten.Storage, param conf.BindParam, commit bool) error {
	t := reflect.TypeFor[T]()
	v := reflect.New(t).Elem()
	if err := conf.BindValue(prop, v, t, param, nil); err != nil {
		return errutil.Explain(err, "bind dynamic value failed")
	}
	if commit {
		r.v.Store(v.Interface())
	}
	return nil
}

// MarshalJSON serializes the stored value as JSON.
func (r *Value[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.v.Load())
}

// refreshObject represents an object bound to dynamic properties that can be refreshed.
type refreshObject struct {
	target refreshable    // The refreshable object.
	param  conf.BindParam // Parameters used for refreshing.
}

// Properties manages dynamic properties and refreshable objects.
// It serves two distinct phases:
//
// 1. Initialization Phase (IOC Container Startup):
//   - RefreshField is called for each configuration-bound field
//   - Registers refreshable objects in the internal objects slice
//   - Sets initial configuration values immediately (commit=true)
//
// 2. Runtime Phase (Dynamic Configuration Updates):
//   - Refresh is called with new configuration data
//   - Executes two-phase refresh: validate all values first, then commit
//   - On validation failure, automatically restores the previous configuration
//   - Thread-safe: All operations are protected by RWMutex
type Properties struct {
	prop    flatten.Storage  // The current properties.
	lock    sync.RWMutex     // A read-write lock for thread-safe access.
	objects []*refreshObject // List of refreshable objects bound to the properties.
}

// New creates and returns a new Properties instance.
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
// values before committing them, so validation failure does not apply partial updates.
// It is thread-safe.
func (p *Properties) Refresh(prop flatten.Storage) (err error) {
	if prop == nil {
		return errutil.Explain(nil, "properties storage cannot be nil")
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	old := p.prop
	p.prop = prop
	defer func() {
		if err != nil {
			p.prop = old
		}
	}()

	if len(p.objects) == 0 {
		return nil
	}

	// First pre-refresh all dynamic values;
	// if validation passes, commit the updates.
	if err = p.refreshObjects(p.objects, false); err != nil {
		return errutil.Explain(err, "validate dynamic configuration (pre-refresh) failed")
	}
	if err = p.refreshObjects(p.objects, true); err != nil {
		return errutil.Explain(err, "apply dynamic configuration (commit) failed")
	}
	return nil
}

// Errors represents a collection of errors.
type Errors struct {
	arr []error
}

// Len returns the number of errors.
func (e *Errors) Len() int {
	return len(e.arr)
}

// Append adds an error to the collection if it is non-nil.
func (e *Errors) Append(err error) {
	if err != nil {
		e.arr = append(e.arr, err)
	}
}

// Error concatenates all errors into a single string.
func (e *Errors) Error() string {
	var sb strings.Builder
	for i, err := range e.arr {
		sb.WriteString(err.Error())
		if i < len(e.arr)-1 {
			sb.WriteString("; ")
		}
	}
	return sb.String()
}

// refreshObjects refreshes all provided objects and aggregates errors.
func (p *Properties) refreshObjects(objects []*refreshObject, commit bool) error {
	ret := &Errors{}
	for _, obj := range objects {
		err := obj.target.onRefresh(p.prop, obj.param, commit)
		if err != nil {
			ret.Append(errutil.Explain(err, "refresh dynamic object %s (key=%s) failed", obj.param.Path, obj.param.Key))
		}
	}
	if ret.Len() == 0 {
		return nil
	}
	return ret
}

// filter is used to selectively refresh objects and fields during IOC initialization.
type filter struct {
	*Properties
}

// Do attempts to refresh a single object if it implements the refreshable interface.
//
// This method is invoked by conf.BindValue during the IOC container initialization phase
// when processing struct fields with `value` tags.
//
// Note: This always uses commit=true because it's only called during initialization,
// not during runtime dynamic refreshes.
func (f *filter) Do(i any, param conf.BindParam) (bool, error) {
	v, ok := i.(refreshable)
	if !ok || v == nil {
		return false, nil
	}
	f.objects = append(f.objects, &refreshObject{
		target: v,
		param:  param,
	})
	return true, v.onRefresh(f.prop, param, true)
}

// RefreshField refreshes a field of a bean and optionally registers it as refreshable.
//
// This method is exclusively used during the IOC container initialization phase to:
//  1. Bind configuration values to struct fields
//  2. Register fields that implement refreshable for future batch refreshes
//
// Parameters:
//   - v: Reflect value of the field (must be a pointer to the actual field)
//   - param: Binding parameters including configuration key and path
//
// Note: For runtime configuration updates, use Refresh method instead.
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
