/*
 * Copyright 2025 The Go-Spring Authors.
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

package messaging

import (
	"context"
	"fmt"

	"go-spring.org/stdlib/goutil"
)

// Recover wraps h so a panic inside the handler is reported through the
// shared panic chain (goutil) and converted into a handler error, instead of
// unwinding into the broker SDK's delivery goroutine — where it would either
// crash the process or silently kill the delivery loop, depending on the
// SDK.
//
// The converted error takes the binder's normal failure path (nack /
// redelivery, exactly like a handler error), so a poisoned message behaves
// the same whether the handler signals failure by error or by panic. Every
// binder in this family wraps the application handler with Recover at
// Subscribe time; applications only need it directly when they drive a
// broker client by hand.
func Recover(h Handler) Handler {
	return func(ctx context.Context, msg *Message) (err error) {
		defer func() {
			if r := recover(); r != nil {
				goutil.ReportPanic(ctx, r)
				err = fmt.Errorf("messaging: handler panicked: %v", r)
			}
		}()
		return h(ctx, msg)
	}
}
