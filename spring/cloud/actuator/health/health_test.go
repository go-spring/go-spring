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
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestNewIndicator_NameAndProbe(t *testing.T) {
	want := errors.New("down")
	ind := NewIndicator("redis:cache", func(context.Context) error { return want })

	assert.String(t, ind.HealthName()).Equal("redis:cache")
	assert.Error(t, ind.CheckHealth(context.Background())).Is(want)
}

func TestNewIndicator_Defaults(t *testing.T) {
	// Without options the indicator is critical and declares no explicit
	// groups (HealthGroups returns nil), so the collector applies its default
	// routing. CheckHealth reflects whatever the probe returns.
	ind := NewIndicator("x", func(context.Context) error { return nil })

	assert.That(t, ind.IsCritical()).True()
	assert.Slice(t, ind.HealthGroups()).Nil()
	assert.Error(t, ind.CheckHealth(context.Background())).Nil()
}

func TestNewIndicator_WithGroups(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil },
		WithGroups(GroupLiveness))

	assert.Slice(t, ind.HealthGroups()).Equal([]Group{GroupLiveness})
}

func TestNewIndicator_NonCritical(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil }, NonCritical())

	assert.That(t, ind.IsCritical()).False()
}

func TestNewIndicator_CombinesOptions(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil },
		WithGroups(GroupStartup, GroupReadiness), NonCritical())

	assert.Slice(t, ind.HealthGroups()).Equal([]Group{GroupStartup, GroupReadiness})
	assert.That(t, ind.IsCritical()).False()
}
