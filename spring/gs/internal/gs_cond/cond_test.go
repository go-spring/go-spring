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

package gs_cond

import (
	"fmt"
	"testing"

	"go-spring.org/gs-mock/gsmock"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

var (
	trueCond  = OnFunc(func(ctx gs.ConditionContext) (bool, error) { return true, nil })
	falseCond = OnFunc(func(ctx gs.ConditionContext) (bool, error) { return false, nil })
)

func TestConditionString(t *testing.T) {

	c := OnFunc(func(ctx gs.ConditionContext) (bool, error) { return false, nil })
	assert.That(t, fmt.Sprint(c)).Equal(`OnFunc(fn=gs_cond.TestConditionString.func1)`)

	c = OnProperty("a").HavingValue("123")
	assert.That(t, fmt.Sprint(c)).Equal(`OnProperty(name=a,havingValue=123)`)

	c = OnProperty("a").HavingValue("123").MatchIfMissing()
	assert.That(t, fmt.Sprint(c)).Equal(`OnProperty(name=a,havingValue=123,matchIfMissing)`)

	c = OnBean[any]("a")
	assert.That(t, fmt.Sprint(c)).Equal(`OnBean(selector={Type:any,Name:a})`)

	c = OnBean[error]()
	assert.That(t, fmt.Sprint(c)).Equal(`OnBean(selector={Type:error})`)

	c = OnMissingBean[any]("a")
	assert.That(t, fmt.Sprint(c)).Equal(`OnMissingBean(selector={Type:any,Name:a})`)

	c = OnMissingBeanID(gs.BeanIDFor[error]())
	assert.That(t, fmt.Sprint(c)).Equal(`OnMissingBean(selector={Type:error})`)

	c = OnSingleBean[any]("a")
	assert.That(t, fmt.Sprint(c)).Equal(`OnSingleBean(selector={Type:any,Name:a})`)

	c = OnSingleBeanID(gs.BeanIDFor[error]())
	assert.That(t, fmt.Sprint(c)).Equal(`OnSingleBean(selector={Type:error})`)

	c = OnExpression("a")
	assert.That(t, fmt.Sprint(c)).Equal(`OnExpression(expression=a)`)

	c = Not(OnBean[any]("a"))
	assert.That(t, fmt.Sprint(c)).Equal(`Not(OnBean(selector={Type:any,Name:a}))`)

	c = Or(OnBean[any]("a"))
	assert.That(t, fmt.Sprint(c)).Equal(`Or(OnBean(selector={Type:any,Name:a}))`)

	c = Or(OnBean[any]("a"), OnBean[any]("b"))
	assert.That(t, fmt.Sprint(c)).Equal(`Or(OnBean(selector={Type:any,Name:a}),OnBean(selector={Type:any,Name:b}))`)

	c = And(OnBean[any]("a"))
	assert.That(t, fmt.Sprint(c)).Equal(`And(OnBean(selector={Type:any,Name:a}))`)

	c = And(
		OnBeanID(gs.BeanID{Name: "a"}),
		OnBeanID(gs.BeanID{Name: "b"}),
	)
	assert.That(t, fmt.Sprint(c)).Equal(`And(OnBean(selector={Name:a}),OnBean(selector={Name:b}))`)

	c = None(OnBean[any]("a"))
	assert.That(t, fmt.Sprint(c)).Equal(`None(OnBean(selector={Type:any,Name:a}))`)

	c = None(OnBean[any]("a"), OnBean[any]("b"))
	assert.That(t, fmt.Sprint(c)).Equal(`None(OnBean(selector={Type:any,Name:a}),OnBean(selector={Type:any,Name:b}))`)

	c = And(
		OnBean[any]("a"),
		Or(
			OnBean[any]("b"),
			Not(OnBean[any]("c")),
		),
	)
	assert.That(t, fmt.Sprint(c)).Equal(`And(OnBean(selector={Type:any,Name:a}),Or(OnBean(selector={Type:any,Name:b}),Not(OnBean(selector={Type:any,Name:c}))))`)
}

func TestOnFunc(t *testing.T) {

	t.Run("nil function", func(t *testing.T) {
		assert.Panic(t, func() {
			OnFunc(nil)
		}, "condition function cannot be nil")
	})

	t.Run("success", func(t *testing.T) {
		fn := func(ctx gs.ConditionContext) (bool, error) { return true, nil }
		cond := OnFunc(fn)
		ok, err := cond.Matches(nil)
		assert.That(t, ok).True()
		assert.That(t, err).Nil()
	})

	t.Run("returns error", func(t *testing.T) {
		fn := func(ctx gs.ConditionContext) (bool, error) { return false, errutil.Explain(nil, "test error") }
		cond := OnFunc(fn)
		_, err := cond.Matches(nil)
		assert.Error(t, err).Matches("test error")
	})

	t.Run("returns false", func(t *testing.T) {
		fn := func(ctx gs.ConditionContext) (bool, error) { return false, nil }
		cond := OnFunc(fn)
		ok, err := cond.Matches(nil)
		assert.That(t, ok).False()
		assert.That(t, err).Nil()
	})
}

func TestOnProperty(t *testing.T) {

	t.Run("property exist", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockHas().ReturnValue(true)

		cond := OnProperty("test.prop")
		ok, err := cond.Matches(ctx)
		assert.That(t, ok).True()
		assert.That(t, err).Nil()
	})

	t.Run("property exist and match", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockHas().ReturnValue(true)
		ctx.MockProp().ReturnValue("42", true)

		cond := OnProperty("test.prop").HavingValue("42")
		ok, err := cond.Matches(ctx)
		assert.That(t, ok).True()
		assert.That(t, err).Nil()
	})

	t.Run("property exist but not match", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockHas().ReturnValue(true)
		ctx.MockProp().ReturnValue("42", true)

		cond := OnProperty("test.prop").HavingValue("100")
		ok, _ := cond.Matches(ctx)
		assert.That(t, ok).False()
	})

	t.Run("property not exist but MatchIfMissing", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockHas().ReturnValue(false)

		cond := OnProperty("missing.prop").MatchIfMissing()
		ok, _ := cond.Matches(ctx)
		assert.That(t, ok).True()
	})

	t.Run("property not exist without MatchIfMissing", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockHas().ReturnValue(false)

		cond := OnProperty("missing.prop")
		ok, _ := cond.Matches(ctx)
		assert.That(t, ok).False()
	})

	t.Run("expression", func(t *testing.T) {

		t.Run("number expression", func(t *testing.T) {
			m := gsmock.NewManager()
			ctx := gs.NewConditionContextMockImpl(m)
			ctx.MockHas().ReturnValue(true)
			ctx.MockProp().ReturnValue("42", true)

			cond := OnProperty("test.prop").HavingValue("expr:int($) > 40")
			ok, _ := cond.Matches(ctx)
			assert.That(t, ok).True()
		})

		t.Run("string expression", func(t *testing.T) {
			m := gsmock.NewManager()
			ctx := gs.NewConditionContextMockImpl(m)
			ctx.MockHas().ReturnValue(true)
			ctx.MockProp().ReturnValue("42", true)

			cond := OnProperty("test.prop").HavingValue(`expr:$ == "42"`)
			ok, _ := cond.Matches(ctx)
			assert.That(t, ok).True()
		})

		t.Run("invalid expression", func(t *testing.T) {
			m := gsmock.NewManager()
			ctx := gs.NewConditionContextMockImpl(m)
			ctx.MockHas().ReturnValue(true)
			ctx.MockProp().ReturnValue("42", true)

			cond := OnProperty("test.prop").HavingValue("expr:invalid syntax")
			_, err := cond.Matches(ctx)
			assert.Error(t, err).Matches("unexpected token Identifier")
		})
	})
}

func TestOnBean(t *testing.T) {

	t.Run("found bean", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue([]gs.ConditionBean{nil}, nil)

		cond := OnBean[any]("b")
		ok, err := cond.Matches(ctx)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("not found bean", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, nil)

		cond := OnBean[any]("b")
		ok, err := cond.Matches(ctx)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnBean[any]("b")
		ok, err := cond.Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestOnMissingBean(t *testing.T) {

	t.Run("not found bean", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, nil)

		cond := OnMissingBean[any]("bean1")
		ok, err := cond.Matches(ctx)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("found bean", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue([]gs.ConditionBean{nil}, nil)

		cond := OnMissingBean[any]("bean1")
		ok, err := cond.Matches(ctx)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnMissingBean[any]("b")
		ok, err := cond.Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestOnSingleBean(t *testing.T) {

	t.Run("found only one bean", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue([]gs.ConditionBean{nil}, nil)

		cond := OnSingleBean[any]("b")
		ok, _ := cond.Matches(ctx)
		assert.That(t, ok).True()
	})

	t.Run("found two beans", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue([]gs.ConditionBean{nil, nil}, nil)

		cond := OnSingleBean[any]("b")
		ok, _ := cond.Matches(ctx)
		assert.That(t, ok).False()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnSingleBean[any]("b")
		ok, err := cond.Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestOnExpression(t *testing.T) {
	m := gsmock.NewManager()
	ctx := gs.NewConditionContextMockImpl(m)

	cond := OnExpression("1+1==2")
	_, err := cond.Matches(ctx)
	assert.Error(t, err).Is(errutil.ErrUnimplementedMethod)
}

func TestNot(t *testing.T) {

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			Not(nil)
		}, "c cannot be nil")
	})

	t.Run("returns true", func(t *testing.T) {
		cond := Not(trueCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("returns false", func(t *testing.T) {
		cond := Not(falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnSingleBean[any]("b")
		ok, err := Not(cond).Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestAnd(t *testing.T) {

	t.Run("nil", func(t *testing.T) {
		cond := And()
		assert.That(t, cond).NotNil()
	})

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			And(nil)
		}, "conditions cannot contains nil")

		assert.Panic(t, func() {
			And(trueCond, nil)
		}, "conditions cannot contains nil")
	})

	t.Run("one condition", func(t *testing.T) {
		cond := And(trueCond)
		assert.That(t, cond).TypeOf(&onAnd{})
	})

	t.Run("two conditions | true", func(t *testing.T) {
		cond := And(trueCond, trueCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("two conditions | false", func(t *testing.T) {
		cond := And(trueCond, falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnSingleBean[any]("b")
		ok, err := And(cond, trueCond).Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestOr(t *testing.T) {

	t.Run("nil", func(t *testing.T) {
		cond := Or()
		assert.That(t, cond).NotNil()
	})

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			Or(nil)
		}, "conditions cannot contains nil")

		assert.Panic(t, func() {
			Or(trueCond, nil)
		}, "conditions cannot contains nil")
	})

	t.Run("one condition", func(t *testing.T) {
		cond := Or(trueCond)
		assert.That(t, cond).TypeOf(&onOr{})
	})

	t.Run("two conditions | true", func(t *testing.T) {
		cond := Or(trueCond, falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("two conditions | false", func(t *testing.T) {
		cond := Or(falseCond, falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnSingleBean[any]("b")
		ok, err := Or(cond, trueCond).Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}

func TestNone(t *testing.T) {

	t.Run("nil", func(t *testing.T) {
		cond := None()
		assert.That(t, cond).NotNil()
	})

	t.Run("nil condition", func(t *testing.T) {
		assert.Panic(t, func() {
			None(nil)
		}, "conditions cannot contains nil")

		assert.Panic(t, func() {
			None(trueCond, nil)
		}, "conditions cannot contains nil")
	})

	t.Run("one condition", func(t *testing.T) {
		cond := None(trueCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("two conditions | false", func(t *testing.T) {
		cond := None(trueCond, falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).False()
	})

	t.Run("two conditions | true", func(t *testing.T) {
		cond := None(falseCond, falseCond)
		ok, err := cond.Matches(nil)
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("returns error", func(t *testing.T) {
		m := gsmock.NewManager()
		ctx := gs.NewConditionContextMockImpl(m)
		ctx.MockFind().ReturnValue(nil, errutil.Explain(nil, "test error"))

		cond := OnSingleBean[any]("b")
		ok, err := None(cond, trueCond).Matches(ctx)
		assert.Error(t, err).Matches("test error")
		assert.That(t, ok).False()
	})
}
