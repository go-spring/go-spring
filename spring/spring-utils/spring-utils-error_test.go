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
			SpringUtils.AssertEqual(t, errors.New("reason: panic"), recover())
		}()
		SpringUtils.Panic(fmt.Errorf("reason: %s", "panic")).When(true)
	})

	t.Run("Panicf", func(t *testing.T) {
		defer func() {
			SpringUtils.AssertEqual(t, errors.New("reason: panicf"), recover())
		}()
		SpringUtils.Panicf("reason: %s", "panicf").When(true)
	})
}

func TestWithCause(t *testing.T) {

	t.Run("cause is string", func(t *testing.T) {
		err := SpringUtils.WithCause("this is a string")
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, "this is a string", v)
	})

	t.Run("cause is error", func(t *testing.T) {
		err := SpringUtils.WithCause(errors.New("this is an error"))
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, errors.New("this is an error"), v)
	})

	t.Run("cause is int", func(t *testing.T) {
		err := SpringUtils.WithCause(123456)
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, 123456, v)
	})
}

func panic2Error(v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = SpringUtils.WithCause(r)
		}
	}()
	panic(v)
}

func TestPanic2Error(t *testing.T) {

	t.Run("panic is string", func(t *testing.T) {
		err := panic2Error("this is a string")
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, "this is a string", v)
	})

	t.Run("panic is error", func(t *testing.T) {
		err := panic2Error(errors.New("this is an error"))
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, errors.New("this is an error"), v)
	})

	t.Run("panic is int", func(t *testing.T) {
		err := panic2Error(123456)
		v := SpringUtils.Cause(err)
		SpringUtils.AssertEqual(t, 123456, v)
	})
}

func TestErrorWithFileLine(t *testing.T) {

	err := SpringUtils.ErrorWithFileLine(errors.New("this is an error"))
	SpringUtils.AssertMatches(t, ".*:98: this is an error", err.Error())

	fnError := func(e error) error {
		return SpringUtils.ErrorWithFileLine(e, 1)
	}

	err = fnError(errors.New("this is an error"))
	SpringUtils.AssertMatches(t, ".*:105: this is an error", err.Error())
}
