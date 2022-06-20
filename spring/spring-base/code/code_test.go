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

	"github.com/go-spring/spring-base/code"
)

func f1() ([]string, time.Duration) {
	ret, cost := f2()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f2() ([]string, time.Duration) {
	ret, cost := f3()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f3() ([]string, time.Duration) {
	ret, cost := f4()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f4() ([]string, time.Duration) {
	ret, cost := f5()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f5() ([]string, time.Duration) {
	ret, cost := f6()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f6() ([]string, time.Duration) {
	ret, cost := f7()
	start := time.Now()
	fileLine := code.FileLine()
	cost += time.Since(start)
	return append(ret, fileLine), cost
}

func f7() ([]string, time.Duration) {

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
		ret, cost := f1()
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
