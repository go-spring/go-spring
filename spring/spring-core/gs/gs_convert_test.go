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

package gs_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/gs"
)

type redisClient struct {
	listOp *listOperations
}

type listOperations struct {
	value int
}

func TestBeanConverter(t *testing.T) {

	gs.RegisterBeanConverter((*listOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redisClient); ok {
			return v.listOp, nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*listOperations)(nil)))
	})

	c := gs.New()
	c.Object(&redisClient{listOp: &listOperations{value: 4}})
	err := runTest(c, func(p gs.Context) {
		var op *listOperations
		err := p.Get(&op, "redisClient")
		assert.Nil(t, err)
		assert.Equal(t, op.value, 4)
	})
	assert.Nil(t, err)
}
