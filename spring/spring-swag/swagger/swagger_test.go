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

package swagger_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-swag/swagger"
)

func Test_Doc(t *testing.T) {
	c := web.NewAbstractContainer(web.ContainerConfig{})
	swagger.Doc(c).WithID("go-spring").WithHost("https://go-spring.com")
	m := c.HandleGet("/idx", web.FUNC(func(ctx web.Context) {}))
	swagger.Path(m).WithDescription("welcome to go-spring")
	web.RegisterSwaggerHandler(func(router web.Router, doc string) { fmt.Println(doc) })
	_ = c.Start()
	assert.True(t, true)
}
