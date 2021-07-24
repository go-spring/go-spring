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

package gsutil_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/gsutil"
	"github.com/go-spring/spring-core/gsutil/testdata"
)

func fnNoArgs() {}

func fnWithArgs(i int) {}

type receiver struct{}

func (r receiver) fnNoArgs() {}

func (r receiver) fnWithArgs(i int) {}

func (r *receiver) ptrFnNoArgs() {}

func (r *receiver) ptrFnWithArgs(i int) {}

func TestFileLine(t *testing.T) {

	fmt.Println(gsutil.FileLine(fnNoArgs))
	fmt.Println(gsutil.FileLine(fnWithArgs))
	fmt.Println(gsutil.FileLine(receiver{}.fnNoArgs))
	fmt.Println(gsutil.FileLine(receiver{}.fnWithArgs))
	fmt.Println(gsutil.FileLine((&receiver{}).ptrFnNoArgs))
	fmt.Println(gsutil.FileLine((&receiver{}).ptrFnWithArgs))
	fmt.Println(gsutil.FileLine(receiver.fnNoArgs))
	fmt.Println(gsutil.FileLine(receiver.fnWithArgs))
	fmt.Println(gsutil.FileLine((*receiver).ptrFnNoArgs))
	fmt.Println(gsutil.FileLine((*receiver).ptrFnWithArgs))

	fmt.Println(gsutil.FileLine(testdata.FnNoArgs))
	fmt.Println(gsutil.FileLine(testdata.FnWithArgs))
	fmt.Println(gsutil.FileLine(testdata.Receiver{}.FnNoArgs))
	fmt.Println(gsutil.FileLine(testdata.Receiver{}.FnWithArgs))
	fmt.Println(gsutil.FileLine((&testdata.Receiver{}).PtrFnNoArgs))
	fmt.Println(gsutil.FileLine((&testdata.Receiver{}).PtrFnWithArgs))
	fmt.Println(gsutil.FileLine(testdata.Receiver.FnNoArgs))
	fmt.Println(gsutil.FileLine(testdata.Receiver.FnWithArgs))
	fmt.Println(gsutil.FileLine((*testdata.Receiver).PtrFnNoArgs))
	fmt.Println(gsutil.FileLine((*testdata.Receiver).PtrFnWithArgs))
}
