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

package mock

import (
	"context"
	"testing"

	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/test/cases"
	"github.com/golang/mock/gomock"
)

func TestMock(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	c := redis.NewMockClient(ctrl)

	c.EXPECT().Exists(ctx, "mykey").Return(int64(0), nil)
	c.EXPECT().Append(ctx, "mykey", "Hello").Return(int64(5), nil)
	c.EXPECT().Append(ctx, "mykey", " World").Return(int64(11), nil)
	c.EXPECT().Get(ctx, "mykey").Return("Hello World", nil)

	cases.Append.Func(t, ctx, c)
}
