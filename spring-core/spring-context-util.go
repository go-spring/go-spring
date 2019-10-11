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
	"fmt"
	"reflect"
	"strings"
)

// 获取Bean的唯一签名，规则 ：pkgpath + struct name
func getBeanUnameByType(t reflect.Type) string {
	return fmt.Sprintf(
		"%s.%s",
		strings.Replace(t.Elem().PkgPath(), "/", ".", -1),
		t.Elem().Name(),
	)
}

// 反射提取type，强制必须传入指针
func MustPointerTypeOf(bean SpringBean) reflect.Type {
	t := reflect.TypeOf(bean)
	if t.Kind() != reflect.Ptr {
		panic("bean must be pointer")
	}
	return t
}
