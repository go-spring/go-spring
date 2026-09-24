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

// Command example runs the lock demo the way a real application does: the
// locker comes from configuration ("spring.lock.instances.memory.demo") through the
// starter wiring, business code injects lock.Locker and never knows the
// backend. The backend here is the in-process MemoryLocker via
// starter-lock-memory, so the whole demo runs with zero external services —
// swap the blank import to starter-lock-redis (say) and the same code works
// against a real backend.
package main

import (
	"go-spring.org/spring/gs"

	_ "go-spring.org/cloud/lock/example/service"
	_ "go-spring.org/starter-lock-memory"
)

func main() { gs.Run() }
