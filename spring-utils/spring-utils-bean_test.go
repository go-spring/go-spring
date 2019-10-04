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

package SpringUtils

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestCopyBean(t *testing.T) {

	type SrcBase1 struct {
		B1 int
	}

	type SrcBase2 struct {
		B1 int
	}

	var src struct {
		SrcBase1

		B SrcBase1

		F1 int
		F2 int

		SrcBase2

		Ft struct {
			Ft1 int
			Ft3 int
		}

		F3 string
	}

	// 存在同名字段时序列化失效
	src.SrcBase1.B1 = 7
	src.SrcBase2.B1 = 3

	src.B.B1 = 70

	src.F1 = 9
	src.F2 = 13
	src.Ft.Ft1 = 27
	src.Ft.Ft3 = 37

	src.F3 = "this is test"

	b, _ := json.MarshalIndent(src, "", "  ")
	fmt.Println(string(b))

	type DestBase1 struct {
		B1 int
	}

	type DestBase2 struct {
		B1 int
	}

	var dest struct {
		DestBase2
		DestBase1

		B DestBase1

		F1 int
		F2 int32

		Ft struct {
			Ft1 int
			Ft2 int
		}

		F3 string
	}

	times := 50000

	start := time.Now()
	for i := 0; i < times; i++ {
		CopyBean(&src, &dest)
	}

	fmt.Println(dest)
	fmt.Println(time.Now().Sub(start).String())

	start = time.Now()
	for i := 0; i < times; i++ {
		CopyBeanUseJson(&src, &dest)
	}

	fmt.Println(dest)
	fmt.Println(time.Now().Sub(start).String())
}
