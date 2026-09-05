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

package health2

import (
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/actuator/health"
)

// NewBigCacheHealth builds an indicator for a BigCache instance. BigCache is an
// in-process heap cache with no network endpoint, so there is no connectivity
// to probe: the instance existing means it is ready. The indicator therefore
// always reports UP, contributing a readiness signal (and confirming the
// instance is registered) without a round-trip. It is exported as
// health.Indicator, so an application that also imports starter-actuator gets
// the instance folded into /readiness with no extra wiring.
func NewBigCacheHealth(name string, _ *bigcache.BigCache) *health.Indicator {
	return &health.Indicator{Name: "bigcache:" + name, Probe: func(ctx context.Context) error {
		return nil
	}}
}
