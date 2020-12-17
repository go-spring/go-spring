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

package SpringCore_test

import (
	"testing"

	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

func TestNewDefaultProperties(t *testing.T) {

	p1 := SpringCore.NewDefaultProperties()
	p1.SetProperty("key_override", "p1")
	p1.SetProperty("key_p1", "p1")

	p2 := SpringCore.NewDefaultProperties()
	p2.SetProperty("key_override", "p2")
	p2.SetProperty("key_p2", "p2")

	p3 := SpringCore.NewDefaultProperties()
	p3.SetProperty("key_override", "p3")
	p3.SetProperty("key_p3", "p3")

	p4 := SpringCore.NewDefaultProperties()
	p4.SetProperty("key_override", "p4")
	p4.SetProperty("key_p4", "p4")

	p5 := SpringCore.NewDefaultProperties()
	p5.SetProperty("key_override", "p5")
	p5.SetProperty("key_p5", "p5")

	l0 := SpringCore.NewPriorityProperties(p2, p1)
	l0 = SpringCore.NewPriorityProperties(p3, l0)
	l0 = SpringCore.NewPriorityProperties(p4, l0)
	l0 = SpringCore.NewPriorityProperties(p5, l0)

	key_override := l0.GetProperty("key_override")
	SpringUtils.AssertEqual(t, key_override, "p5")

	key := l0.GetProperty("key_p1")
	SpringUtils.AssertEqual(t, key, "p1")

	SpringUtils.AssertEqual(t, l0.Depth(), 5)
}

func TestPriorityProperties_InsertBefore(t *testing.T) {

	p1 := SpringCore.NewDefaultProperties()
	p1.SetProperty("key_override", "p1")
	p1.SetProperty("key_p1", "p1")

	p2 := SpringCore.NewDefaultProperties()
	p2.SetProperty("key_override", "p2")
	p2.SetProperty("key_p2", "p2")

	p3 := SpringCore.NewDefaultProperties()
	p3.SetProperty("key_override", "p3")
	p3.SetProperty("key_p3", "p3")

	p4 := SpringCore.NewDefaultProperties()
	p4.SetProperty("key_override", "p4")
	p4.SetProperty("key_p4", "p4")

	p5 := SpringCore.NewDefaultProperties()
	p5.SetProperty("key_override", "p5")
	p5.SetProperty("key_p5", "p5")

	l0 := SpringCore.NewPriorityProperties(p3, p1)
	l0.InsertBefore(p2, p1)
	l0 = SpringCore.NewPriorityProperties(p5, l0)
	l0.InsertBefore(p4, p3)

	key_override := l0.GetProperty("key_override")
	SpringUtils.AssertEqual(t, key_override, "p5")

	key := l0.GetProperty("key_p1")
	SpringUtils.AssertEqual(t, key, "p1")

	SpringUtils.AssertEqual(t, l0.Depth(), 5)
}
