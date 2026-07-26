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

package StarterGateway

import "fmt"

// parseError describes a malformed predicate/filter literal in the route config.
// Compilation surfaces it so a bad edit is rejected and the previous route table
// is kept in place instead of being swapped for a broken one.
type parseError struct {
	what  string
	token string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("gateway: invalid %s: %q", e.what, e.token)
}
