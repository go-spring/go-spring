// +build !gs_replayer

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

package replayer

import (
	"context"

	"github.com/go-spring/spring-base/recorder"
	"github.com/go-spring/spring-base/util"
)

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return false
}

// Replay 根据 action 传入的匹配信息返回对应的数据。
func Replay(ctx context.Context, action *recorder.Action) (ok bool, err error) {
	panic(util.UnsupportedMethod)
}
