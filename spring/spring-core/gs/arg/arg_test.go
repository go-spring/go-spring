/*
 * Copyright 2012-2019 the original author or authors.
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

package arg_test

import (
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/golang/mock/gomock"
)

func TestBind(t *testing.T) {

	t.Run("zero argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := arg.NewMockContext(ctrl)
		fn := func() {}
		c, err := arg.Bind(fn, []arg.Arg{}, 1)
		if err != nil {
			t.Fatal(err)
		}
		values, err := c.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(values), 0)
	})

	t.Run("one value argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := arg.NewMockContext(ctrl)
		expectInt := 0
		fn := func(i int) {
			expectInt = i
		}
		c, err := arg.Bind(fn, []arg.Arg{
			arg.Value(3),
		}, 1)
		if err != nil {
			t.Fatal(err)
		}
		values, err := c.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectInt, 3)
		assert.Equal(t, len(values), 0)
	})

	t.Run("one ctx value argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := arg.NewMockContext(ctrl)
		ctx.EXPECT().Bind(gomock.Any(), "${a.b.c}").DoAndReturn(func(v, tag interface{}) error {
			v.(reflect.Value).SetInt(3)
			return nil
		})
		expectInt := 0
		fn := func(i int) {
			expectInt = i
		}
		c, err := arg.Bind(fn, []arg.Arg{
			"${a.b.c}",
		}, 1)
		if err != nil {
			t.Fatal(err)
		}
		values, err := c.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectInt, 3)
		assert.Equal(t, len(values), 0)
	})

	t.Run("one ctx named bean argument", func(t *testing.T) {
		type st struct {
			i int
		}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := arg.NewMockContext(ctrl)
		ctx.EXPECT().Wire(gomock.Any(), "a").DoAndReturn(func(v, tag interface{}) error {
			v.(reflect.Value).Set(reflect.ValueOf(&st{3}))
			return nil
		})
		expectInt := 0
		fn := func(v *st) {
			expectInt = v.i
		}
		c, err := arg.Bind(fn, []arg.Arg{
			"a",
		}, 1)
		if err != nil {
			t.Fatal(err)
		}
		values, err := c.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectInt, 3)
		assert.Equal(t, len(values), 0)
	})

	t.Run("one ctx unnamed bean argument", func(t *testing.T) {
		type st struct {
			i int
		}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctx := arg.NewMockContext(ctrl)
		ctx.EXPECT().Wire(gomock.Any(), "").DoAndReturn(func(v, tag interface{}) error {
			v.(reflect.Value).Set(reflect.ValueOf(&st{3}))
			return nil
		})
		expectInt := 0
		fn := func(v *st) {
			expectInt = v.i
		}
		c, err := arg.Bind(fn, []arg.Arg{}, 1)
		if err != nil {
			t.Fatal(err)
		}
		values, err := c.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectInt, 3)
		assert.Equal(t, len(values), 0)
	})

}
