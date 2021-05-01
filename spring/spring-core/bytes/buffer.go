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

package bytes

import (
	"bytes"

	"github.com/spf13/cast"
)

type Buffer struct{ *bytes.Buffer }

func NewBuffer(buf []byte) *Buffer {
	return &Buffer{bytes.NewBuffer(buf)}
}

func NewBufferString(s string) *Buffer {
	return &Buffer{bytes.NewBufferString(s)}
}

func (b *Buffer) WriteInt(i int) error {
	_, err := b.WriteString(cast.ToString(i))
	return err
}

func (b *Buffer) WriteFloat64(i float64) error {
	_, err := b.WriteString(cast.ToString(i))
	return err
}
