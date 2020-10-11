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

package SpringWeb

// WebValidator 参数校验器接口
type WebValidator interface {
	Engine() interface{}
	Validate(i interface{}) error
}

// Validator 全局参数校验器
var Validator WebValidator

// Validate 参数校验
func Validate(i interface{}) error {
	if Validator != nil {
		return Validator.Validate(i)
	}
	return nil
}
