/*
 * Copyright 2012-2019 the original author or authors.
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

package app

import (
	"context"
	"time"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(new(MyRunner)).Export((*gs.AppRunner)(nil))
}

type MyRunner struct {
	Logger *log.Logger `logger:""`
}

func (r *MyRunner) Run(ctx gs.Context) {
	ctx.Go(func(ctx context.Context) {
		defer func() { r.Logger.Info("exit after waiting in MyRunner::Run") }()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.Logger.Info("MyRunner::Run")
			}
		}
	})
}
