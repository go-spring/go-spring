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

package web

import (
	"bytes"
	"encoding/xml"
)

func BindXML(i interface{}, ctx Context) error {
	body, err := ctx.RequestBody()
	if err != nil {
		return err
	}
	r := bytes.NewReader(body)
	decoder := xml.NewDecoder(r)
	return decoder.Decode(i)
}
