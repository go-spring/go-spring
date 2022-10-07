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

package log

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
)

var (
	readers = map[string]Reader{}
)

func init() {
	RegisterReader(new(XMLReader), ".xml")
}

type Node struct {
	Label      string
	Children   []*Node
	Attributes map[string]string
}

func (node *Node) child(label string) *Node {
	for _, c := range node.Children {
		if c.Label == label {
			return c
		}
	}
	return nil
}

// Reader 配置项解析器。
type Reader interface {
	Read(b []byte) (*Node, error)
}

// RegisterReader 注册配置项解析器。
func RegisterReader(r Reader, ext ...string) {
	for _, s := range ext {
		readers[s] = r
	}
}

type XMLReader struct{}

func (r *XMLReader) Read(b []byte) (*Node, error) {
	stack := []*Node{{Label: "<<STACK>>"}}
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			curr := &Node{
				Label:      t.Name.Local,
				Attributes: make(map[string]string),
			}
			for _, attr := range t.Attr {
				curr.Attributes[attr.Name.Local] = attr.Value
			}
			stack = append(stack, curr)
		case xml.EndElement:
			curr := stack[len(stack)-1]
			parent := stack[len(stack)-2]
			parent.Children = append(parent.Children, curr)
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack[0].Children) == 0 {
		return nil, errors.New("error xml config file")
	}
	return stack[0].Children[0], nil
}
