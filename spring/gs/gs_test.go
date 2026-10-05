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

// Tests for gs.go: argument binding, condition combinators, Group, OnOnce,
// and the init-time panic guards on Module/Group.

package gs

import (
	"strings"
	"sync/atomic"
	"testing"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// BindArg stamps its own runtime.Caller(1), which inside the BindArg wrapper
// points at gs/gs.go — the wrapper must re-stamp the user's call site so
// ArgErr diagnostics point at real user code.
func TestBindArgFileLine(t *testing.T) {
	a := BindArg(func() (string, error) { return "", nil })
	s := a.String()
	assert.That(t, strings.Contains(s, "gs_test.go")).True(
		"fileline should point at this test file")
	assert.That(t, strings.Contains(s, "gs/gs.go")).False(
		"fileline should not point at the gs wrapper")
}

// condOf builds a condition that always reports ok, independent of context.
func condOf(ok bool) Condition {
	return OnFunc(func(ctx ConditionContext) (bool, error) {
		return ok, nil
	})
}

// TestConditionCombinators exercises the public Not/Or/And/None combinators.
func TestConditionCombinators(t *testing.T) {
	cases := []struct {
		name string
		cond Condition
		want bool
	}{
		{"Not inverts true", Not(condOf(true)), false},
		{"Not inverts false", Not(condOf(false)), true},
		{"Or all false", Or(condOf(false), condOf(false)), false},
		{"Or one true", Or(condOf(false), condOf(true)), true},
		{"Or all true", Or(condOf(true), condOf(true)), true},
		{"Or empty", Or(), false},
		{"And all true", And(condOf(true), condOf(true)), true},
		{"And one false", And(condOf(true), condOf(false)), false},
		{"And all false", And(condOf(false), condOf(false)), false},
		{"And empty", And(), true},
		{"None all false", None(condOf(false), condOf(false)), true},
		{"None one true", None(condOf(false), condOf(true)), false},
		{"None all true", None(condOf(true), condOf(true)), false},
		{"None empty", None(), true},
		{"nested combinators", And(condOf(true), Or(condOf(false), Not(condOf(false)))), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := c.cond.Matches(nil)
			assert.That(t, err).Nil()
			assert.That(t, ok).Equal(c.want)
		})
	}
}

// groupItemConfig is the per-entry configuration bound from a map value.
type groupItemConfig struct {
	URL string `value:"${url}"`
}

// groupItem is the bean materialized for each map entry.
type groupItem struct {
	Name string
	URL  string
}

// groupDestroyed counts destructor invocations across the app lifecycle.
var groupDestroyed atomic.Int32

func init() {
	Group("${group.items}",
		func(cp *ContextProvider, name string, cfg groupItemConfig) (*groupItem, error) {
			return &groupItem{Name: name, URL: cfg.URL}, nil
		},
		func(c *groupItem) error {
			groupDestroyed.Add(1)
			return nil
		},
	)
}

// TestGroup proves Group materializes one named bean per configuration-map
// entry (the map key becoming the bean name and reaching the constructor's
// name argument) and that the destructor runs for each bean on shutdown.
func TestGroup(t *testing.T) {
	Web(false).Configure(func(app App) {
		app.Property("group.items.a.url", "http://a.example.com")
		app.Property("group.items.b.url", "http://b.example.com")
	}).RunTest(t, func(s *struct {
		Items []*groupItem `autowire:""`
	}) {
		urls := make(map[string]string, len(s.Items))
		for _, it := range s.Items {
			urls[it.Name] = it.URL
		}
		assert.Number(t, len(urls)).Equal(2)
		assert.That(t, urls["a"]).Equal("http://a.example.com")
		assert.That(t, urls["b"]).Equal("http://b.example.com")
	})

	// RunTest has shut the app down before returning, so both destructors
	// must have run by now.
	assert.Number(t, groupDestroyed.Load()).Equal(2)
}

// valueEmbedPod is a configuration struct meant to be embedded anonymously
// in a bean rather than registered as a bean of its own.
type valueEmbedPod struct {
	Name string `value:"${pod.name:=}"`
}

type valueEmbedSvc struct {
	valueEmbedPod
}

// TestValueTagEmbedBinds proves a struct with `value` tags, embedded
// anonymously in a bean, still gets its fields bound from configuration —
// no separate bean registration needed.
func TestValueTagEmbedBinds(t *testing.T) {
	t.Setenv("GS_POD_NAME", "pod-xyz")
	RunTest(t, func(s *valueEmbedSvc) {
		assert.That(t, s.Name).Equal("pod-xyz")
	})
}

func TestOnOnce(t *testing.T) {

	t.Run("no conditions", func(t *testing.T) {
		assert.That(t, OnOnce()).NotNil()
	})

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			OnOnce(nil)
		}, "conditions cannot contain nil")
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
			return false, errutil.Explain(nil, "condition error")
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
