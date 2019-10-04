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

package SpringUtils

import (
	"encoding/json"
	"errors"
	"reflect"
)

//
// 使用 json 序列化框架进行拷贝，支持匿名字段，支持类型转换。
//
func CopyBeanUseJson(src interface{}, dest interface{}) error {
	bytes, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, dest)
}

//
// 使用反射直接赋值的方式拷贝，不支持匿名字段，不支持类型转换。
// TODO 完善该方法，可以参考 json 序列化，需要进行性能测试。
//
func CopyBean(src interface{}, dest interface{}) error {

	srcVal := reflect.ValueOf(src)
	destVal := reflect.ValueOf(dest)

	if srcVal.Kind() != reflect.Ptr || destVal.Kind() != reflect.Ptr {
		return errors.New("src & dest should be pointer")
	}

	srcType := srcVal.Type().Elem()
	destType := destVal.Type().Elem()

	if srcType.Kind() != reflect.Struct || destType.Kind() != reflect.Struct {
		return errors.New("src & dest should be struct")
	}

	return copyStruct(srcVal.Elem(), srcType, destVal.Elem(), destType)
}

func copyStruct(srcVal reflect.Value, srcType reflect.Type, destVal reflect.Value, destType reflect.Type) error {

	for i := 0; i < srcType.NumField(); i++ {

		srcField := srcType.Field(i)
		srcFieldVal := srcVal.Field(i)

		if _, ok := destType.FieldByName(srcField.Name); ok {

			destFieldVal := destVal.FieldByName(srcField.Name)
			copyField(srcFieldVal, destFieldVal)
		}
	}

	return nil
}

func copyField(srcFieldVal reflect.Value, destFieldVal reflect.Value) {

	if srcFieldVal.Kind() == reflect.Struct && destFieldVal.Kind() == reflect.Struct {
		copyStruct(srcFieldVal, srcFieldVal.Type(), destFieldVal, destFieldVal.Type())

	} else {
		if srcFieldVal.Type().AssignableTo(destFieldVal.Type()) {
			destFieldVal.Set(srcFieldVal)
		}
	}
}
