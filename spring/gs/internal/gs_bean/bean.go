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

// Package gs_bean provides core bean management for Go-Spring framework.
package gs_bean

import (
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_arg"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/typeutil"
)

// BeanStatus represents the different lifecycle statuses of a bean.
type BeanStatus int8

const (
	StatusDeleted   = BeanStatus(-1)   // Bean has been deleted.
	StatusDefault   = BeanStatus(iota) // Default status of the bean.
	StatusResolving                    // Bean is being resolved.
	StatusResolved                     // Bean has been resolved.
	StatusCreating                     // Bean is being created.
	StatusCreated                      // Bean has been created.
	StatusWired                        // Bean has been wired.
)

// String returns a human-readable string for the bean status.
func (status BeanStatus) String() string {
	switch status {
	case StatusDeleted:
		return "deleted"
	case StatusDefault:
		return "default"
	case StatusResolving:
		return "resolving"
	case StatusResolved:
		return "resolved"
	case StatusCreating:
		return "creating"
	case StatusCreated:
		return "created"
	case StatusWired:
		return "wired"
	default:
		return "unknown"
	}
}

// Configuration specifies parameters for configuring beans during registration.
type Configuration struct {
	Includes []string // Methods to include
	Excludes []string // Methods to exclude
}

// BeanDefinition contains both metadata and runtime information of a bean.
type BeanDefinition struct {
	v             reflect.Value    // The value of the bean.
	t             reflect.Type     // The type of the bean.
	f             *gs_arg.Callable // Callable for constructor functions
	name          string           // The name of the bean.
	init          any              // Bean initialization function
	destroy       any              // Bean destruction function
	dependsOn     []gs.BeanID      // Explicit dependencies of the bean
	exports       []reflect.Type   // Interfaces exported by this bean
	conditions    []gs.Condition   // Conditions controlling bean creation
	status        BeanStatus       // Current lifecycle status
	fileLine      string           // File and line where bean is defined
	configuration *Configuration   // Configuration for sub/child beans
}

// Clone creates a copy of the BeanDefinition.
// For pointer beans, a new instance of the underlying type is created.
// For function beans, the value is shared (functions are immutable in Go).
// This ensures the cloned BeanDefinition has a separate reflect.Value when necessary.
func (d *BeanDefinition) Clone() *BeanDefinition {
	r := *d
	if d.f != nil { // Constructor
		r.v = reflect.New(d.t).Elem()
		return &r
	}
	if d.t.Kind() == reflect.Func { // Function
		return &r
	}
	r.v = reflect.New(d.t.Elem())
	return &r
}

// BeanID returns the bean's identifier.
func (d *BeanDefinition) BeanID() gs.BeanID {
	return gs.BeanID{Name: d.GetName(), Type: d.GetType()}
}

// GetName returns the bean's name.
func (d *BeanDefinition) GetName() string {
	return d.name
}

// GetType returns the bean's type.
func (d *BeanDefinition) GetType() reflect.Type {
	return d.t
}

// GetValue returns the bean as reflect.Value.
func (d *BeanDefinition) GetValue() reflect.Value {
	return d.v
}

// Interface returns the underlying bean.
func (d *BeanDefinition) Interface() any {
	return d.v.Interface()
}

// Callable returns the bean's callable constructor.
func (d *BeanDefinition) Callable() *gs_arg.Callable {
	return d.f
}

// GetStatus returns the bean's current lifecycle status.
func (d *BeanDefinition) GetStatus() BeanStatus {
	return d.status
}

// GetInit returns the bean's initialization function.
func (d *BeanDefinition) GetInit() any {
	return d.init
}

// GetDestroy returns the bean's destruction function.
func (d *BeanDefinition) GetDestroy() any {
	return d.destroy
}

// FileLine returns the source file and line number of the bean.
func (d *BeanDefinition) FileLine() string {
	return d.fileLine
}

// Conditions returns the list of conditions for the bean.
func (d *BeanDefinition) Conditions() []gs.Condition {
	return d.conditions
}

// GetDependsOn returns the list of dependencies for the bean.
func (d *BeanDefinition) GetDependsOn() []gs.BeanID {
	return d.dependsOn
}

// GetExports returns the interfaces exported by the bean.
func (d *BeanDefinition) GetExports() []reflect.Type {
	return d.exports
}

// GetArgValue returns the bean’s value for argument injection.
func (d *BeanDefinition) GetArgValue(_ gs.ArgContext, t reflect.Type) (reflect.Value, error) {
	v := d.GetValue()
	if !v.Type().AssignableTo(t) {
		err := errutil.Explain(nil, "cannot assign type %s to type %s", v.Type().String(), t.String())
		return reflect.Value{}, err
	}
	return v, nil
}

// GetConfiguration returns the configuration for the bean.
func (d *BeanDefinition) GetConfiguration() *Configuration {
	return d.configuration
}

// Name sets the bean's name.
func (d *BeanDefinition) Name(name string) *BeanDefinition {
	d.name = name
	return d
}

// validLifeCycleFunc checks if the given function is a valid lifecycle function.
// Valid lifecycle function signature: func(bean) or func(bean) error
func validLifeCycleFunc(fn any, beanType reflect.Type) {
	if fn == nil {
		panic("lifecycle function cannot be nil")
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic(fmt.Sprintf("invalid lifecycle function: got %T, want func(bean) or func(bean) error", fn))
	}
	if v.IsNil() {
		panic("lifecycle function cannot be nil")
	}
	fnType := v.Type()
	if !typeutil.IsFuncType(fnType) || fnType.NumIn() != 1 {
		panic(fmt.Sprintf("invalid lifecycle function: got %T, want func(bean) or func(bean) error", fn))
	}
	if t := fnType.In(0); t.Kind() == reflect.Interface {
		if !beanType.Implements(t) {
			panic(fmt.Sprintf("invalid lifecycle function: got %T, want func(bean) or func(bean) error", fn))
		}
	} else if t != beanType {
		panic(fmt.Sprintf("invalid lifecycle function: got %T, want func(bean) or func(bean) error", fn))
	}
	if !typeutil.ReturnNothing(fnType) && !typeutil.ReturnOnlyError(fnType) {
		panic(fmt.Sprintf("invalid lifecycle function: got %T, want func(bean) or func(bean) error", fn))
	}
}

// Init sets the bean's initialization function.
// The init function is called after the bean is created and all dependencies are injected.
// Valid signatures: func(bean) or func(bean) error.
func (d *BeanDefinition) Init(fn any) *BeanDefinition {
	validLifeCycleFunc(fn, d.GetType())
	d.init = fn
	return d
}

// Destroy sets the bean's destruction function.
// The destroy function is called before the bean is destroyed.
// Valid signatures: func(bean) or func(bean) error.
func (d *BeanDefinition) Destroy(fn any) *BeanDefinition {
	validLifeCycleFunc(fn, d.GetType())
	d.destroy = fn
	return d
}

// InitMethod sets the bean's initialization method by name.
// The method must have signature: func() or func() error.
func (d *BeanDefinition) InitMethod(method string) *BeanDefinition {
	m, ok := d.t.MethodByName(method)
	if !ok {
		panic(fmt.Sprintf("method %s not found on type %s", method, d.t))
	}
	return d.Init(m.Func.Interface())
}

// DestroyMethod sets the bean's destruction method by name.
// The method must have signature: func() or func() error.
func (d *BeanDefinition) DestroyMethod(method string) *BeanDefinition {
	m, ok := d.t.MethodByName(method)
	if !ok {
		panic(fmt.Sprintf("method %s not found on type %s", method, d.t))
	}
	return d.Destroy(m.Func.Interface())
}

// DependsOn adds dependencies to the bean.
func (d *BeanDefinition) DependsOn(selectors ...gs.BeanID) *BeanDefinition {
	d.dependsOn = append(d.dependsOn, selectors...)
	return d
}

// Export registers interfaces exported by the bean.
func (d *BeanDefinition) Export(exports ...reflect.Type) *BeanDefinition {
	for _, t := range exports {
		if t == nil {
			panic("export type cannot be nil")
		}
		if t.Kind() != reflect.Interface {
			panic(fmt.Sprintf("export failed: %v is not an interface type", t))
		}
		if !d.GetType().Implements(t) {
			panic(fmt.Sprintf("export failed: %v does not implement interface %v", d.t, t))
		}
		if t == d.GetType() || slices.Contains(d.exports, t) {
			continue
		}
		d.exports = append(d.exports, t)
	}
	return d
}

func checkConditions(conditions []gs.Condition) {
	for _, c := range conditions {
		if c == nil {
			panic("conditions cannot contains nil")
		}
	}
}

// Condition appends conditions for the bean.
func (d *BeanDefinition) Condition(conditions ...gs.Condition) *BeanDefinition {
	checkConditions(conditions)
	d.conditions = append(d.conditions, conditions...)
	return d
}

// OnProfiles adds a creation condition based on active profiles.
// The bean will only be created if the application's "spring.profiles.active"
// property contains at least one of the specified profiles.
// Prefix with "!" to negate (e.g., "!prod" means "not production").
//
// Example:
//
//	d.OnProfiles("dev", "test")   // bean created if active profile is "dev" or "test"
//	d.OnProfiles("!prod")         // bean created if active profile is NOT "prod"
//	d.OnProfiles("dev", "!cloud") // bean created if "dev" active OR "cloud" NOT active
func (d *BeanDefinition) OnProfiles(profiles ...string) *BeanDefinition {
	if len(profiles) == 0 {
		panic("OnProfiles requires at least one profile")
	}
	for i, p := range profiles {
		p = strings.TrimSpace(p)
		if p == "" || p == "!" {
			panic(fmt.Sprintf("invalid profile at index %d: %q", i, profiles[i]))
		}
		profiles[i] = p
	}
	d.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
		val, ok := ctx.Prop("spring.profiles.active")
		if val = strings.TrimSpace(val); !ok || val == "" {
			return false, nil
		}

		active := make(map[string]bool)
		for s := range strings.SplitSeq(val, ",") {
			active[strings.TrimSpace(s)] = true
		}

		for _, p := range profiles {
			if strings.HasPrefix(p, "!") {
				if !active[p[1:]] {
					return true, nil
				}
			} else if active[p] {
				return true, nil
			}
		}
		return false, nil
	}))
	return d
}

// Configuration sets configuration (include/exclude) for the bean.
func (d *BeanDefinition) Configuration(c ...Configuration) *BeanDefinition {
	var cfg Configuration
	if len(c) > 0 {
		cfg = c[0]
	}
	d.configuration = &Configuration{
		Includes: cfg.Includes,
		Excludes: cfg.Excludes,
	}
	return d
}

// SetStatus sets the bean's current lifecycle status.
func (d *BeanDefinition) SetStatus(status BeanStatus) {
	d.status = status
}

// SetFileLine sets the source file and line number of the bean.
func (d *BeanDefinition) SetFileLine(file string, line int) {
	d.fileLine = fmt.Sprintf("%s:%d", file, line)
}

// Caller records the source file and line number of the bean.
func (d *BeanDefinition) Caller(skip int) *BeanDefinition {
	_, file, line, _ := runtime.Caller(skip)
	d.SetFileLine(file, line)
	return d
}

// String returns a human-readable description of the bean.
func (d *BeanDefinition) String() string {
	return fmt.Sprintf("name=%s %s", d.name, d.fileLine)
}

// NewBean creates a new BeanDefinition.
//
// Parameters:
//   - objOrCtor: either a bean instance (struct pointer) or a constructor function
//   - ctorArgs: optional arguments for constructor binding, ignored if objOrCtor is not a function
//
// Returns:
//   - *BeanDefinition: the created bean definition with metadata and lifecycle information
//
// Panics:
//   - if objOrCtor is not a reference type
//   - if objOrCtor is nil
//   - if constructor function has invalid signature (should be func(...)bean or func(...)(bean, error))
//   - if method constructor receives invalid arguments
//
// Examples:
//
//	// Create bean from object instance
//	bean := NewBean(&MyService{})
//
//	// Create bean from constructor function
//	bean := NewBean(NewMyService)
//
//	// Create bean from method with owner dependency
//	parent := NewBean(&Parent{})
//	child := NewBean((*Parent).CreateChild, parent)
func NewBean(objOrCtor any, ctorArgs ...gs.Arg) *BeanDefinition {

	var f *gs_arg.Callable
	var v reflect.Value
	var fromValue bool
	var name string
	var cond gs.Condition

	switch i := objOrCtor.(type) {
	case reflect.Value:
		fromValue = true
		v = i
	default:
		v = reflect.ValueOf(i)
	}

	// Ensure the bean instance is valid before reading its type.
	if !v.IsValid() {
		panic("bean instance cannot be nil")
	}

	t := v.Type()
	if !typeutil.IsBeanType(t) {
		panic(fmt.Sprintf("bean must be reference type, got %v", t))
	}

	// Ensure the bean instance is valid and not nil
	if v.IsNil() {
		panic("bean instance cannot be nil")
	}

	// Handle constructor functions
	if !fromValue && t.Kind() == reflect.Func {

		if !typeutil.IsConstructor(t) {
			t1 := "func(...)bean"
			t2 := "func(...)(bean, error)"
			panic(fmt.Sprintf("constructor should be %s or %s", t1, t2))
		}

		// Bind constructor arguments
		var err error
		f, err = gs_arg.NewCallable(objOrCtor, ctorArgs)
		if err != nil {
			panic(err)
		}

		var in0 reflect.Type
		if t.NumIn() > 0 {
			in0 = t.In(0)
		}

		// Prepare the return type
		out0 := t.Out(0)
		v = reflect.New(out0)
		if typeutil.IsBeanType(out0) {
			v = v.Elem()
		}

		t = v.Type()
		if !typeutil.IsBeanType(t) {
			panic(fmt.Sprintf("constructor return type must be reference type, got %v", t))
		}

		// Derive bean name from constructor function name
		fnPtr := reflect.ValueOf(objOrCtor).Pointer()
		fnInfo := runtime.FuncForPC(fnPtr)
		funcName := fnInfo.Name()
		name = funcName[strings.LastIndex(funcName, "/")+1:]
		name = name[strings.Index(name, ".")+1:]
		if name[0] == '(' {
			name = name[strings.Index(name, ".")+1:]
		}

		// If the constructor is a method, set a condition for its owner bean
		method := strings.LastIndexByte(fnInfo.Name(), ')') > 0
		if method {
			var s = gs.BeanID{Type: in0}
			if len(ctorArgs) > 0 {
				switch a := ctorArgs[0].(type) {
				case *BeanDefinition:
					s = gs.BeanID{Type: a.t, Name: a.name}
				case gs_arg.IndexArg:
					if a.Idx == 0 {
						switch x := a.Arg.(type) {
						case *BeanDefinition:
							s = gs.BeanID{Type: x.t, Name: x.name}
						default:
							panic("IndexArg[0] must contain a *BeanDefinition")
						}
					}
				default:
					panic("first constructor argument must be *BeanDefinition or IndexArg[0]")
				}
			}
			cond = gs_cond.OnBeanID(s)
		}
	}

	// Fallback: derive name from the type
	if name == "" {
		s := strings.Split(t.String(), ".")
		name = strings.TrimPrefix(s[len(s)-1], "*")
	}

	d := &BeanDefinition{f: f, t: t, v: v, name: name, status: StatusDefault}
	if cond != nil {
		d.Condition(cond)
	}
	return d
}
