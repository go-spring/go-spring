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

package i18n_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web/i18n"
)

func init() {

	err := i18n.RegisterLanguage("zh-CN", map[string]string{
		"message": "这是一条消息",
	})
	util.Panic(err).When(err != nil)

	err = i18n.RegisterLanguage("en-US", map[string]string{
		"message": "this is a message",
	})
	util.Panic(err).When(err != nil)
}

func TestGet(t *testing.T) {
	assert.Equal(t, i18n.Get("message"), "这是一条消息")
	assert.Equal(t, i18n.Get("message", i18n.Language("en-US")), "this is a message")
	assert.Equal(t, i18n.Get("message", i18n.Language("fr")), "")
}
