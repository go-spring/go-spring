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

package console

import (
	"fmt"
)

const (
	RED    = color("[31m")
	GREEN  = color("[32m")
	YELLOW = color("[33m")
)

type color string

func (c color) Sprint(a ...interface{}) string {
	return "\x1b" + string(c) + fmt.Sprint(a...) + "\x1b[0m"
}

func (c color) Sprintf(format string, a ...interface{}) string {
	return "\x1b" + string(c) + fmt.Sprintf(format, a...) + "\x1b[0m"
}
