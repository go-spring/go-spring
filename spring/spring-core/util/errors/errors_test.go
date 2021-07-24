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

package errors_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/util/assert"
	"github.com/go-spring/spring-core/util/errors"
)

func TestPanicCond_When(t *testing.T) {
	util.Panic(errors.New("test error")).When(false)

	t.Run("Panic", func(t *testing.T) {
		defer func() {
			assert.Equal(t, recover(), errors.New("reason: panic"))
		}()
		util.Panic(fmt.Errorf("reason: %s", "panic")).When(true)
	})

	t.Run("Panicf", func(t *testing.T) {
		defer func() {
			assert.Equal(t, recover(), errors.New("reason: panicf"))
		}()
		util.Panicf("reason: %s", "panicf").When(true)
	})
}

func TestWithCause(t *testing.T) {

	t.Run("cause is string", func(t *testing.T) {
		err := errors.WithCause("this is a string")
		v := errors.Cause(err)
		assert.Equal(t, v, "this is a string")
	})

	t.Run("cause is error", func(t *testing.T) {
		err := errors.WithCause(errors.New("this is an error"))
		v := errors.Cause(err)
		assert.Equal(t, v, errors.New("this is an error"))
	})

	t.Run("cause is int", func(t *testing.T) {
		err := errors.WithCause(123456)
		v := errors.Cause(err)
		assert.Equal(t, v, 123456)
	})
}

func panic2Error(v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.WithCause(r)
		}
	}()
	panic(v)
}

func TestPanic2Error(t *testing.T) {

	t.Run("panic is string", func(t *testing.T) {
		err := panic2Error("this is a string")
		v := errors.Cause(err)
		assert.Equal(t, v, "this is a string")
	})

	t.Run("panic is error", func(t *testing.T) {
		err := panic2Error(errors.New("this is an error"))
		v := errors.Cause(err)
		assert.Equal(t, v, errors.New("this is an error"))
	})

	t.Run("panic is int", func(t *testing.T) {
		err := panic2Error(123456)
		v := errors.Cause(err)
		assert.Equal(t, v, 123456)
	})
}

func TestErrorWithFileLine(t *testing.T) {

	err := errors.WithFileLine(errors.New("this is an error"), 0)
	assert.Error(t, err, ".*:99: this is an error")

	fnError := func(e error) error {
		return errors.WithFileLine(e, 1)
	}

	err = fnError(errors.New("this is an error"))
	assert.Error(t, err, ".*:106: this is an error")
}
