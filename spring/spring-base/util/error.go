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

package util

import (
	"errors"
	"fmt"
)

// ForbiddenMethod throws this error when calling a method is prohibited.
var ForbiddenMethod = errors.New("forbidden method")

// UnimplementedMethod throws this error when calling an unimplemented method.
var UnimplementedMethod = errors.New("unimplemented method")

var WrapFormat = func(err error, fileline string, msg string) error {
	if err == nil {
		return fmt.Errorf("%s %s", fileline, msg)
	}
	return fmt.Errorf("%s %s; %w", fileline, msg, err)
}

// Error returns an error with the file and line.
// The file and line may be calculated at the compile time in the future.
func Error(fileline string, text string) error {
	return WrapFormat(nil, fileline, text)
}

// Errorf returns an error with the file and line.
// The file and line may be calculated at the compile time in the future.
func Errorf(fileline string, format string, a ...interface{}) error {
	return WrapFormat(nil, fileline, fmt.Sprintf(format, a...))
}

// Wrap returns an error with the file and line.
// The file and line may be calculated at the compile time in the future.
func Wrap(err error, fileline string, text string) error {
	return WrapFormat(err, fileline, text)
}

// Wrapf returns an error with the file and line.
// The file and line may be calculated at the compile time in the future.
func Wrapf(err error, fileline string, format string, a ...interface{}) error {
	return WrapFormat(err, fileline, fmt.Sprintf(format, a...))
}
