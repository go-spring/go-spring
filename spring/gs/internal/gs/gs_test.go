/*
 * Copyright 2025 The Go-Spring Authors.
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

package gs

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestAs(t *testing.T) {

	t.Run("interface type", func(t *testing.T) {
		s := As[io.Reader]()
		assert.That(t, s.String()).Equal("io.Reader")
	})

	t.Run("non-interface type", func(t *testing.T) {
		assert.Panic(t, func() {
			As[int]()
		}, "T must be interface")
	})
}

func TestBeanSelector(t *testing.T) {

	t.Run("no name", func(t *testing.T) {
		s := BeanIDFor[io.Reader]()
		assert.That(t, s.Name).Equal("")
		assert.That(t, s.Type).Equal(reflect.TypeFor[io.Reader]())
		assert.That(t, fmt.Sprint(s)).Equal("{Type:io.Reader}")
	})

	t.Run("with name", func(t *testing.T) {
		s := BeanIDFor[io.Writer]("writer")
		assert.That(t, s.Name).Equal("writer")
		assert.That(t, s.Type).Equal(reflect.TypeFor[io.Writer]())
		assert.That(t, fmt.Sprint(s)).Equal("{Type:io.Writer,Name:writer}")
	})
}
