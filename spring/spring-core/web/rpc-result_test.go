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

package web_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
)

func TestRpcError(t *testing.T) {
	err := errors.New("this is an error")

	r1 := web.ERROR.Error(err)
	assert.Equal(t, r1, &web.RpcResult{
		ErrorCode: web.ErrorCode(web.ERROR),
		Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-core/web/rpc-result_test.go:30: this is an error",
	})

	r2 := web.ERROR.ErrorWithData(err, "error_with_data")
	assert.Equal(t, r2, &web.RpcResult{
		ErrorCode: web.ErrorCode(web.ERROR),
		Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-core/web/rpc-result_test.go:36: this is an error",
		Data:      "error_with_data",
	})

	func() {
		defer func() {
			assert.Equal(t, recover(), &web.RpcResult{
				ErrorCode: web.ErrorCode(web.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-core/web/rpc-result_test.go:50: this is an error",
			})
		}()
		web.ERROR.Panic(err).When(err != nil)
	}()

	func() {
		defer func() {
			assert.Equal(t, recover(), &web.RpcResult{
				ErrorCode: web.ErrorCode(web.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-core/web/rpc-result_test.go:60: this is an error",
			})
		}()
		web.ERROR.Panicf(err.Error()).When(true)
	}()

	func() {
		defer func() {
			assert.Equal(t, recover(), &web.RpcResult{
				ErrorCode: web.ErrorCode(web.ERROR),
				Err:       "/Users/didi/GitHub/go-spring/go-spring/spring/spring-core/web/rpc-result_test.go:70: this is an error",
			})
		}()
		web.ERROR.PanicImmediately(err)
	}()
}
