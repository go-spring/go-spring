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

package assert

type BoolAssertion struct {
	t T
	v bool
}

func ThatBool(t T, v bool) *BoolAssertion {
	return &BoolAssertion{
		t: t,
		v: v,
	}
}

func (a *BoolAssertion) IsTrue(msg ...string) *BoolAssertion {
	a.t.Helper()
	if !a.v {
		fail(a.t, "got false but expect true", msg...)
	}
	return a
}

func (a *BoolAssertion) IsFalse(msg ...string) *BoolAssertion {
	a.t.Helper()
	if a.v {
		fail(a.t, "got true but expect false", msg...)
	}
	return a
}
