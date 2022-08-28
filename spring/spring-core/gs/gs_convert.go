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

package gs

import (
	"fmt"
	"reflect"

	"github.com/go-spring/spring-core/redis"
)

var beanConverters = map[reflect.Type]BeanConverter{}

func init() {
	initRedisBeanConverter()
}

type BeanConverter func(interface{}) (interface{}, error)

func RegisterBeanConverter(i interface{}, converter BeanConverter) {
	beanConverters[reflect.TypeOf(i)] = converter
}

func initRedisBeanConverter() {
	RegisterBeanConverter((*redis.KeyOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForKey(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.KeyOperations)(nil)))
	})
	RegisterBeanConverter((*redis.BitmapOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForBitmap(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.BitmapOperations)(nil)))
	})
	RegisterBeanConverter((*redis.StringOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForString(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.StringOperations)(nil)))
	})
	RegisterBeanConverter((*redis.HashOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForHash(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.HashOperations)(nil)))
	})
	RegisterBeanConverter((*redis.ListOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForList(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.ListOperations)(nil)))
	})
	RegisterBeanConverter((*redis.SetOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForSet(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.SetOperations)(nil)))
	})
	RegisterBeanConverter((*redis.ZSetOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForZSet(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.ZSetOperations)(nil)))
	})
	RegisterBeanConverter((*redis.ServerOperations)(nil), func(i interface{}) (interface{}, error) {
		if v, ok := i.(*redis.Client); ok {
			return v.OpsForServer(), nil
		}
		return nil, fmt.Errorf("%v can't convert to type %s", i, reflect.TypeOf((*redis.ServerOperations)(nil)))
	})
}
