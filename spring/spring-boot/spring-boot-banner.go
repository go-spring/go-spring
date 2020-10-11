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

package SpringBoot

import (
	"fmt"
	"strings"

	"github.com/labstack/gommon/color"
)

// defaultBanner 默认的 Banner 字符
const defaultBanner = `
 _______  _______         _______  _______  _______ _________ _        _______ 
(  ____ \(  ___  )       (  ____ \(  ____ )(  ____ )\__   __/( (    /|(  ____ \
| (    \/| (   ) |       | (    \/| (    )|| (    )|   ) (   |  \  ( || (    \/
| |      | |   | | _____ | (_____ | (____)|| (____)|   | |   |   \ | || |      
| | ____ | |   | |(_____)(_____  )|  _____)|     __)   | |   | (\ \) || | ____ 
| | \_  )| |   | |             ) || (      | (\ (      | |   | | \   || | \_  )
| (___) || (___) |       /\____) || )      | ) \ \_____) (___| )  \  || (___) |
(_______)(_______)       \_______)|/       |/   \__/\_______/|/    )_)(_______)
`

// version 版本信息
const version = `go-spring@v1.0.5    http://go-spring.com/`

type BannerMode int

const (
	BannerModeOff     BannerMode = 0
	BannerModeConsole BannerMode = 1
)

// customBanner 自定义 Banner 字符串
var customBanner = ""

// SetBanner 设置自定义 Banner 字符串
func SetBanner(banner string) {
	customBanner = banner
}

// printBanner 打印 Banner 到控制台
func printBanner(banner string) {

	// 确保 Banner 前面有空行
	if banner[0] != '\n' {
		fmt.Println()
	}

	maxLength := 0
	for _, s := range strings.Split(banner, "\n") {
		fmt.Println(color.Cyan(s))
		if len(s) > maxLength {
			maxLength = len(s)
		}
	}

	// 确保 Banner 后面有空行
	if banner[len(banner)-1] != '\n' {
		fmt.Println()
	}

	var padding []byte
	if n := (maxLength - len(version)) / 2; n > 0 {
		padding = make([]byte, n)
		for i := range padding {
			padding[i] = ' '
		}
	}
	fmt.Println(string(padding) + version + "\n")
}
