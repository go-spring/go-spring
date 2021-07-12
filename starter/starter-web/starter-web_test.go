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

package StarterWeb_test

import (
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/starter-web"
)

func TestSort(t *testing.T) {

	container := func(basePath string) web.Container {
		return web.NewAbstractContainer(web.ContainerConfig{BasePath: basePath})
	}

	testSort := func(input []string, output []string) bool {
		var containers []web.Container
		for _, s := range input {
			containers = append(containers, container(s))
		}
		StarterWeb.SortContainers(containers)
		for i, c := range containers {
			if output[i] != c.Config().BasePath {
				return false
			}
		}
		return true
	}

	assert.Equal(t, true, testSort([]string{"/c/d", "/a/b", "/c", "/a", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/c", "/a/b", "/a", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/c", "/a", "/a/b", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/c", "/a", "/", "/a/b"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/a", "/c/d", "/a/b", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/a", "/a/b", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/a/b", "/a", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	assert.Equal(t, true, testSort([]string{"/c/d", "/a/b", "/c", "/", "/a"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
}
