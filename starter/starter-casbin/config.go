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

package StarterCasbin

// Config holds the configuration parameters for a Casbin enforcer.
type Config struct {
	Model    string `value:"${model}"`          // Path to the model file (model.conf)
	Policy   string `value:"${policy}"`         // Path to the policy file (policy.csv)
	AutoSave bool   `value:"${autoSave:=true}"` // Whether policy mutations are persisted back to the policy file
}
