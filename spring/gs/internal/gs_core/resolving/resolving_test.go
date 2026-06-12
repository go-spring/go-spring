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

package resolving

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_arg"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/spring/gs/internal/gs_init"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/ordered"
	"go-spring.org/stdlib/testing/assert"
)

type Logger interface {
	Print(msg string)
}

type SimpleLogger struct{}

func (l *SimpleLogger) Print(msg string) {}

type CtxLogger interface {
	CtxPrint(ctx context.Context, msg string)
}

type ZeroLogger struct {
	Name string
}

func NewLogger(name string) Logger {
	return NewZeroLogger(name)
}

func NewZeroLogger(name string) *ZeroLogger {
	return &ZeroLogger{Name: name}
}

func (l *ZeroLogger) Print(msg string) {}

func (l *ZeroLogger) CtxPrint(ctx context.Context, msg string) {}

type ChildBean struct {
	Value int
}

func (b *ChildBean) Echo() {}

type TestBean struct {
	Value int
}

func (b *TestBean) NewChild() *ChildBean {
	return &ChildBean{b.Value}
}

func (b *TestBean) NewChildV2() (*ChildBean, error) {
	return &ChildBean{b.Value}, nil
}

func (b *TestBean) Echo() {}

func TestResolving(t *testing.T) {

	t.Run("register error when container is refreshed", func(t *testing.T) {
		r := New()
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, err).Nil()
		assert.Panic(t, func() {
			r.Provide(&gs_bean.BeanDefinition{})
		}, "container is already refreshing or refreshed")
	})

	t.Run("invalid include pattern", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Configuration(
			gs_bean.Configuration{
				Includes: []string{"*"},
			},
		)
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("error parsing regexp: missing argument to repetition operator: `*`")
	})

	t.Run("invalid exclude pattern", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Configuration(
			gs_bean.Configuration{
				Excludes: []string{"*"},
			},
		)
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("error parsing regexp: missing argument to repetition operator: `*`")
	})

	t.Run("module error", func(t *testing.T) {
		defer func() { gs_init.Clear() }()
		gs_init.AddModule(nil, func(r gs_init.BeanProvider, p flatten.Storage) error {
			return errutil.Explain(nil, "module error")
		}, "", 0)

		r := New()
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, err).NotNil()
		assert.Error(t, err).Matches("module error")
	})

	t.Run("resolve error in bean condition", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Condition(
			gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
				return false, errutil.Explain(nil, "condition error")
			}),
		)
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("failed to resolve bean .*: condition OnFunc(.*) matches error: condition error")
	})

	t.Run("resolve error with multiple conditions", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Condition(
			gs_cond.OnBean[*TestBean](),
		)
		r.Provide(&TestBean{Value: 1}).Condition(
			gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
				return false, errutil.Explain(nil, "condition error")
			}),
		)
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("failed to resolve bean .*: condition OnBean(.*) matches error")
		assert.Error(t, err).Matches("condition OnFunc(.*) matches error: condition error")
	})

	t.Run("condition not match", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Condition(
			gs_cond.OnProperty("test.property").HavingValue("true"),
		)
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, err).Nil()
		assert.That(t, len(r.Beans())).Equal(0)
	})

	t.Run("duplicate bean", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1})
		r.Provide(&TestBean{Value: 2})
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("found duplicate beans")
	})

	t.Run("duplicate bean with same name", func(t *testing.T) {
		r := New()
		r.Provide(&ZeroLogger{}).Name("a").Export(gs.As[Logger]())
		r.Provide(&SimpleLogger{}).Name("a").Export(gs.As[Logger]())
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("found duplicate beans")
	})

	t.Run("export same as interface bean type", func(t *testing.T) {
		r := New()
		r.Provide(func() Logger {
			return &SimpleLogger{}
		}).Name("logger").Export(gs.As[Logger]())
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, err).Nil()
	})

	t.Run("refresh container multiple times", func(t *testing.T) {
		r := New()
		err := r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, err).Nil()
		err = r.Refresh(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.Error(t, err).Matches("container is already refreshing or refreshed")
	})

	t.Run("configuration success", func(t *testing.T) {
		r := New()
		r.Provide(&TestBean{Value: 1}).Configuration(
			gs_bean.Configuration{
				Includes: []string{"^NewChild$"},
			},
		).Name("TestBean")

		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{}))
		err := r.Refresh(p)
		assert.That(t, err).Nil()

		var names []string
		for _, b := range r.Beans() {
			names = append(names, b.GetName())
		}
		assert.That(t, len(names)).Equal(2)
	})

	t.Run("success", func(t *testing.T) {
		defer func() { gs_init.Clear() }()
		gs_init.AddModule(nil, func(r gs_init.BeanProvider, p flatten.Storage) error {
			keys := make(map[string]struct{})
			if !p.MapKeys("logger", keys) {
				return nil
			}
			for _, name := range ordered.MapKeys(keys) {
				arg := gs_arg.Tag(fmt.Sprintf("logger.%s", name))
				r.Provide(NewZeroLogger, arg).
					Export(gs.As[Logger](), gs.As[CtxLogger]()).
					Name(name)
			}
			return nil
		}, "", 0)

		r := New()
		{
			b := r.Provide(&http.Server{}).
				Condition(gs_cond.OnBean[*http.ServeMux]())
			assert.That(t, b.GetName()).Equal("Server")
		}
		{
			b := r.Provide(http.NewServeMux).Name("ServeMux-1").
				Condition(gs_cond.OnProperty("Enable.ServeMux-1").HavingValue("true"))
			assert.That(t, b.GetName()).Equal("ServeMux-1")
		}
		{
			b := r.Provide(http.NewServeMux).Name("ServeMux-2").
				Condition(gs_cond.OnProperty("Enable.ServeMux-2").HavingValue("true"))
			assert.That(t, b.GetName()).Equal("ServeMux-2")
		}
		{
			b := r.Provide(&TestBean{Value: 1}).Configuration().Name("TestBean")
			assert.That(t, b.GetName()).Equal("TestBean")
		}
		{
			b := r.Provide(&TestBean{Value: 1}).Name("TestBean-2").
				Configuration(gs_bean.Configuration{
					Excludes: []string{"^NewChild$"},
				})
			assert.That(t, b.GetName()).Equal("TestBean-2")
		}
		{
			b := r.Provide(&TestBean{Value: 3}).Name("TestBean-3")
			r.Provide((*TestBean).NewChild, b)
		}

		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"logger": map[string]string{
				"a": "",
				"b": "",
			},
			"Enable": map[string]any{
				"ServeMux-2": true,
			},
		}))
		err := r.Refresh(p)
		assert.That(t, err).Nil()

		var names []string
		for _, b := range r.Beans() {
			names = append(names, b.GetName())
		}
		assert.That(t, names).Equal([]string{
			"Server",
			"ServeMux-2",
			"TestBean",
			"TestBean-2",
			"TestBean-3",
			"NewChild",
			"a",
			"b",
			"TestBean_NewChild",
			"TestBean_NewChildV2",
			"TestBean-2_NewChildV2",
		})
	})
}
