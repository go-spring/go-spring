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

func TestCheck(t *testing.T) {

	var r struct {
		True  bool
		False bool
		Nil   interface{}
	}

	err := assert.Check(assert.Cases{
		{r.True, "r.True want true but is false"},
		{!r.False, "r.False want false but is true"},
		{r.Nil == nil, "r.Nil want nil but not nil"},
	})

	assert.Error(t, err, "r.True want true but is false")
}

func TestTrue(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.True(g, true)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got false but expect true"})
		g.EXPECT().Fail()
		assert.True(g, false)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got false but expect true; param (index=0)"})
		g.EXPECT().Fail()
		assert.True(g, false, "param (index=0)")
	}
}

func TestFalse(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.False(g, false)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got true but expect false"})
		g.EXPECT().Fail()
		assert.False(g, true)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got true but expect false; param (index=0)"})
		g.EXPECT().Fail()
		assert.False(g, true, "param (index=0)")
	}
}

func TestNil(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Nil(g, nil)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Nil(g, (*int)(nil))
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 3 but expect nil"})
		g.EXPECT().Fail()
		assert.Nil(g, 3)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 3 but expect nil; param (index=0)"})
		g.EXPECT().Fail()
		assert.Nil(g, 3, "param (index=0)")
	}
}

func TestNotNil(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.NotNil(g, 3)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got nil but expect not nil"})
		g.EXPECT().Fail()
		assert.NotNil(g, nil)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got nil but expect not nil; param (index=0)"})
		g.EXPECT().Fail()
		assert.NotNil(g, nil, "param (index=0)")
	}
}

func TestEqual(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Equal(g, 0, 0)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 0 but expect (string) 0"})
		g.EXPECT().Fail()
		assert.Equal(g, 0, "0")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		g.EXPECT().Fail()
		assert.Equal(g, 0, "0", "param (index=0)")
	}
}

func TestNotEqual(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.NotEqual(g, "0", 0)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not (string) 0"})
		g.EXPECT().Fail()
		assert.NotEqual(g, "0", "0")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not (string) 0; param (index=0)"})
		g.EXPECT().Fail()
		assert.NotEqual(g, "0", "0", "param (index=0)")
	}
}

func TestSame(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Same(g, "0", "0")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 0 but expect (string) 0"})
		g.EXPECT().Fail()
		assert.Same(g, 0, "0")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		g.EXPECT().Fail()
		assert.Same(g, 0, "0", "param (index=0)")
	}
}

func TestNotSame(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.NotSame(g, "0", 0)
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not (string) 0"})
		g.EXPECT().Fail()
		assert.NotSame(g, "0", "0")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not (string) 0; param (index=0)"})
		g.EXPECT().Fail()
		assert.NotSame(g, "0", "0", "param (index=0)")
	}
}

func TestPanic(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Panic(g, func() { panic("this is an error") }, "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"did not panic"})
		g.EXPECT().Fail()
		assert.Panic(g, func() {}, "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"invalid pattern"})
		g.EXPECT().Fail()
		assert.Panic(g, func() { panic("this is an error") }, "an error \\")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		g.EXPECT().Fail()
		assert.Panic(g, func() { panic("there's no error") }, "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		g.EXPECT().Fail()
		assert.Panic(g, func() { panic("there's no error") }, "an error", "param (index=0)")
	}
}

func TestMatches(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Matches(g, "this is an error", "this is an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"invalid pattern"})
		g.EXPECT().Fail()
		assert.Matches(g, "this is an error", "an error \\")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		g.EXPECT().Fail()
		assert.Matches(g, "there's no error", "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		g.EXPECT().Fail()
		assert.Matches(g, "there's no error", "an error", "param (index=0)")
	}
}

func TestError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Error(g, errors.New("this is an error"), "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"invalid pattern"})
		g.EXPECT().Fail()
		assert.Error(g, errors.New("there's no error"), "an error \\")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not nil error"})
		g.EXPECT().Fail()
		assert.Error(g, nil, "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect not nil error; param (index=0)"})
		g.EXPECT().Fail()
		assert.Error(g, nil, "an error", "param (index=0)")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		g.EXPECT().Fail()
		assert.Error(g, errors.New("there's no error"), "an error")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		g.EXPECT().Fail()
		assert.Error(g, errors.New("there's no error"), "an error", "param (index=0)")
	}
}

func TestTypeOf(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.TypeOf(g, new(int), (*int)(nil))
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got type (string) but expect type (fmt.Stringer)"})
		g.EXPECT().Fail()
		assert.TypeOf(g, "string", (*fmt.Stringer)(nil))
	}
}

func TestImplements(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.Implements(g, errors.New("error"), (*error)(nil))
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"expect should be interface"})
		g.EXPECT().Fail()
		assert.Implements(g, new(int), (*int)(nil))
	}
}
