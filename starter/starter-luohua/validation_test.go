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
	"testing"

	"go-spring.org/cloud/validation"
)

// TestValidationLocalizesThroughLuohuaCatalog locks the validation owned seam's
// company customization: go-spring validation errors carry a plain
// "validation.<rule>" message key, and a company decides how they read by
// supplying the Localize msg func from its own i18n. Luohua feeds its error
// catalog in, so a validation failure renders in the fleet's wording (not a
// hardcoded English default).
func TestValidationLocalizesThroughLuohuaCatalog(t *testing.T) {
	cat := luohuaCatalog("zh") // luohua's company catalog, zh default
	es := validation.ValidationErrors{
		{Field: "Age", Rule: "min", Param: "18"},
	}

	zh := LocalizeValidation(cat, context.Background(), es)
	if len(zh) != 1 || zh[0] != "Age 不得小于 18" {
		t.Fatalf("zh localization: got %v, want [Age 不得小于 18]", zh)
	}

	en := LocalizeValidation(luohuaCatalog("en"), context.Background(), es)
	if len(en) != 1 || en[0] != "Age must be at least 18" {
		t.Fatalf("en localization: got %v, want [Age must be at least 18]", en)
	}
}
