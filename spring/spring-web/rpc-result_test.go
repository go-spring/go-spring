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
)

func TestRpcError(t *testing.T) {
	err := errors.New("this is an error")

	r1 := SpringWeb.ERROR.Error(err)
	SpringUtils.AssertEqual(t, r1, &SpringWeb.RpcResult{
		ErrorCode: SpringWeb.ErrorCode(SpringWeb.ERROR),
		Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:30: this is an error",
	})

	r2 := SpringWeb.ERROR.ErrorWithData(err, "error_with_data")
	SpringUtils.AssertEqual(t, r2, &SpringWeb.RpcResult{
		ErrorCode: SpringWeb.ErrorCode(SpringWeb.ERROR),
		Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:36: this is an error",
		Data:      "error_with_data",
	})

	func() {
		defer func() {
			SpringUtils.AssertEqual(t, recover(), &SpringWeb.RpcResult{
				ErrorCode: SpringWeb.ErrorCode(SpringWeb.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:50: this is an error",
			})
		}()
		SpringWeb.ERROR.Panic(err).When(err != nil)
	}()

	func() {
		defer func() {
			SpringUtils.AssertEqual(t, recover(), &SpringWeb.RpcResult{
				ErrorCode: SpringWeb.ErrorCode(SpringWeb.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:60: this is an error",
			})
		}()
		SpringWeb.ERROR.Panicf(err.Error()).When(true)
	}()

	func() {
		defer func() {
			SpringUtils.AssertEqual(t, recover(), &SpringWeb.RpcResult{
				ErrorCode: SpringWeb.ErrorCode(SpringWeb.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-web/spring-web-rpc-result_test.go:70: this is an error",
			})
		}()
		SpringWeb.ERROR.PanicImmediately(err)
	}()
}
