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

package health

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// plainIndicator implements only Indicator, so it takes every default.
type plainIndicator struct{}

func (plainIndicator) HealthName() string                { return "plain" }
func (plainIndicator) CheckHealth(context.Context) error { return nil }

// nonCritical additionally opts out of critical via the Critical interface.
type nonCritical struct{ plainIndicator }

func (nonCritical) IsCritical() bool { return false }

func TestIsCritical_DefaultsTrue(t *testing.T) {
	// An indicator that does not implement Critical is treated as critical, so
	// its failure lowers the aggregate (backward-compatible behavior).
	assert.That(t, IsCritical(plainIndicator{})).True()
}

func TestIsCritical_HonorsOptOut(t *testing.T) {
	assert.That(t, IsCritical(nonCritical{})).False()
}
