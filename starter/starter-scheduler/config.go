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

package StarterScheduler

import "time"

// Config binds ${spring.scheduler}. Only process-level knobs live here: a job's
// trigger and execution options are declared where the job is registered (see
// [Provide]), so a schedule is read in one place instead of being matched to
// code by name across two sources.
type Config struct {
	// DrainTimeout bounds how long Stop waits for in-flight runs to finish during
	// graceful shutdown before giving up.
	DrainTimeout time.Duration `value:"${drain-timeout:=30s}"`
}
