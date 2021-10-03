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

package util_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
)

func TestNow(t *testing.T) {
	assert.True(t, time.Now().Sub(util.Now(nil)).Milliseconds() < 1)
	assert.True(t, time.Now().Sub(util.Now(context.TODO())).Milliseconds() < 1)
	ctx := util.MockNow(context.TODO(), time.Now().Add(-60*time.Second))
	assert.True(t, time.Now().Sub(util.Now(ctx).Add(60*time.Second)).Milliseconds() < 1)
}
