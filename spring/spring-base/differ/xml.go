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

package differ

type XmlDiffItem struct {
	A interface{}
	B interface{}
}

type XmlDiffResult struct {
	Differs map[string]XmlDiffItem
	Ignores map[string]XmlDiffItem
	Equals  map[string]XmlDiffItem
}

// XmlDiffer XML 比较器。
type XmlDiffer struct {
}

// NewXmlDiffer 创建新的 XML 比较器。
func NewXmlDiffer() *XmlDiffer {
	return &XmlDiffer{}
}

// Diff 比较 a,b 两个 XML 字符串，返回它们异同之处。
func (d *XmlDiffer) Diff(a, b string) *XmlDiffResult {
	return nil
}

// DiffXML 比较 a,b 两个 XML 字符串，返回它们异同之处。
func DiffXML(a, b string) *XmlDiffResult {
	return NewXmlDiffer().Diff(a, b)
}
