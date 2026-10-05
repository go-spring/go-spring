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

// recover.go is the hertz seam of the unified panic policy. It replaces the
// hertz contrib recovery middleware (same outermost position, same 500
// semantics) so a recovered panic is additionally reported through the shared
// goutil chain — structured log via go-spring.org/log — with the panicking
// frames still on the stack, instead of going only to hlog.

package StarterHertz

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go-spring.org/stdlib/goutil"
)

// Recover is the panic-recovery middleware installed outermost when
// middleware.recovery.enabled is on (the default). It reports the panic
// through the shared chain and aborts with 500, matching the semantics of
// hertz's built-in recovery middleware.
func Recover() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				goutil.ReportPanic(ctx, r)
				c.AbortWithStatus(consts.StatusInternalServerError)
			}
		}()
		c.Next(ctx)
	}
}
