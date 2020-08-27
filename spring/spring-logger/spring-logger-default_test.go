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

package SpringLogger_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-spring/spring-logger"
)

func TestConsole(t *testing.T) {
	c := SpringLogger.NewConsole(SpringLogger.TraceLevel)

	fmt.Println("a", "=", "1")
	c.Debug("a", "=", "1")
	c.Debugf("a=%d", 1)

	c.Warn("a", "=", "1")
	c.Warnf("a=%d", 1)

	c.Error("a", "=", "1")
	c.Errorf("a=%d", 1)

	t.Run("panic error", func(t *testing.T) {
		defer func() {
			err := recover()
			fmt.Println(err)
		}()
		c.Panic("error")
	})

	t.Run("panic error new", func(t *testing.T) {
		defer func() {
			err := recover()
			fmt.Println(err)
		}()
		c.Panic(errors.New("error"))
	})
}
