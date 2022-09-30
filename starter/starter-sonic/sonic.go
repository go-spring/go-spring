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

package StarterSonic

import (
	"io"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
	"github.com/go-spring/spring-base/json"
)

func init() {
	json.MarshalFunc = sonic.Marshal
	json.MarshalIndentFunc = func(v interface{}, prefix, indent string) ([]byte, error) {
		return encoder.EncodeIndented(v, prefix, indent, encoder.CompatibleWithStd)
	}
	json.UnmarshalFunc = sonic.Unmarshal
	json.NewEncoderFunc = func(w io.Writer) json.Encoder {
		return encoder.NewStreamEncoder(w)
	}
	json.NewDecoderFunc = func(r io.Reader) json.Decoder {
		return decoder.NewStreamDecoder(r)
	}
}
