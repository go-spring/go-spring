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
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-spring/spring-core/conf/k8s"
	"github.com/go-spring/spring-core/conf/local"
)

func init() {
	NewScheme(local.Scheme, "file", "local")
	NewScheme(k8s.Scheme, "k8s")
}

const MaxSchemeLength = 16

// Scheme 从扩展后的 URL 读取文件数据，同时返回文件的扩展名，读取失败时返回错误。
type Scheme func(u *url.URL) (_ []byte, ext string, _ error)

var schemeMap = make(map[string]Scheme)

func NewScheme(scheme Scheme, name ...string) {
	for _, s := range name {
		schemeMap[s] = scheme
	}
}

// ReadFile 从一个扩展的 URI 读取文件数据，标准的 URI 规则为
// scheme:[//[user:password@]host[:port]][/]path[?query][#fragment]
// 常见的 scheme 有 file://、http://、ftp://、mysql://、redis:// 等，
// 进入微服务时代后还有 etcd://、consul:// 等，go-spring 为 kubernetes
// 的 config-map 增加了一种新的 scheme —— k8s:// 。
// 有些 scheme 意味着只能访问本地资源，有些 scheme 意味着只能访问远程资源，
// 有些 scheme 则意味着既能访问本地资源又能访问远程资源。标准的 URI 在读取
// 本地资源时，由于没有 host 信息，一般会简化成 file:///usr/bin/go 这样
// 的形式。但是这种形式既不好写，也不容易记。因此 go-spring 在 URI 的基础上
// 扩展了本地资源和远程资源的区分规则。
// go-spring 扩展后的 URI 规则是：当表示一个本地相对路径的资源时，scheme
// 后面仅需一个分号，即 scheme:relative/path/to/file[#fragment]；当表示
// 一个本地绝对路径的资源时，除了标准的 URI 规则即三分号形式外，也支持 scheme
// 后面一个分号一个左斜杠的形式，即 scheme:/absolute/path/to/file[?query]
// [#fragment]；当表示一个远程地址的资源时，使用标准的 URI 规则。扩展后的规
// 则比较符合直觉，很容易记。
// 像 etcd、consul、config-map 这些 K-V (property-like) 形式的配置中心，
// 在某些情况下可能表现出单一文件的特征，有些情况下则会表现出文件包的特征。当它
// 们表示单一文件时，URI 最多只能用到 query 之前的部分，而当它们表示文件包时，
// 想要获取文件包中的文件则使用 fragment 部分。
func ReadFile(fileURL string) ([]byte, string, error) {

	u, err := parseURL(fileURL)
	if err != nil {
		return nil, "", err
	}

	if u.Path == "" {
		return nil, "", errors.New("error path")
	}

	if u.Scheme == "" {
		u.Scheme = "file"
	}

	s, ok := schemeMap[u.Scheme]
	if !ok {
		return nil, "", fmt.Errorf("can't find scheme %s", u.Scheme)
	}

	return s(u)
}

// trimScheme 返回 fileURL 的 scheme 和去掉 scheme 的部分。
func trimScheme(fileURL string) (scheme string, rest string) {

	n := MaxSchemeLength + 1
	if n > len(fileURL) {
		n = len(fileURL)
	}

	i := strings.Index(fileURL[:n], ":")
	if i < 0 {
		return "", fileURL
	}
	if i == len(fileURL)-1 {
		return fileURL[:i], ""
	}
	return fileURL[:i], fileURL[i+1:]
}

// parseURL 将 fileURL 解析成 url.URL 对象，使用扩展后的 URI 规则。
func parseURL(fileURL string) (u *url.URL, err error) {

	scheme, rest := trimScheme(fileURL)
	if rest == "" {
		return nil, errors.New("error path")
	}

	if strings.HasPrefix(rest, "//") {
		u, err = url.Parse(fileURL)
	} else {
		u, err = url.Parse(rest)
	}

	if err != nil {
		return nil, err
	}

	u.Scheme = scheme
	return u, nil
}
