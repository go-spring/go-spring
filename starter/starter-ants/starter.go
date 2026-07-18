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

package StarterAnts

import (
	"github.com/panjf2000/ants/v2"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple ants pools as a group.
	// Each pool is created according to the configuration in "${spring.ants}".
	// This allows defining multiple goroutine pools dynamically.
	//
	// ants spawns a background purge goroutine (unless disable-purge is set),
	// so Release must be called on shutdown to release it — the destroy
	// callback handles that.
	gs.Group("${spring.ants}", newPool, destroyPool)
}

// newPool creates a new ants pool based on the provided configuration.
func newPool(c Config) (*ants.Pool, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "ants driver not found: %s", c.Driver)
	}
	pool, err := d.CreatePool(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create ants pool")
	}
	return pool, nil
}

// destroyPool releases the ants pool, stopping its background purge goroutine
// and reclaiming all workers.
func destroyPool(pool *ants.Pool) error {
	pool.Release()
	return nil
}
