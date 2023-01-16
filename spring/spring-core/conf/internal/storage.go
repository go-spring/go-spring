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

	"github.com/go-spring/spring-base/util"
)

type nodeType int

const (
	nodeTypeNil nodeType = iota
	nodeTypeValue
	nodeTypeMap
	nodeTypeArray
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
	return util.SortedKeys(s.data)
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
			return nil, fmt.Errorf("property '%s' is value", JoinPath(path[:i+1]))
		case nodeTypeArray, nodeTypeMap:
			tree = v
		}
	}
	m := tree.data.(map[string]*treeNode)
	keys := util.SortedKeys(m)
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
		switch tree.node {
		case nodeTypeArray:
			if node.Type != PathTypeIndex {
				return false
			}
		case nodeTypeMap:
			if node.Type != PathTypeKey {
				return false
			}
		}
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
	tree, err := s.merge(key, val)
	if err != nil {
		return err
	}
	switch tree.node {
	case nodeTypeNil, nodeTypeValue:
		s.data[key] = val
	}
	return nil
}

func (s *Storage) merge(key, val string) (*treeNode, error) {
	path, err := SplitPath(key)
	if err != nil {
		return nil, err
	}
	if path[0].Type == PathTypeIndex {
		return nil, fmt.Errorf("invalid key '%s'", key)
	}
	tree := s.tree
	for i, pathNode := range path {
		if tree.node == nodeTypeMap {
			if pathNode.Type != PathTypeKey {
				return nil, fmt.Errorf("property '%s' is a map but '%s' wants other type", JoinPath(path[:i]), key)
			}
		}
		m := tree.data.(map[string]*treeNode)
		v, ok := m[pathNode.Elem]
		if v != nil && v.node == nodeTypeNil {
			delete(s.data, JoinPath(path[:i+1]))
		}
		if !ok || v.node == nodeTypeNil {
			if i < len(path)-1 {
				n := &treeNode{
					data: make(map[string]*treeNode),
				}
				if path[i+1].Type == PathTypeIndex {
					n.node = nodeTypeArray
				} else {
					n.node = nodeTypeMap
				}
				m[pathNode.Elem] = n
				tree = n
				continue
			}
			if val == "" {
				tree = &treeNode{node: nodeTypeNil}
			} else {
				tree = &treeNode{node: nodeTypeValue}
			}
			m[pathNode.Elem] = tree
			break // break for 100% test
		}
		switch v.node {
		case nodeTypeMap:
			if i < len(path)-1 {
				tree = v
				continue
			}
			if val == "" {
				return v, nil
			}
			return nil, fmt.Errorf("property '%s' is a map but '%s' wants other type", JoinPath(path[:i+1]), key)
		case nodeTypeArray:
			if pathNode.Type != PathTypeIndex {
				if i < len(path)-1 && path[i+1].Type != PathTypeIndex {
					return nil, fmt.Errorf("property '%s' is an array but '%s' wants other type", JoinPath(path[:i+1]), key)
				}
			}
			if i < len(path)-1 {
				tree = v
				continue
			}
			if val == "" {
				return v, nil
			}
			return nil, fmt.Errorf("property '%s' is an array but '%s' wants other type", JoinPath(path[:i+1]), key)
		case nodeTypeValue:
			if i == len(path)-1 {
				return v, nil
			}
			return nil, fmt.Errorf("property '%s' is a value but '%s' wants other type", JoinPath(path[:i+1]), key)
		}
	}
	return tree, nil
}
