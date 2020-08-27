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

package SpringUtils_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-spring/spring-utils"
)

func TestPanicCond_When(t *testing.T) {
	SpringUtils.Panic(errors.New("test error")).When(false)

	t.Run("Panic", func(t *testing.T) {
		defer func() {
			fmt.Println(recover().(error).Error())
		}()
		SpringUtils.Panic(fmt.Errorf("reason: %s", "panic")).When(true)
	})

	t.Run("Panicf", func(t *testing.T) {
		defer func() {
			fmt.Println(recover().(error).Error())
		}()
		SpringUtils.Panicf("reason: %s", "panicf").When(true)
	})
}
