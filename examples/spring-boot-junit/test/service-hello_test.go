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

package test

import (
	"testing"

	"github.com/go-spring/examples/spring-boot-junit/service"
	"github.com/go-spring/spring-boot"
	"github.com/magiconair/properties/assert"
)

func init() {
	SpringBoot.RegisterBean(new(ServiceHelloSuite))
}

type ServiceHelloSuite struct {
	_ SpringBoot.JUnitSuite `export:""`

	Hello *service.Hello `autowire:""`
}

func (s *ServiceHelloSuite) Test(t *testing.T) {
	assert.Equal(t, s.Hello.Say("world"), "hello world from junit")
	t.Run("child", func(t *testing.T) {
		assert.Equal(t, s.Hello.Say("world"), "hello world from junit")
	})
}
