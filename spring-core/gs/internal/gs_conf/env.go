/*
 * Copyright 2024 The Go-Spring Authors.
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

package gs_conf

import (
	"os"
	"strings"

	"github.com/go-spring/stdlib/flatten"
)

// extractEnvironments extracts environment variables.
//
// Variables with the prefix "GS_" are transformed:
//   - The prefix "GS_" is removed.
//   - Remaining underscores '_' are replaced by dots '.'.
//   - Keys are converted to lowercase.
//
// All other variables are stored using their original key and value.
// Malformed environment variables (e.g., "=value") are ignored.
func extractEnvironments() (*flatten.Properties, error) {

	p := flatten.NewProperties(nil)
	environs := os.Environ()
	if len(environs) == 0 {
		return p, nil
	}

	const prefix = "GS_"
	for _, env := range environs {
		ss := strings.SplitN(env, "=", 2)
		if len(ss[0]) == 0 {
			continue // Skip malformed env vars like "=::=:"
		}

		k, v := ss[0], ""
		if len(ss) > 1 {
			v = ss[1]
		}

		propKey := k
		if s, ok := strings.CutPrefix(k, prefix); ok {
			propKey = strings.ReplaceAll(s, "_", ".")
			propKey = strings.ToLower(propKey)
		}
		p.Set(propKey, v)
	}
	return p, nil
}
