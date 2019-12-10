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

package SpringCore

import (
	"reflect"
	"strings"
)

func IsValidBean(k reflect.Kind) bool {
	for _, validKind := range _VALID_BEAN_KINDS {
		if k == validKind {
			return true
		}
	}
	return false
}

func IsValidReceiver(k reflect.Kind) bool {
	for _, validReceiver := range _VALID_RECEIVER_KINDS {
		if k == validReceiver {
			return true
		}
	}
	return false
}

//
// 获取原始类型的全限定名，golang 允许不同的路径下存在相同的包，故此有全限定名的需求。形如
// "github.com/go-spring/go-spring/spring-core/SpringCore.DefaultSpringContext"
//
func TypeName(t reflect.Type) string {

	if t == nil {
		panic("type shouldn't be nil")
	}

	// Map 的全限定名太复杂，不予处理，而且 Map 作为注入对象要三思而后行！
	for {
		if k := t.Kind(); k != reflect.Ptr && k != reflect.Slice {
			break
		} else {
			t = t.Elem()
		}
	}

	if pkgPath := t.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + t.String()
	} else {
		return t.String()
	}
}

//
// 解析 BeanId 的内容，"TypeName:BeanName?" 或者 "[]?"
//
func ParseBeanId(beanId string) (typeName string, beanName string, nullable bool) {

	if ss := strings.Split(beanId, ":"); len(ss) > 1 {
		typeName = ss[0]
		beanName = ss[1]
	} else {
		beanName = ss[0]
	}

	if strings.HasSuffix(beanName, "?") {
		beanName = beanName[:len(beanName)-1]
		nullable = true
	}

	if beanName == "[]" && typeName != "" {
		panic("collection mode shouldn't have type")
	}
	return
}
