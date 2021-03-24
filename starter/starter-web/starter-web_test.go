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

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
	"github.com/go-spring/starter-web"
)

func TestSort(t *testing.T) {

	container := func(basePath string) SpringWeb.WebContainer {
		return SpringWeb.NewAbstractContainer(SpringWeb.ContainerConfig{BasePath: basePath})
	}

	testSort := func(input []string, output []string) bool {
		var containers []SpringWeb.WebContainer
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

	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/a/b", "/c", "/a", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/c", "/a/b", "/a", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/c", "/a", "/a/b", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/c", "/a", "/", "/a/b"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/a", "/c/d", "/a/b", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/a", "/a/b", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/a/b", "/a", "/c", "/"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
	SpringUtils.AssertEqual(t, true, testSort([]string{"/c/d", "/a/b", "/c", "/", "/a"}, []string{"/c/d", "/c", "/a/b", "/a", "/"}))
}
