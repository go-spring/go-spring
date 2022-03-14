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

package conf

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/internal/cast"
)

var (
	nilNode = &NilNode{}
)

type Node interface {
	Clone() Node
}

type NilNode struct{}

func NewNilNode() *NilNode {
	return nilNode
}

func (n *NilNode) Clone() Node {
	return nilNode
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

type MapNode struct {
	Data map[string]Node
}

func NewMapNode() *MapNode {
	return &MapNode{
		Data: make(map[string]Node),
	}
}

func (n *MapNode) Clone() Node {
	m := make(map[string]Node, len(n.Data))
	for key, value := range n.Data {
		m[key] = value.Clone()
	}
	return &MapNode{Data: m}
}

func (n *MapNode) Get(key string) Node {
	return n.Data[key]
}

func (n *MapNode) Set(key string, node Node) {
	n.Data[key] = node
}

type ArrayNode struct {
	Data []Node
}

func NewArrayNode() *ArrayNode {
	return &ArrayNode{}
}

func (n *ArrayNode) Clone() Node {
	m := make([]Node, len(n.Data))
	for i, value := range n.Data {
		m[i] = value.Clone()
	}
	return &ArrayNode{Data: m}
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
	for i, v := range n.Data {
		data[i] = v
	}
	data[index] = node
	n.Data = data
}

func keyToPath(key string) []string {
	var buf bytes.Buffer
	for _, c := range key {
		switch c {
		case '[':
			buf.WriteByte('.')
		case ']':
		default:
			buf.WriteByte(byte(c))
		}
	}
	return strings.Split(buf.String(), ".")
}

func Parse(node *Node, value interface{}) error {
	switch value.(type) {
	case nil:
		*node = NewNilNode()
		return nil
	default:
		switch v := reflect.ValueOf(value); v.Kind() {
		case reflect.Map:
			if v.Len() == 0 {
				*node = NewMapNode()
				return nil
			}
			iter := v.MapRange()
			for iter.Next() {
				var pathNode Node
				err := Parse(&pathNode, iter.Value().Interface())
				if err != nil {
					return err
				}
				srcKey, err := cast.ToStringE(iter.Key().Interface())
				if err != nil {
					return err
				}
				path := keyToPath(srcKey)
				err = buildNode(node, srcKey, path, pathNode)
				if err != nil {
					return err
				}
			}
			return nil
		case reflect.Array, reflect.Slice:
			retNode := NewArrayNode()
			for i := v.Len() - 1; i >= 0; i-- {
				var tmpNode Node
				err := Parse(&tmpNode, v.Index(i).Interface())
				if err != nil {
					return err
				}
				retNode.Set(i, tmpNode)
			}
			*node = retNode
			return nil
		case reflect.Bool:
			*node = NewValueNode(strconv.FormatBool(v.Bool()))
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			*node = NewValueNode(strconv.FormatInt(v.Int(), 10))
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			*node = NewValueNode(strconv.FormatUint(v.Uint(), 10))
			return nil
		case reflect.Float32, reflect.Float64:
			*node = NewValueNode(strconv.FormatFloat(v.Float(), 'f', -1, 64))
			return nil
		case reflect.String:
			*node = NewValueNode(v.String())
			return nil
		default:
			return fmt.Errorf("unsupported value kind %s", v.Kind())
		}
	}
}

func buildNode(node *Node, srcKey string, path []string, pathNode Node) error {
	switch n, err := strconv.ParseInt(path[0], 10, 64); err != nil {
	case true:
		var (
			valNode  Node
			tempNode *MapNode
		)
		switch v := (*node).(type) {
		case nil:
			tempNode = NewMapNode()
			*node = tempNode
		case *MapNode:
			tempNode = v
			valNode = v.Get(path[0])
		default:
			return fmt.Errorf("key %q conflicts with other key", srcKey)
		}
		if len(path) == 1 {
			if valNode != nil {
				return fmt.Errorf("key %q conflicts with other key", srcKey)
			}
			valNode = pathNode
		} else {
			err = buildNode(&valNode, srcKey, path[1:], pathNode)
			if err != nil {
				return err
			}
		}
		tempNode.Set(path[0], valNode)
		return nil
	default:
		if n < 0 {
			return fmt.Errorf("invalid indexed key %s", srcKey)
		}
		var (
			valNode  Node
			tempNode *ArrayNode
		)
		switch v := (*node).(type) {
		case nil:
			tempNode = NewArrayNode()
			*node = tempNode
		case *ArrayNode:
			tempNode = v
			valNode = v.Get(int(n))
		default:
			return fmt.Errorf("key %q conflicts with other key", srcKey)
		}
		if len(path) == 1 {
			if valNode != nil {
				return fmt.Errorf("key %q conflicts with other key", srcKey)
			}
			valNode = pathNode
		} else {
			err = buildNode(&valNode, srcKey, path[1:], pathNode)
			if err != nil {
				return err
			}
		}
		tempNode.Set(int(n), valNode)
		return nil
	}
}
