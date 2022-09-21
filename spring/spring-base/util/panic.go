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
	"fmt"
)

// PanicCond panic condition.
type PanicCond struct {
	fn func() interface{}
}

// When throws a panic when `isPanic` is true.
func (p *PanicCond) When(isPanic bool) {
	if isPanic {
		panic(p.fn())
	}
}

// NewPanicCond returns a panic condition.
func NewPanicCond(fn func() interface{}) *PanicCond {
	return &PanicCond{fn}
}

// Panic returns a panic condition that throws an error.
func Panic(err interface{}) *PanicCond {
	return NewPanicCond(func() interface{} { return err })
}

// Panicf returns a panic condition that throws a formatted error.
func Panicf(format string, a ...interface{}) *PanicCond {
	return NewPanicCond(func() interface{} { return fmt.Errorf(format, a...) })
}
