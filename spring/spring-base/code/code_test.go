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

package code_test

import (
	"fmt"
	"runtime/debug"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/code"
)

func f1(t *testing.T) ([]string, time.Duration) {
	ret, cost := f2(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 32)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f2(t *testing.T) ([]string, time.Duration) {
	ret, cost := f3(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 42)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f3(t *testing.T) ([]string, time.Duration) {
	ret, cost := f4(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 52)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f4(t *testing.T) ([]string, time.Duration) {
	ret, cost := f5(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 62)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f5(t *testing.T) ([]string, time.Duration) {
	ret, cost := f6(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 72)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f6(t *testing.T) ([]string, time.Duration) {
	ret, cost := f7(t)
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 82)
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f7(t *testing.T) ([]string, time.Duration) {
	assert.String(t, code.File()).HasSuffix("code/code_test.go")
	assert.Equal(t, code.Line(), 91)
	{
		start := time.Now()
		_ = debug.Stack()
		fmt.Println("\t", "debug.Stack cost", time.Since(start))
	}
	start := time.Now()
	fileLine := code.FileLine()
	cost := time.Since(start)
	return []string{fileLine}, cost
}

func TestFileLine(t *testing.T) {
	for i := 0; i < 5; i++ {
		fmt.Printf("loop %d\n", i)
		ret, cost := f1(t)
		fmt.Println("\t", ret)
		fmt.Println("\t", "all code.FileLine cost", cost)
	}
	//loop 0
	//	 debug.Stack cost 37.794µs
	//	 all code.FileLine cost 14.638µs
	//loop 1
	//	 debug.Stack cost 11.699µs
	//	 all code.FileLine cost 6.398µs
	//loop 2
	//	 debug.Stack cost 20.62µs
	//	 all code.FileLine cost 4.185µs
	//loop 3
	//	 debug.Stack cost 11.736µs
	//	 all code.FileLine cost 4.274µs
	//loop 4
	//	 debug.Stack cost 19.821µs
	//	 all code.FileLine cost 4.061µs
}
