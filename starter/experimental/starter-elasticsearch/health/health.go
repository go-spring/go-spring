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
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/cloud/actuator/health"
)

// NewClientHealth builds an indicator for an Elasticsearch client. It is
// registered once per configured instance and exported as health.Indicator, so
// an application that also imports starter-actuator gets Elasticsearch readiness
// folded into /readiness with no extra wiring.
//
// The probe issues an Info request; a context is always passed because the
// transport's OpenTelemetry instrumentation derives its span from it and panics
// on a nil parent context.
func NewClientHealth(name string, client *elasticsearch.Client) *health.Indicator {
	return &health.Indicator{Name: "elasticsearch:" + name, Probe: func(ctx context.Context) error {
		res, err := client.Info(client.Info.WithContext(ctx))
		if err != nil {
			return err
		}
		defer func() { _ = res.Body.Close() }()
		if res.IsError() {
			return fmt.Errorf("elasticsearch: info returned %s", res.Status())
		}
		return nil
	}}
}
