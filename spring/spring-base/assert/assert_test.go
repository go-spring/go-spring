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

package assert_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/golang/mock/gomock"
)

func Case(t *testing.T, f func(g *assert.MockT)) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	g := assert.NewMockT(ctrl)
	g.EXPECT().Helper().AnyTimes()
	f(g)
}

func TestTrue(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.True(g, true)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got false but expect true"})
		assert.True(g, false)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got false but expect true; param (index=0)"})
		assert.True(g, false, "param (index=0)")
	})
}

func TestFalse(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.False(g, false)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got true but expect false"})
		assert.False(g, true)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got true but expect false; param (index=0)"})
		assert.False(g, true, "param (index=0)")
	})
}

func TestNil(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Nil(g, nil)
	})

	Case(t, func(g *assert.MockT) {
		assert.Nil(g, (*int)(nil))
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 3 but expect nil"})
		assert.Nil(g, 3)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 3 but expect nil; param (index=0)"})
		assert.Nil(g, 3, "param (index=0)")
	})
}

func TestNotNil(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.NotNil(g, 3)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got nil but expect not nil"})
		assert.NotNil(g, nil)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got nil but expect not nil; param (index=0)"})
		assert.NotNil(g, nil, "param (index=0)")
	})
}

func TestEqual(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Equal(g, 0, 0)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0"})
		assert.Equal(g, 0, "0")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		assert.Equal(g, 0, "0", "param (index=0)")
	})
}

func TestNotEqual(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.NotEqual(g, "0", 0)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0"})
		assert.NotEqual(g, "0", "0")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0; param (index=0)"})
		assert.NotEqual(g, "0", "0", "param (index=0)")
	})
}

func TestSame(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Same(g, "0", "0")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0"})
		assert.Same(g, 0, "0")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		assert.Same(g, 0, "0", "param (index=0)")
	})
}

func TestNotSame(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.NotSame(g, "0", 0)
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0"})
		assert.NotSame(g, "0", "0")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0; param (index=0)"})
		assert.NotSame(g, "0", "0", "param (index=0)")
	})
}

func TestPanic(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Panic(g, func() { panic("this is an error") }, "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"did not panic"})
		assert.Panic(g, func() {}, "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Panic(g, func() { panic("this is an error") }, "an error \\")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Panic(g, func() { panic("there's no error") }, "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Panic(g, func() { panic("there's no error") }, "an error", "param (index=0)")
	})
}

func TestMatches(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Matches(g, "this is an error", "this is an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Matches(g, "this is an error", "an error \\")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Matches(g, "there's no error", "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Matches(g, "there's no error", "an error", "param (index=0)")
	})
}

func TestError(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Error(g, errors.New("this is an error"), "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Error(g, errors.New("there's no error"), "an error \\")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not nil error"})
		assert.Error(g, nil, "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not nil error; param (index=0)"})
		assert.Error(g, nil, "an error", "param (index=0)")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Error(g, errors.New("there's no error"), "an error")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Error(g, errors.New("there's no error"), "an error", "param (index=0)")
	})
}

func TestTypeOf(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.TypeOf(g, new(int), (*int)(nil))
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got type (string) but expect type (fmt.Stringer)"})
		assert.TypeOf(g, "string", (*fmt.Stringer)(nil))
	})
}

func TestImplements(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.Implements(g, errors.New("error"), (*error)(nil))
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect should be interface"})
		assert.Implements(g, new(int), (*int)(nil))
	})
}
