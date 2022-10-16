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
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/golang/mock/gomock"
)

func runCase(t *testing.T, f func(g *assert.MockT)) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	g := assert.NewMockT(ctrl)
	g.EXPECT().Helper().AnyTimes()
	f(g)
}

func TestTrue(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.True(g, true)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got false but expect true"})
		assert.True(g, false)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got false but expect true; param (index=0)"})
		assert.True(g, false, "param (index=0)")
	})
}

func TestFalse(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.False(g, false)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got true but expect false"})
		assert.False(g, true)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got true but expect false; param (index=0)"})
		assert.False(g, true, "param (index=0)")
	})
}

func TestNil(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Nil(g, nil)
	})
	runCase(t, func(g *assert.MockT) {
		assert.Nil(g, (*int)(nil))
	})
	runCase(t, func(g *assert.MockT) {
		var a []string
		assert.Nil(g, a)
	})
	runCase(t, func(g *assert.MockT) {
		var m map[string]string
		assert.Nil(g, m)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 3 but expect nil"})
		assert.Nil(g, 3)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 3 but expect nil; param (index=0)"})
		assert.Nil(g, 3, "param (index=0)")
	})
}

func TestNotNil(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.NotNil(g, 3)
	})
	runCase(t, func(g *assert.MockT) {
		a := make([]string, 0)
		assert.NotNil(g, a)
	})
	runCase(t, func(g *assert.MockT) {
		m := make(map[string]string)
		assert.NotNil(g, m)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got nil but expect not nil"})
		assert.NotNil(g, nil)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got nil but expect not nil; param (index=0)"})
		assert.NotNil(g, nil, "param (index=0)")
	})
}

func TestEqual(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Equal(g, 0, 0)
	})
	runCase(t, func(g *assert.MockT) {
		assert.Equal(g, []string{"a"}, []string{"a"})
	})
	runCase(t, func(g *assert.MockT) {
		assert.Equal(g, struct {
			text string
		}{text: "a"}, struct {
			text string
		}{text: "a"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (struct { Text string }) {a} but expect (struct { Text string \"json:\\\"text\\\"\" }) {a}"})
		assert.Equal(g, struct {
			Text string
		}{Text: "a"}, struct {
			Text string `json:"text"`
		}{Text: "a"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (struct { text string }) {a} but expect (struct { msg string }) {a}"})
		assert.Equal(g, struct {
			text string
		}{text: "a"}, struct {
			msg string
		}{msg: "a"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0"})
		assert.Equal(g, 0, "0")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		assert.Equal(g, 0, "0", "param (index=0)")
	})
}

func TestNotEqual(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.NotEqual(g, "0", 0)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got ([]string) [a] but expect not ([]string) [a]"})
		assert.NotEqual(g, []string{"a"}, []string{"a"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (string) 0 but expect not (string) 0"})
		assert.NotEqual(g, "0", "0")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (string) 0 but expect not (string) 0; param (index=0)"})
		assert.NotEqual(g, "0", "0", "param (index=0)")
	})
}

func TestJsonEqual(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.JsonEqual(g, `{"a":0,"b":1}`, `{"b":1,"a":0}`)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid character 'h' in literal true (expecting 'r')"})
		assert.JsonEqual(g, `this is an error`, `[{"b":1},{"a":0}]`)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid character 'h' in literal true (expecting 'r')"})
		assert.JsonEqual(g, `{"a":0,"b":1}`, `this is an error`)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (string) {\"a\":0,\"b\":1} but expect (string) [{\"b\":1},{\"a\":0}]"})
		assert.JsonEqual(g, `{"a":0,"b":1}`, `[{"b":1},{"a":0}]`)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (string) {\"a\":0} but expect (string) {\"a\":1}; param (index=0)"})
		assert.JsonEqual(g, `{"a":0}`, `{"a":1}`, "param (index=0)")
	})
}

func TestSame(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Same(g, "0", "0")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0"})
		assert.Same(g, 0, "0")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 0 but expect (string) 0; param (index=0)"})
		assert.Same(g, 0, "0", "param (index=0)")
	})
}

func TestNotSame(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.NotSame(g, "0", 0)
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0"})
		assert.NotSame(g, "0", "0")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not (string) 0; param (index=0)"})
		assert.NotSame(g, "0", "0", "param (index=0)")
	})
}

func TestPanic(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Panic(g, func() { panic("this is an error") }, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"did not panic"})
		assert.Panic(g, func() {}, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Panic(g, func() { panic("this is an error") }, "an error \\")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Panic(g, func() { panic("there's no error") }, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Panic(g, func() { panic("there's no error") }, "an error", "param (index=0)")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Panic(g, func() { panic(errors.New("there's no error")) }, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Panic(g, func() { panic(bytes.NewBufferString("there's no error")) }, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"[there's no error]\" which does not match \"an error\""})
		assert.Panic(g, func() { panic([]string{"there's no error"}) }, "an error")
	})
}

func TestMatches(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Matches(g, "this is an error", "this is an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Matches(g, "this is an error", "an error \\")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Matches(g, "there's no error", "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Matches(g, "there's no error", "an error", "param (index=0)")
	})
}

func TestError(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Error(g, errors.New("this is an error"), "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"invalid pattern"})
		assert.Error(g, errors.New("there's no error"), "an error \\")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not nil error"})
		assert.Error(g, nil, "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect not nil error; param (index=0)"})
		assert.Error(g, nil, "an error", "param (index=0)")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\""})
		assert.Error(g, errors.New("there's no error"), "an error")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got \"there's no error\" which does not match \"an error\"; param (index=0)"})
		assert.Error(g, errors.New("there's no error"), "an error", "param (index=0)")
	})
}

func TestTypeOf(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.TypeOf(g, new(int), (*int)(nil))
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got type (string) but expect type (fmt.Stringer)"})
		assert.TypeOf(g, "string", (*fmt.Stringer)(nil))
	})
}

func TestImplements(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		assert.Implements(g, errors.New("error"), (*error)(nil))
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"expect should be interface"})
		assert.Implements(g, new(int), (*int)(nil))
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got type (*int) but expect type (io.Reader)"})
		assert.Implements(g, new(int), (*io.Reader)(nil))
	})
}

func TestInSlice(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported expect value (string) 1"})
		assert.InSlice(g, 1, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 1 is not in ([]string) [1]"})
		assert.InSlice(g, 1, []string{"1"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int64) 1 is not in ([]int64) [3 2]"})
		assert.InSlice(g, int64(1), []int64{3, 2})
	})
	runCase(t, func(g *assert.MockT) {
		assert.InSlice(g, int64(1), []int64{3, 2, 1})
		assert.InSlice(g, "1", []string{"3", "2", "1"})
	})
}

func TestNotInSlice(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported expect value (string) 1"})
		assert.NotInSlice(g, 1, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got type (int) doesn't match expect type ([]string)"})
		assert.NotInSlice(g, 1, []string{"1"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (string) 1 is in ([]string) [3 2 1]"})
		assert.NotInSlice(g, "1", []string{"3", "2", "1"})
	})
	runCase(t, func(g *assert.MockT) {
		assert.NotInSlice(g, int64(1), []int64{3, 2})
	})
}

func TestSubInSlice(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported got value (int) 1"})
		assert.SubInSlice(g, 1, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported expect value (string) 1"})
		assert.SubInSlice(g, []int{1}, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got ([]int) [1] is not sub in ([]string) [1]"})
		assert.SubInSlice(g, []int{1}, []string{"1"})
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got ([]int) [1] is not sub in ([]int) [3 2]"})
		assert.SubInSlice(g, []int{1}, []int{3, 2})
	})
	runCase(t, func(g *assert.MockT) {
		assert.SubInSlice(g, []int{1}, []int{3, 2, 1})
		assert.SubInSlice(g, []string{"1"}, []string{"3", "2", "1"})
	})
}

func TestInMapKeys(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported expect value (string) 1"})
		assert.InMapKeys(g, 1, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 1 is not in keys of (map[string]string) map[1:1]"})
		assert.InMapKeys(g, 1, map[string]string{"1": "1"})
	})
	runCase(t, func(g *assert.MockT) {
		assert.InMapKeys(g, int64(1), map[int64]int64{3: 1, 2: 2, 1: 3})
		assert.InMapKeys(g, "1", map[string]string{"3": "1", "2": "2", "1": "3"})
	})
}

func TestInMapValues(t *testing.T) {
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"unsupported expect value (string) 1"})
		assert.InMapValues(g, 1, "1")
	})
	runCase(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"got (int) 1 is not in values of (map[string]string) map[1:1]"})
		assert.InMapValues(g, 1, map[string]string{"1": "1"})
	})
	runCase(t, func(g *assert.MockT) {
		assert.InMapValues(g, int64(1), map[int64]int64{3: 1, 2: 2, 1: 3})
		assert.InMapValues(g, "1", map[string]string{"3": "1", "2": "2", "1": "3"})
	})
}
