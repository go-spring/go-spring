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

// defaultBanner 默认的 banner 字符
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

// printBanner 打印 banner 到控制台
func printBanner(banner string) {
	for _, s := range strings.Split(banner, "\n") {
		fmt.Println(color.Cyan(s))
	}
}
