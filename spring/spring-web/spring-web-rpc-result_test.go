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

package SpringWeb_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
	"github.com/magiconair/properties/assert"
)

func TestRpcError(t *testing.T) {
	err := errors.New("this is an error")

	r1 := SpringWeb.ERROR.Error(err)
	assert.Equal(t, SpringUtils.ToJson(r1), `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:31: this is an error"}`)

	r2 := SpringWeb.ERROR.ErrorWithData(err, "data")
	assert.Equal(t, SpringUtils.ToJson(r2), `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:34: this is an error","data":"data"}`)

	func() {
		defer func() {
			assert.Equal(t, SpringUtils.ToJson(recover()), `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:41: this is an error"}`)
		}()
		SpringWeb.ERROR.Panic(err).When(err != nil)
	}()

	func() {
		defer func() {
			assert.Equal(t, SpringUtils.ToJson(recover()), `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:48: this is an error"}`)
		}()
		SpringWeb.ERROR.Panicf(err.Error()).When(true)
	}()

	func() {
		defer func() {
			assert.Equal(t, SpringUtils.ToJson(recover()), `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:55: this is an error"}`)
		}()
		SpringWeb.ERROR.PanicImmediately(err)
	}()
}
