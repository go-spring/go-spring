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

	"github.com/hibiken/asynq"
	"go-spring.org/cloud/actuator/health"
)

// NewClientHealth builds an indicator for an Asynq instance. The probe pings
// Redis through asynq's Inspector (a fresh connection per check, so it
// verifies reachability without coupling to the producer/worker lifecycle).
//
// connOpt is the Redis connection option the caller (the starter) already
// resolved from Config; keeping it as a parameter avoids this subpackage
// depending on the starter's private Config type.
func NewClientHealth(name string, connOpt asynq.RedisConnOpt) *health.Indicator {
	return &health.Indicator{Name: "asynq:" + name, Probe: func(ctx context.Context) error {
		insp := asynq.NewInspector(connOpt)
		defer insp.Close()
		// GetQueueInfo returns nil,nil for an empty queue; a reachable Redis
		// returns without error either way. Only a connection/command error
		// should fail the probe.
		_, err := insp.GetQueueInfo("default")
		return err
	}}
}
