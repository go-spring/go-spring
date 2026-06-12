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

package gs

import (
	"errors"
	"reflect"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

type runTestTarget struct{}

func TestOnOnce(t *testing.T) {

	t.Run("no conditions", func(t *testing.T) {
		assert.That(t, OnOnce()).Nil()
	})

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			OnOnce(nil)
		}, "condition cannot be nil")
	})

	t.Run("evaluated once", func(t *testing.T) {
		count := 0
		cond := OnOnce(OnFunc(func(ctx ConditionContext) (bool, error) {
			count++
			return true, nil
		}))

		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()

		ok, err = cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
		assert.That(t, count).Equal(1)
	})

	t.Run("caches error", func(t *testing.T) {
		count := 0
		cond := OnOnce(OnFunc(func(ctx ConditionContext) (bool, error) {
			count++
			return false, errors.New("condition error")
		}))

		ok, err := cond.Matches(nil)
		assert.Error(t, err).Matches("condition error")
		assert.That(t, ok).False()

		ok, err = cond.Matches(nil)
		assert.Error(t, err).Matches("condition error")
		assert.That(t, ok).False()
		assert.That(t, count).Equal(1)
	})
}

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
				err:  `RunTest requires func\(\*TestStruct\), got <nil>`,
			},
			{
				name: "non function",
				fn:   1,
				err:  `RunTest requires func\(\*TestStruct\), got int`,
			},
			{
				name: "nil function",
				fn:   nilFn,
				err:  `RunTest requires non-nil func\(\*TestStruct\)`,
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

func TestModuleNilFunction(t *testing.T) {
	oldInited := inited
	inited = false
	defer func() {
		inited = oldInited
	}()

	assert.Panic(t, func() {
		Module(nil, nil)
	}, "gs.Module function cannot be nil")
}

func TestGroupNilFunction(t *testing.T) {
	oldInited := inited
	inited = false
	defer func() {
		inited = oldInited
	}()

	assert.Panic(t, func() {
		Group[runTestTarget, *runTestTarget]("${items}", nil, nil)
	}, "gs.Group function cannot be nil")
}
