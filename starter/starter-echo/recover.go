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

// recover.go is the echo seam of the unified panic policy. It replaces echo's
// own middleware.Recover (same outermost position, same 500 semantics) so a
// recovered panic is additionally reported through the shared goutil chain —
// structured log via go-spring.org/log — with the panicking frames still on
// the stack, instead of going only to echo's logger.

package StarterEcho

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go-spring.org/stdlib/goutil"
)

// Recover is the panic-recovery middleware installed outermost when
// middleware.recovery.enabled is on (the default). It reports the panic
// through the shared chain and answers 500, matching the semantics of echo's
// built-in middleware.Recover.
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					goutil.ReportPanic(c.Request().Context(), r)
					c.Error(echo.NewHTTPError(http.StatusInternalServerError))
				}
			}()
			return next(c)
		}
	}
}
