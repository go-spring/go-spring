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

// Package StarterBatchRedis contributes Redis-backed
// [batch.JobRepository] beans to a Go-Spring application. Blank-importing
// this package registers one JobRepository per entry under
// spring.batch-repository.<name>; each repository reuses the *redis.Client
// bean named by its `client` field (provided by starter-go-redis under
// spring.go-redis.<client>).
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it
// exports no port and holds no connection of its own, it merely contributes
// a bean type behind the framework-neutral batch.JobRepository seam. Switching
// the repository backend from Redis to a SQL database is therefore a
// blank-import swap — no business code changes.
//
// The JobRepository bean is registered under its config name and exported as
// batch.JobRepository, so the batch runner (starter-batch) picks it up by
// interface via spring.batch.repository=<name>:
//
//	spring.go-redis.cache.addr=127.0.0.1:6379
//	spring.batch-repository.jobs.client=cache
//	spring.batch.repository=jobs
package StarterBatchRedis

import (
	"runtime"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/batch"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.batch-repository"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.batch-repository}"); err != nil {
			return err
		}
		for name, c := range m {
			// Fail fast: silently defaulting to some arbitrary *redis.Client
			// would hide a misconfiguration that only surfaces the first time
			// the batch runner tries to persist progress, potentially in
			// production. Refusing to boot is safer.
			if c.Client == "" {
				return errutil.Explain(nil, "batch-redis: instance %q missing required property %q",
					name, "spring.batch-repository."+name+".client")
			}
			// TagArg injects the *redis.Client bean by name — this is the
			// seam that ties the JobRepository to a specific redis instance.
			b := r.Provide(newRedisRepository, gs.ValueArg(c), gs.TagArg(c.Client)).
				Name(name).
				Export(gs.As[batch.JobRepository]())
			b.SetFileLine(file, line)
		}
		return nil
	})
}
