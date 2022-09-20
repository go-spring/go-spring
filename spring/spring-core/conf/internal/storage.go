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

package internal

import (
	"fmt"
	"sort"
	"strings"
)

type nodeType int

const (
	nodeTypeNil nodeType = iota
	nodeTypeValue
	nodeTypeMap
	nodeTypeList
)

type treeNode struct {
	node nodeType
	data interface{}
}

// Copy returns a new copy of the *treeNode object.
func (t *treeNode) Copy() *treeNode {
	r := &treeNode{
		node: t.node,
	}
	switch m := t.data.(type) {
	case map[string]*treeNode:
		c := make(map[string]*treeNode)
		for k, v := range m {
			c[k] = v.Copy()
		}
		r.data = c
	default:
		r.data = t.data
	}
	return r
}

// Storage stores data in the properties format.
type Storage struct {
	tree *treeNode
	data map[string]string
}

// NewStorage returns a new *Storage object.
func NewStorage() *Storage {
	return &Storage{
		tree: &treeNode{
			node: nodeTypeMap,
			data: make(map[string]*treeNode),
		},
		data: make(map[string]string),
	}
}

// Copy returns a new copy of the *Storage object.
func (s *Storage) Copy() *Storage {
	return &Storage{
		tree: s.tree.Copy(),
		data: s.Data(),
	}
}

// Data returns key-value pairs of the properties.
func (s *Storage) Data() map[string]string {
	if len(s.data) == 0 {
		return nil
	}
	m := make(map[string]string)
	for k, v := range s.data {
		m[k] = v
	}
	return m
}

// Keys returns keys of the properties.
func (s *Storage) Keys() []string {
	if len(s.data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SubKeys returns the sub keys of the key item.
func (s *Storage) SubKeys(key string) ([]string, error) {
	path, err := SplitPath(key)
	if err != nil {
		return nil, err
	}
	tree := s.tree
	for i, pathNode := range path {
		m := tree.data.(map[string]*treeNode)
		v, ok := m[pathNode.Elem]
		if !ok || v.node == nodeTypeNil {
			return nil, nil
		}
		switch v.node {
		case nodeTypeValue:
			return nil, fmt.Errorf("property '%s' is value", GenPath(path[:i+1]))
		case nodeTypeList, nodeTypeMap:
			tree = v
		}
	}
	var keys []string
	for k := range tree.data.(map[string]*treeNode) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// Has returns whether the key exists.
func (s *Storage) Has(key string) bool {
	path, err := SplitPath(key)
	if err != nil {
		return false
	}
	tree := s.tree
	for i, node := range path {
		m := tree.data.(map[string]*treeNode)
		v, ok := m[node.Elem]
		if !ok {
			return false
		}
		if v.node == nodeTypeNil || v.node == nodeTypeValue {
			return i == len(path)-1
		}
		tree = v
	}
	return true
}

// Get returns the key's value.
func (s *Storage) Get(key string) string {
	val, _ := s.data[key]
	return val
}

// Set stores the key and its value.
func (s *Storage) Set(key, val string) error {
	val = strings.TrimSpace(val)
	err := s.buildTree(key, val)
	if err != nil {
		return err
	}
	path, _ := SplitPath(key)
	for i := range path {
		k := GenPath(path[:i+1])
		if _, ok := s.data[k]; ok {
			delete(s.data, k)
		}
	}
	s.data[key] = val
	return nil
}

func (s *Storage) buildTree(key, val string) error {
	path, err := SplitPath(key)
	if err != nil {
		return err
	}
	if path[0].Type == PathTypeIndex {
		return fmt.Errorf("invalid key '%s'", key)
	}
	tree := s.tree
	for i, pathNode := range path {
		if tree.node == nodeTypeMap {
			if pathNode.Type != PathTypeKey {
				return fmt.Errorf("property '%s' is a map but '%s' wants other type", GenPath(path[:i]), key)
			}
		}
		m := tree.data.(map[string]*treeNode)
		v, ok := m[pathNode.Elem]
		if !ok || v.node == nodeTypeNil {
			if i < len(path)-1 {
				n := &treeNode{
					data: make(map[string]*treeNode),
				}
				if pathNode.Type == PathTypeKey {
					if path[i+1].Type == PathTypeIndex {
						n.node = nodeTypeList
					} else {
						n.node = nodeTypeMap
					}
				} else if pathNode.Type == PathTypeIndex {
					n.node = nodeTypeList
				}
				m[pathNode.Elem] = n
				tree = n
				continue
			}
			if val == "" {
				m[pathNode.Elem] = &treeNode{
					node: nodeTypeNil,
					data: nodeTypeNil,
				}
				continue
			}
			m[pathNode.Elem] = &treeNode{
				node: nodeTypeValue,
				data: nodeTypeValue,
			}
			continue
		}
		switch v.node {
		case nodeTypeMap:
			if i < len(path)-1 {
				tree = v
				continue
			}
			if val == "" {
				s.remove(key, v)
				v.data = make(map[string]*treeNode)
				return nil
			}
			return fmt.Errorf("property '%s' is a map but '%s' wants other type", GenPath(path[:i+1]), key)
		case nodeTypeList:
			if pathNode.Type != PathTypeIndex {
				if i < len(path)-1 && path[i+1].Type != PathTypeIndex {
					return fmt.Errorf("property '%s' is a list but '%s' wants other type", GenPath(path[:i+1]), key)
				}
			}
			if i < len(path)-1 {
				tree = v
				continue
			}
			if val == "" {
				s.remove(key, v)
				v.data = make(map[string]*treeNode)
				return nil
			}
			return fmt.Errorf("property '%s' is a list but '%s' wants other type", GenPath(path[:i+1]), key)
		case nodeTypeValue:
			if i == len(path)-1 {
				if val == "" {
					s.remove(key, v)
				}
				return nil
			}
			return fmt.Errorf("property '%s' is a value but '%s' wants other type", GenPath(path[:i+1]), key)
		}
	}
	return nil
}

func (s *Storage) remove(key string, tree *treeNode) {
	switch tree.node {
	case nodeTypeValue:
		delete(s.data, key)
	case nodeTypeMap:
		m := tree.data.(map[string]*treeNode)
		for k, v := range m {
			s.remove(key+"."+k, v)
		}
	case nodeTypeList:
		m := tree.data.(map[string]*treeNode)
		for k, v := range m {
			s.remove(key+"["+k+"]", v)
		}
	}
}
