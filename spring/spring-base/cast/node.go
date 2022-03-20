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

package cast

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	nilNode = &NilNode{}
)

type Node interface {
	Clone() Node
	JSON() string
	Value() interface{}
}

type NilNode struct{}

func NewNilNode() *NilNode {
	return nilNode
}

func (n *NilNode) Clone() Node {
	return nilNode
}

func (n *NilNode) JSON() string {
	return "null"
}

func (n *NilNode) Value() interface{} {
	return nil
}

type ValueNode struct {
	Data string
}

func NewValueNode(data string) *ValueNode {
	return &ValueNode{Data: data}
}

func (n *ValueNode) Clone() Node {
	return &ValueNode{Data: n.Data}
}

func (n *ValueNode) JSON() string {
	return strconv.Quote(n.Data)
}

func (n *ValueNode) Value() interface{} {
	return n.Data
}

type MapNode struct {
	Data map[string]Node
}

func NewMapNode() *MapNode {
	return &MapNode{
		Data: make(map[string]Node),
	}
}

func (n *MapNode) Get(key string) Node {
	return n.Data[key]
}

func (n *MapNode) Set(key string, node Node) {
	n.Data[key] = node
}

func (n *MapNode) Clone() Node {
	m := make(map[string]Node, len(n.Data))
	for key, value := range n.Data {
		m[key] = value.Clone()
	}
	return &MapNode{Data: m}
}

func (n *MapNode) JSON() string {
	var buf bytes.Buffer
	buf.WriteByte('{')
	index := 0
	count := len(n.Data)
	for key, value := range n.Data {
		buf.WriteString(strconv.Quote(key))
		buf.WriteByte(':')
		buf.WriteString(value.JSON())
		if index < count-1 {
			buf.WriteByte(',')
		}
		index++
	}
	buf.WriteByte('}')
	return buf.String()
}

func (n *MapNode) Value() interface{} {
	m := make(map[string]interface{}, len(n.Data))
	for key, value := range n.Data {
		m[key] = value.Value()
	}
	return m
}

type ArrayNode struct {
	Data []Node
}

func NewArrayNode() *ArrayNode {
	return &ArrayNode{}
}

func (n *ArrayNode) Get(index int) Node {
	if index >= len(n.Data) {
		return nil
	}
	return n.Data[index]
}

func (n *ArrayNode) Set(index int, node Node) {
	if index < len(n.Data) {
		n.Data[index] = node
		return
	}
	data := make([]Node, index+1, index+1)
	for i := 0; i < len(data); i++ {
		data[i] = NewNilNode()
	}
	for i, v := range n.Data {
		data[i] = v
	}
	data[index] = node
	n.Data = data
}

func (n *ArrayNode) Clone() Node {
	arr := make([]Node, len(n.Data))
	for i, value := range n.Data {
		arr[i] = value.Clone()
	}
	return &ArrayNode{Data: arr}
}

func (n *ArrayNode) JSON() string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	count := len(n.Data)
	for i, value := range n.Data {
		buf.WriteString(value.JSON())
		if i < count-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteByte(']')
	return buf.String()
}

func (n *ArrayNode) Value() interface{} {
	arr := make([]interface{}, len(n.Data))
	for i, value := range n.Data {
		arr[i] = value.Value()
	}
	return arr
}

func MergeNode(a, b Node) (Node, error) {
	c := a.Clone()
	err := mergeNode(rootKey, c, b)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func mergeNode(prefix string, a, b Node) error {
	switch v := a.(type) {
	case *MapNode:
		return mergeMapNode(prefix, v, b)
	case *ArrayNode:
		return mergeArrayNode(prefix, v, b)
	case *ValueNode:
		return mergeValueNode(prefix, v, b)
	case *NilNode:
		return mergeNilNode(prefix, v, b)
	default:
		return errors.New("error node type")
	}
}

func mergeMapNode(prefix string, a *MapNode, b Node) error {
	switch v := b.(type) {
	case *MapNode:
		for key, nodeB := range v.Data {
			nodeA := a.Get(key)
			if nodeA == nil {
				a.Set(key, nodeB)
				continue
			}
			if strings.Contains(key, ".") {
				key = "[" + key + "]"
			}
			err := mergeNode(prefix+"."+key, nodeA, nodeB)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		nodeA := prefix + ":" + a.JSON()
		nodeB := prefix + ":" + b.JSON()
		return fmt.Errorf("conf %s conflicts with %s", nodeA, nodeB)
	}
}

func mergeArrayNode(prefix string, a *ArrayNode, b Node) error {
	switch v := b.(type) {
	case *ArrayNode:
		for i := len(v.Data) - 1; i >= 0; i-- {
			nodeB := v.Get(i)
			nodeA := a.Get(i)
			if nodeA == nil {
				a.Set(i, nodeB)
				continue
			}
			err := mergeNode(prefix+"["+strconv.Itoa(i)+"]", nodeA, nodeB)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		nodeA := prefix + ":" + a.JSON()
		nodeB := prefix + ":" + b.JSON()
		return fmt.Errorf("conf %s conflicts with %s", nodeA, nodeB)
	}
}

func mergeValueNode(prefix string, a *ValueNode, b Node) error {
	switch v := b.(type) {
	case *ValueNode:
		a.Data = v.Data
		return nil
	default:
		nodeA := prefix + ":" + a.JSON()
		nodeB := prefix + ":" + b.JSON()
		return fmt.Errorf("conf %s conflicts with %s", nodeA, nodeB)
	}
}

func mergeNilNode(prefix string, a *NilNode, b Node) error {
	switch b.(type) {
	case *NilNode:
		return nil
	default:
		nodeA := prefix + ":" + a.JSON()
		nodeB := prefix + ":" + b.JSON()
		return fmt.Errorf("conf %s conflicts with %s", nodeA, nodeB)
	}
}
