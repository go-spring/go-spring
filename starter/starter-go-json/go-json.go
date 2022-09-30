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

package StarterGoJSON

import (
	"io"

	"github.com/go-spring/spring-base/json"
	gojson "github.com/goccy/go-json"
)

func init() {
	json.MarshalFunc = gojson.Marshal
	json.MarshalIndentFunc = gojson.MarshalIndent
	json.UnmarshalFunc = gojson.Unmarshal
	json.NewEncoderFunc = func(w io.Writer) json.Encoder { return gojson.NewEncoder(w) }
	json.NewDecoderFunc = func(r io.Reader) json.Decoder { return gojson.NewDecoder(r) }
}
