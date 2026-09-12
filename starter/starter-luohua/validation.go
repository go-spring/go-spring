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

package luohua

import (
	"context"

	"go-spring.org/stdlib/i18n"
	"go-spring.org/stdlib/validation"
)

// LocalizeValidation renders go-spring [validation.ValidationErrors] in a
// company's wording by feeding them the luohua error catalog. The go-spring
// validation seam is deliberately a plain msg func (so the package never
// imports an i18n implementation); a company drives it by supplying that func
// from its own message source. [i18n.Localizer] curries a [i18n.MessageSource]
// (e.g. luohua's catalog, whose validation.* keys carry the fleet's phrasing)
// into exactly the func [validation.ValidationErrors.Localize] expects.
func LocalizeValidation(src i18n.MessageSource, ctx context.Context, es validation.ValidationErrors) []string {
	return es.Localize(i18n.Localizer(src, ctx))
}
