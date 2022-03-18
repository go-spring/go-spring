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
	"sort"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/internal/cast"
)

var (
	nilNode = &NilNode{}
)

type Node interface {
	Clone() Node
	JSON() string
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

func (n *ArrayNode) JSON() string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	count := len(n.Data)
	for i, value := range n.Data {
		if value == nil {
			buf.WriteString("null")
		} else {
			buf.WriteString(value.JSON())
		}
		if i < count-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteByte(']')
	return buf.String()
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

func Parse(value interface{}) (Node, error) {
	switch value.(type) {
	case nil:
		return NewNilNode(), nil
	default:
		switch v := reflect.ValueOf(value); v.Kind() {
		case reflect.Map:
			if v.Len() == 0 {
				return NewMapNode(), nil
			}
			keys := make([]string, v.Len())
			for i, k := range v.MapKeys() {
				strKey, err := cast.ToStringE(k.Interface())
				if err != nil {
					return nil, err
				}
				keys[i] = strKey
			}
			sort.Strings(keys)
			var retNode Node
			for _, key := range keys {
				val := v.MapIndex(reflect.ValueOf(key))
				pathNode, err := Parse(val.Interface())
				if err != nil {
					return nil, err
				}
				err = buildNode(&retNode, keyToPath(key), 0, pathNode)
				if err != nil {
					return nil, err
				}
			}
			return retNode, nil
		case reflect.Array, reflect.Slice:
			retNode := NewArrayNode()
			for i := v.Len() - 1; i >= 0; i-- {
				tmpNode, err := Parse(v.Index(i).Interface())
				if err != nil {
					return nil, err
				}
				retNode.Set(i, tmpNode)
			}
			return retNode, nil
		case reflect.Bool:
			return NewValueNode(strconv.FormatBool(v.Bool())), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return NewValueNode(strconv.FormatInt(v.Int(), 10)), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return NewValueNode(strconv.FormatUint(v.Uint(), 10)), nil
		case reflect.Float32, reflect.Float64:
			return NewValueNode(strconv.FormatFloat(v.Float(), 'f', -1, 64)), nil
		case reflect.String:
			return NewValueNode(v.String()), nil
		default:
			return nil, fmt.Errorf("unsupported value kind %s", v.Kind())
		}
	}
}

func buildNode(node *Node, path []string, depth int, pathNode Node) error {
	switch n, err := strconv.ParseInt(path[depth], 10, 64); err != nil {
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
			valNode = v.Get(path[depth])
		default:
			nodeConf := strings.Join(path[:depth], ".") + ":" + v.JSON()
			pathNodeConf := strings.Join(path[:depth+1], ".") + ":" + pathNode.JSON()
			return fmt.Errorf("conf %s conflicts with %s", pathNodeConf, nodeConf)
		}
		if depth == len(path)-1 {
			if valNode != nil {
				valNodeConf := strings.Join(path[:depth+1], ".") + ":" + valNode.JSON()
				pathNodeConf := strings.Join(path[:depth+1], ".") + ":" + pathNode.JSON()
				return fmt.Errorf("conf %s conflicts with %s", pathNodeConf, valNodeConf)
			}
			valNode = pathNode
		} else {
			err = buildNode(&valNode, path, depth+1, pathNode)
			if err != nil {
				return err
			}
		}
		tempNode.Set(path[depth], valNode)
		return nil
	default:
		if n < 0 {
			return fmt.Errorf("invalid indexed key %s", strings.Join(path, "."))
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
			nodeConf := strings.Join(path[:depth], ".") + ":" + v.JSON()
			pathNodeConf := strings.Join(path[:depth], ".") + "[" + path[depth] + "]:" + pathNode.JSON()
			return fmt.Errorf("conf %s conflicts with %s", pathNodeConf, nodeConf)
		}
		if depth == len(path)-1 {
			if valNode != nil {
				valNodeConf := strings.Join(path[:depth], ".") + ":" + valNode.JSON()
				pathNodeConf := strings.Join(path[:depth], ".") + "[" + path[depth] + "]:" + pathNode.JSON()
				return fmt.Errorf("conf %s conflicts with %s", pathNodeConf, valNodeConf)
			}
			valNode = pathNode
		} else {
			err = buildNode(&valNode, path, depth+1, pathNode)
			if err != nil {
				return err
			}
		}
		tempNode.Set(int(n), valNode)
		return nil
	}
}
