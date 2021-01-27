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
	"fmt"
	"testing"

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-utils/testdata"
)

func fnNoArgs() {}

func fnWithArgs(i int) {}

type receiver struct{}

func (r receiver) fnNoArgs() {}

func (r receiver) fnWithArgs(i int) {}

func (r *receiver) ptrFnNoArgs() {}

func (r *receiver) ptrFnWithArgs(i int) {}

func TestFileLine(t *testing.T) {

	fmt.Println(SpringUtils.FileLine(fnNoArgs))
	fmt.Println(SpringUtils.FileLine(fnWithArgs))
	fmt.Println(SpringUtils.FileLine(receiver{}.fnNoArgs))
	fmt.Println(SpringUtils.FileLine(receiver{}.fnWithArgs))
	fmt.Println(SpringUtils.FileLine((&receiver{}).ptrFnNoArgs))
	fmt.Println(SpringUtils.FileLine((&receiver{}).ptrFnWithArgs))
	fmt.Println(SpringUtils.FileLine(receiver.fnNoArgs))
	fmt.Println(SpringUtils.FileLine(receiver.fnWithArgs))
	fmt.Println(SpringUtils.FileLine((*receiver).ptrFnNoArgs))
	fmt.Println(SpringUtils.FileLine((*receiver).ptrFnWithArgs))

	fmt.Println(SpringUtils.FileLine(testdata.FnNoArgs))
	fmt.Println(SpringUtils.FileLine(testdata.FnWithArgs))
	fmt.Println(SpringUtils.FileLine(testdata.Receiver{}.FnNoArgs))
	fmt.Println(SpringUtils.FileLine(testdata.Receiver{}.FnWithArgs))
	fmt.Println(SpringUtils.FileLine((&testdata.Receiver{}).PtrFnNoArgs))
	fmt.Println(SpringUtils.FileLine((&testdata.Receiver{}).PtrFnWithArgs))
	fmt.Println(SpringUtils.FileLine(testdata.Receiver.FnNoArgs))
	fmt.Println(SpringUtils.FileLine(testdata.Receiver.FnWithArgs))
	fmt.Println(SpringUtils.FileLine((*testdata.Receiver).PtrFnNoArgs))
	fmt.Println(SpringUtils.FileLine((*testdata.Receiver).PtrFnWithArgs))
}
