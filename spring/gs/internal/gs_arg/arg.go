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

// Package gs_arg provides implementations for argument resolution and binding
// used by the Go-Spring framework.
//
// Key Features:
//   - Configuration property binding and dependency injection via struct tags.
//   - Precise positional binding through index-based arguments.
//   - Direct injection of fixed value arguments.
//   - Full support for variadic function parameters.
//   - Conditional execution with runtime evaluation.
package gs_arg

import (
	"fmt"
	"reflect"
	"runtime"

	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/typeutil"
)

// TagArg represents an argument resolved using a tag for property binding
// or dependency injection.
type TagArg struct {
	Tag string
}

// Tag creates a TagArg with the given tag string.
func Tag(tag string) gs.Arg {
	return TagArg{Tag: tag}
}

// GetArgValue resolves the tag to a value based on the target type.
// - For primitive types (int, string), it binds from configuration.
// - For structs/interfaces, it wires dependencies from the container.
// It returns an error if the type is neither bindable nor injectable.
func (arg TagArg) GetArgValue(ctx gs.ArgContext, t reflect.Type) (reflect.Value, error) {

	// Bind property values based on the argument type.
	if typeutil.IsPropBindingTarget(t) {
		if arg.Tag == "" {
			return reflect.Value{}, errutil.Explain(nil, "missing tag for property binding")
		}
		v := reflect.New(t).Elem()
		if err := ctx.Bind(v, arg.Tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	// Wire dependencies based on the argument type.
	if typeutil.IsBeanInjectionTarget(t) {
		v := reflect.New(t).Elem()
		if err := ctx.Wire(v, arg.Tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	err := errutil.Explain(nil, "unsupported argument type %s", t.String())
	return reflect.Value{}, err
}

// IndexArg represents an argument that is bound by its explicit position
// (index) in the target function’s parameter list.
type IndexArg struct {
	Idx int    // The positional index (0-based).
	Arg gs.Arg // The wrapped argument value.
}

// Index creates an IndexArg with the given index and argument.
func Index(n int, arg gs.Arg) gs.Arg {
	return IndexArg{Idx: n, Arg: arg}
}

// GetArgValue for IndexArg should never be called directly.
// IndexArg is resolved by ArgList when assembling the function’s
// argument list. If called, it panics to indicate incorrect usage.
func (arg IndexArg) GetArgValue(ctx gs.ArgContext, t reflect.Type) (reflect.Value, error) {
	panic(errutil.ErrUnimplementedMethod)
}

// ValueArg represents a constant (fixed) value argument that does not need
// any resolution or injection.
type ValueArg struct {
	v any
}

// Value creates a ValueArg from a fixed constant value.
func Value(v any) gs.Arg {
	return ValueArg{v: v}
}

// GetArgValue returns the fixed value wrapped by ValueArg.
// If the value is nil, it returns the zero value of the target type.
// If the value’s type is not assignable to the target type, it returns an error.
func (arg ValueArg) GetArgValue(ctx gs.ArgContext, t reflect.Type) (reflect.Value, error) {
	if arg.v == nil {
		return reflect.Zero(t), nil
	}
	v := reflect.ValueOf(arg.v)
	return checkAssignable(v, t)
}

func checkAssignable(v reflect.Value, t reflect.Type) (reflect.Value, error) {
	if !v.Type().AssignableTo(t) {
		err := errutil.Explain(nil, "cannot assign type %s to type %s", v.Type().String(), t.String())
		return reflect.Value{}, err
	}
	return v, nil
}

// ArgList manages a collection of arguments for a target function,
// including both fixed and variadic parameters.
//
// It supports two modes:
//   - Indexed mode: all arguments are provided with explicit positions (IndexArg).
//   - Sequential mode: arguments are provided in order (no explicit indices).
type ArgList struct {
	fnType reflect.Type // The reflected type of the target function.
	args   []gs.Arg     // The argument list (indexed or non-indexed).
}

// NewArgList validates and constructs an ArgList for the given function type.
//
// Validation checks:
//   - fnType must be a function type.
//   - Cannot mix indexed and non-indexed arguments.
//   - Index values must be within valid parameter bounds.
func NewArgList(fnType reflect.Type, args []gs.Arg) (*ArgList, error) {
	if fnType == nil {
		return nil, errutil.Explain(nil, "invalid function type <nil>")
	}
	if fnType.Kind() != reflect.Func {
		return nil, errutil.Explain(nil, "invalid function type %s", fnType.String())
	}

	// Determine number of fixed arguments.
	fixedArgCount := fnType.NumIn()
	if fnType.IsVariadic() {
		fixedArgCount--
	} else if len(args) > fixedArgCount {
		return nil, errutil.Explain(nil, "too many arguments for function %s", fnType.String())
	}

	// Initialize argument list with empty Tag() placeholders.
	fnArgs := make([]gs.Arg, fixedArgCount)
	for i := range fnArgs {
		fnArgs[i] = Tag("")
	}

	var (
		useIdx          bool
		notIdx          bool
		indexedFixedArg = make([]bool, fixedArgCount)
	)

	// Process each provided argument.
	for i := range args {
		switch arg := args[i].(type) {
		case IndexArg:
			useIdx = true
			if notIdx {
				return nil, errutil.Explain(nil, "arguments must be all indexed or non-indexed")
			}
			if arg.Idx < 0 || arg.Idx >= fnType.NumIn() {
				return nil, errutil.Explain(nil, "invalid argument index %d", arg.Idx)
			}
			if arg.Idx < fixedArgCount {
				if indexedFixedArg[arg.Idx] {
					return nil, errutil.Explain(nil, "duplicate argument index %d", arg.Idx)
				}
				indexedFixedArg[arg.Idx] = true
				fnArgs[arg.Idx] = arg.Arg
			} else {
				fnArgs = append(fnArgs, arg.Arg)
			}
		default:
			notIdx = true
			if useIdx {
				return nil, errutil.Explain(nil, "arguments must be all indexed or non-indexed")
			}
			if i < fixedArgCount {
				fnArgs[i] = arg
			} else {
				fnArgs = append(fnArgs, arg)
			}
		}
	}

	return &ArgList{fnType: fnType, args: fnArgs}, nil
}

// get resolves all arguments in the ArgList using the provided ArgContext.
// It returns a slice of reflect.Value ready for invocation of the target function.
func (r *ArgList) get(ctx gs.ArgContext) ([]reflect.Value, error) {

	fnType := r.fnType
	numIn := fnType.NumIn()
	variadic := fnType.IsVariadic()
	result := make([]reflect.Value, 0, len(r.args))

	// Processes each argument and converts it to a [reflect.Value].
	for idx, arg := range r.args {

		var t reflect.Type
		if variadic && idx >= numIn-1 {
			// For variadic parameters, use element type of the variadic slice.
			t = fnType.In(numIn - 1).Elem()
		} else {
			t = fnType.In(idx)
		}

		v, err := arg.GetArgValue(ctx, t)
		if err != nil {
			return nil, err
		}
		if v.IsValid() {
			result = append(result, v)
		}
	}
	return result, nil
}

// CallableFunc is an alias for any callable function.
type CallableFunc = any

// Callable wraps a target function together with its resolved ArgList.
// It can be invoked at runtime with the correct arguments.
type Callable struct {
	fn      CallableFunc
	argList *ArgList
}

func callableFuncType(fn CallableFunc) (reflect.Type, error) {
	if fn == nil {
		return nil, errutil.Explain(nil, "invalid function type <nil>")
	}
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()
	if fnType.Kind() != reflect.Func {
		return nil, errutil.Explain(nil, "invalid function type %s", fnType.String())
	}
	if fnValue.IsNil() {
		return nil, errutil.Explain(nil, "function cannot be nil")
	}
	return fnType, nil
}

// NewCallable creates a Callable by binding the given arguments to the function.
func NewCallable(fn CallableFunc, args []gs.Arg) (*Callable, error) {
	fnType, err := callableFuncType(fn)
	if err != nil {
		return nil, err
	}
	argList, err := NewArgList(fnType, args)
	if err != nil {
		return nil, err
	}
	return &Callable{fn: fn, argList: argList}, nil
}

// Call resolves all arguments and invokes the underlying function.
func (r *Callable) Call(ctx gs.ArgContext) ([]reflect.Value, error) {
	ret, err := r.argList.get(ctx)
	if err != nil {
		return nil, err
	}
	return reflect.ValueOf(r.fn).Call(ret), nil
}

// BindArg represents a bound function ready to be executed conditionally.
type BindArg struct {
	r          *Callable      // Wrapped Callable
	fileline   string         // File:line info for debugging
	conditions []gs.Condition // Conditions for conditional execution
}

// validBindFunc validates that a function is a proper bind target.
func validBindFunc(fn CallableFunc) error {
	t, err := callableFuncType(fn)
	if err != nil {
		return err
	}
	if numOut := t.NumOut(); numOut == 1 { // func(...) error
		if o := t.Out(0); !typeutil.IsErrorType(o) {
			return nil
		}
	} else if numOut == 2 { // func(...) (T, error)
		if o := t.Out(t.NumOut() - 1); typeutil.IsErrorType(o) {
			return nil
		}
	}
	err = errutil.Explain(nil, "expected func(...) error or func(...) (T, error)")
	return errutil.Explain(err, "invalid bind function signature")
}

// Bind creates a BindArg for a given function and its arguments.
// The function must have a valid bindable signature.
func Bind(fn CallableFunc, args ...gs.Arg) *BindArg {
	if err := validBindFunc(fn); err != nil {
		panic(err)
	}
	r, err := NewCallable(fn, args)
	if err != nil {
		panic(err)
	}
	arg := &BindArg{r: r}
	_, file, line, _ := runtime.Caller(1)
	arg.SetFileLine(file, line)
	return arg
}

// SetFileLine records the source location of the Bind() call.
func (arg *BindArg) SetFileLine(file string, line int) {
	arg.fileline = fmt.Sprintf("%s:%d", file, line)
}

// Condition appends runtime conditions that must be satisfied before execution.
func (arg *BindArg) Condition(conditions ...gs.Condition) *BindArg {
	gs_cond.ValidateConditions(conditions...)
	arg.conditions = append(arg.conditions, conditions...)
	return arg
}

// GetArgValue executes the function if all conditions are met and returns the result.
// It returns an invalid [reflect.Value] if conditions are not met. It also propagates
// errors from the function or condition checks.
func (arg *BindArg) GetArgValue(ctx gs.ArgContext, t reflect.Type) (reflect.Value, error) {

	// Evaluate all conditions.
	for _, c := range arg.conditions {
		ok, err := ctx.Check(c)
		if err != nil {
			return reflect.Value{}, err
		} else if !ok {
			return reflect.Value{}, nil
		}
	}

	// Execute the function.
	out, err := arg.r.Call(ctx)
	if err != nil {
		return reflect.Value{}, err
	}
	if len(out) == 1 { // func(...) T
		return checkAssignable(out[0], t)
	}
	err, _ = out[1].Interface().(error)
	if err != nil {
		return reflect.Value{}, err
	}
	return checkAssignable(out[0], t) // func(...) (T, error)
}
