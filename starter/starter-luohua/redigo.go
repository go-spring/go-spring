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
	"context"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterRedigo "go-spring.org/starter-redigo"
)

// init provides luohua's redis pool-assembly driver as a single Driver bean,
// gated on ${spring.luohua.redis} (OnProperty is a prefix check). It is one
// bean, so no gs.Module / conf.Bind is needed: the constructor binds
// [RedisConfig] straight from ${spring.luohua.redis} at wiring time.
// Starter-redigo injects this Driver into every pool under ${spring.redigo};
// when no Driver bean is present redigo falls back to its bundled DefaultDriver
// internally, so this capability is what opts a luohua service into the
// company's redis assembly. A team that wants its own redis driver keeps
// spring.luohua.redis off and provides its own Driver bean instead.
func init() {
	gs.Provide(func(c RedisConfig) StarterRedigo.Driver {
		return RedisDriver{tag: c.Tag}
	}, gs.TagArg("${spring.luohua.redis}")).
		Condition(gs.OnProperty("spring.luohua.redis")).
		Caller(1)
}

// RedisDriver is the luohua pool-assembly driver: it builds each pool through
// the standard one-shot assembly ([StarterRedigo.NewPool]) and then layers a
// luohua command interceptor outermost, so a luohua-flavored behavior runs on
// every redis command of every pool in the process.
type RedisDriver struct {
	// tag is luohua's marker from ${spring.luohua.redis.tag}, injected at wiring.
	tag string
}

// CreateClient builds one redis pool per the luohua assembly contract.
func (d RedisDriver) CreateClient(ctx context.Context, c StarterRedigo.Config) (*StarterRedigo.Pool, error) {
	p, err := StarterRedigo.NewPool(ctx, c)
	if err != nil {
		return nil, err
	}
	p.UseCommandInterceptor(luohuaCommandInterceptor(d.tag))
	return p, nil
}

// luohuaCommandInterceptor is the outermost layer on each luohua pool's command
// path. It forwards every command unchanged; the luohua flavor here is a
// per-command debug trace tagged with the configured luohua marker, and it
// demonstrates where a company would hang its own per-command logic (rewrite
// ctx/cmd/args, or short-circuit without reaching redis).
func luohuaCommandInterceptor(tag string) StarterRedigo.CommandInterceptor {
	return func(next StarterRedigo.CommandHandler) StarterRedigo.CommandHandler {
		return func(ctx context.Context, cmd string, args []any) (any, error) {
			log.Debugf(ctx, log.TagAppDef, "luohua/redis tag=%s: %s", tag, cmd)
			return next(ctx, cmd, args)
		}
	}
}
