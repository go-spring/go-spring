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

import (
	"github.com/casbin/casbin/v2"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Casbin enforcers as a group.
	// Each instance is created according to the configuration in "${spring.casbin}",
	// so an application can hold several enforcers (e.g. one per domain) side by side
	// and inject the one it needs by bean name.
	gs.Group("${spring.casbin}", newEnforcer, nil)
}

// newEnforcer builds a *casbin.Enforcer from a model file and a file-backed
// policy. This keeps the starter dependency-free beyond Casbin itself: the
// default file adapter needs no database. To persist policies elsewhere (GORM,
// Redis, ...), provide your own *casbin.Enforcer bean built with the matching
// Casbin adapter instead of relying on this group.
func newEnforcer(c Config) (*casbin.Enforcer, error) {
	e, err := casbin.NewEnforcer(c.Model, c.Policy)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create casbin enforcer")
	}
	e.EnableAutoSave(c.AutoSave)
	return e, nil
}
