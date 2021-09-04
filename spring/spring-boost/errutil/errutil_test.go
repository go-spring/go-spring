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

package errutil_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-boost/assert"
	"github.com/go-spring/spring-boost/errutil"
)

func TestErrorWithFileLine(t *testing.T) {

	err := errutil.WithFileLine(errors.New("this is an error"), 0)
	assert.Error(t, err, ".*:29: this is an error")

	fnError := func(e error) error {
		return errutil.WithFileLine(e, 1)
	}

	err = fnError(errors.New("this is an error"))
	assert.Error(t, err, ".*:36: this is an error")
}
