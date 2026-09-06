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

package luohua

import (
	"testing"

	"go-spring.org/spring/gs"
	StarterRedigo "go-spring.org/starter-redigo"
)

// TestRedisDriverArmsOnConfig proves luohua supplies its redigo pool-assembly
// Driver bean (overriding starter-redigo's bundled default) once
// ${spring.luohua.redis} is armed.
func TestRedisDriverArmsOnConfig(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.luohua.redis.tag", "corp")
		// Arming any spring.luohua.* key runs the master config, whose Identity
		// secret is a required field — satisfy it so only the redis capability
		// under test changes the wiring.
		app.Property("spring.luohua.identity.secret", "s3cret")
	}).RunTest(t, func(ts *struct {
		Driver StarterRedigo.Driver `autowire:"?"`
	}) {
		if ts.Driver == nil {
			t.Fatal("expected a redigo Driver bean when spring.luohua.redis is set")
		}
		switch ts.Driver.(type) {
		case RedisDriver, *RedisDriver:
			// value vs pointer is a gs detail; both are the luohua driver.
		default:
			t.Fatalf("driver is %T, want luohua.RedisDriver", ts.Driver)
		}
	})
}

// TestNoRedisDriverWhenOff confirms luohua provides no redigo Driver bean unless
// ${spring.luohua.redis} is set (and starter-redigo is not armed on its own).
func TestNoRedisDriverWhenOff(t *testing.T) {
	gs.Web(false).RunTest(t, func(ts *struct {
		Driver StarterRedigo.Driver `autowire:"?"`
	}) {
		if ts.Driver != nil {
			t.Fatal("no redigo Driver bean expected without spring.luohua.redis")
		}
	})
}
