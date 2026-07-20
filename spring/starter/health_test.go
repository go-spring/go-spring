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

package starter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"go-spring.org/spring/actuator/health"
)

func TestNewIndicator_NameAndProbe(t *testing.T) {
	want := errors.New("down")
	ind := NewIndicator("redis:cache", func(context.Context) error { return want })
	assert.Equal(t, "redis:cache", ind.HealthName())
	assert.Equal(t, want, ind.CheckHealth(context.Background()))
}

func TestNewIndicator_Defaults(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil })
	// No WithGroups: must fall back to health defaults (readiness + startup),
	// which means Grouped must NOT be implemented.
	_, grouped := ind.(health.Grouped)
	assert.False(t, grouped, "default indicator must not implement Grouped")
	assert.Equal(t, []health.Group{health.GroupReadiness, health.GroupStartup}, health.GroupsOf(ind))
	assert.True(t, health.IsCritical(ind), "default indicator must be critical")
}

func TestNewIndicator_WithGroups(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil },
		WithGroups(health.GroupLiveness))
	g, ok := ind.(health.Grouped)
	assert.True(t, ok)
	assert.Equal(t, []health.Group{health.GroupLiveness}, g.HealthGroups())
	assert.True(t, health.InGroup(ind, health.GroupLiveness))
	assert.False(t, health.InGroup(ind, health.GroupReadiness))
}

func TestNewIndicator_NonCritical(t *testing.T) {
	ind := NewIndicator("x", func(context.Context) error { return nil }, NonCritical())
	assert.False(t, health.IsCritical(ind))
}
