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

package web

import (
	"errors"
	"strings"
)

// 路由风格有 echo、gin 和 {} 三种，
// /a/:b/c/:d/* 这种是 echo 风格；
// /a/:b/c/:d/*e 这种是 gin 风格；
// /a/{b}/c/{e:*} 这种是 {} 风格；
// /a/{b}/c/{*:e} 这也是 {} 风格;
// /a/{b}/c/{*} 这种也是 {} 风格。

type PathStyleEnum int

const (
	EchoPathStyle = PathStyleEnum(0)
	GinPathStyle  = PathStyleEnum(1)
	JavaPathStyle = PathStyleEnum(2)
)

// DefaultWildCardName 默认通配符的名称
const DefaultWildCardName = "@_@"

// pathStyle URL 地址风格
type pathStyle interface {
	addKnownPath(path string)
	addNamedPath(path string)
	addWildCard(name string)
	wildCardName() string
	String() string
}

type basePathStyle struct {
	s strings.Builder
	w string // 通配符的名称
}

func (p *basePathStyle) wildCardName() string {
	return p.w
}

func (p *basePathStyle) String() string {
	return p.s.String()
}

// echoPathStyle Echo 地址风格
type echoPathStyle struct {
	basePathStyle
}

func (p *echoPathStyle) addKnownPath(path string) {
	p.s.WriteString("/" + path)
}

func (p *echoPathStyle) addNamedPath(path string) {
	p.s.WriteString("/:" + path)
}

func (p *echoPathStyle) addWildCard(name string) {
	p.s.WriteString("/*")
	p.w = name
}

// ginPathStyle Gin 地址风格
type ginPathStyle struct {
	basePathStyle
}

func (p *ginPathStyle) addKnownPath(path string) {
	p.s.WriteString("/" + path)
}

func (p *ginPathStyle) addNamedPath(path string) {
	p.s.WriteString("/:" + path)
}

func (p *ginPathStyle) addWildCard(name string) {
	if name == "" { // gin 的路由需要指定一个名称
		name = DefaultWildCardName
	}
	p.s.WriteString("/*" + name)
	p.w = name
}

// javaPathStyle {} 地址风格
type javaPathStyle struct {
	basePathStyle
}

func (p *javaPathStyle) addKnownPath(path string) {
	p.s.WriteString("/" + path)
}

func (p *javaPathStyle) addNamedPath(path string) {
	p.s.WriteString("/{" + path + "}")
}

func (p *javaPathStyle) addWildCard(name string) {
	if name != "" {
		p.s.WriteString("/{*:" + name + "}")
	} else {
		p.s.WriteString("/{*}")
	}
	p.w = name
}

// ToPathStyle 将 URL 转换为指定风格的表示形式
func ToPathStyle(path string, style PathStyleEnum) (string, string) {

	var p pathStyle
	switch style {
	case EchoPathStyle:
		p = &echoPathStyle{}
	case GinPathStyle:
		p = &ginPathStyle{}
	case JavaPathStyle:
		p = &javaPathStyle{}
	default:
		panic(errors.New("error path style"))
	}

	// 去掉开始的 / 字符，后面好计算
	if path[0] == '/' {
		path = path[1:]
	}

	for _, s := range strings.Split(path, "/") {

		// 尾部的 '/' 特殊处理
		if len(s) == 0 {
			p.addKnownPath(s)
			continue
		}

		switch s[0] {
		case '{':
			if s[len(s)-1] != '}' {
				panic(errors.New("error url path"))
			}
			if ss := strings.Split(s[1:len(s)-1], ":"); len(ss) > 1 {
				if ss[0] == "*" {
					p.addWildCard(ss[1])
				} else if ss[1] == "*" {
					p.addWildCard(ss[0])
				} else {
					panic(errors.New("error url path"))
				}
			} else if s[1] == '*' {
				p.addWildCard(s[2 : len(s)-1])
			} else {
				p.addNamedPath(s[1 : len(s)-1])
			}
		case '*':
			if s == "*" {
				p.addWildCard("")
			} else {
				p.addWildCard(s[1:])
			}
		case ':':
			p.addNamedPath(s[1:])
		default:
			p.addKnownPath(s)
		}
	}
	return p.String(), p.wildCardName()
}
