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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/golang/mock/gomock"
)

func TestFluentTrue(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.That(g, true).IsTrue()
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got false but expect true"})
		g.EXPECT().Fail()
		assert.That(g, false).IsTrue()
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"got false but expect true; param (index=0)"})
		g.EXPECT().Fail()
		assert.That(g, false).IsTrue("param (index=0)")
	}
}

func TestFluentHasPrefix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		assert.That(g, "hello, world!").HasPrefix("hello")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"'hello, world!' doesn't hava prefix 'xxx'"})
		g.EXPECT().Fail()
		assert.That(g, "hello, world!").HasPrefix("xxx")
	}

	{
		g := assert.NewMockT(ctrl)
		g.EXPECT().Helper().AnyTimes()
		g.EXPECT().Log([]interface{}{"'hello, world!' doesn't hava prefix 'xxx'; param (index=0)"})
		g.EXPECT().Fail()
		assert.That(g, "hello, world!").HasPrefix("xxx", "param (index=0)")
	}
}
