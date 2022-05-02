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

package replay

import (
	"testing"

	"github.com/go-spring/spring-core/redis/test/cases"
)

func TestBitCount(t *testing.T) {
	RunCase(t, cases.BitCount)
}

func TestBitOpAnd(t *testing.T) {
	RunCase(t, cases.BitOpAnd)
}

func TestBitPos(t *testing.T) {
	RunCase(t, cases.BitPos)
}

func TestGetBit(t *testing.T) {
	RunCase(t, cases.GetBit)
}

func TestSetBit(t *testing.T) {
	RunCase(t, cases.SetBit)
}
