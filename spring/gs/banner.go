/*
 * Copyright 2025 The Go-Spring Authors.
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

package gs

import (
	"fmt"
	"strings"
)

var appBanner = `
   ____    ___            ____    ____    ____    ___   _   _    ____ 
  / ___|  / _ \          / ___|  |  _ \  |  _ \  |_ _| | \ | |  / ___|
 | |  _  | | | |  _____  \___ \  | |_) | | |_) |  | |  |  \| | | |  _ 
 | |_| | | |_| | |_____|  ___) | |  __/  |  _ <   | |  | |\  | | |_| |
  \____|  \___/          |____/  |_|     |_| \_\ |___| |_| \_|  \____| 
`

// Banner sets a custom app banner.
func Banner(banner string) {
	appBanner = banner
}

// printBanner prints the app banner.
func printBanner() {
	if len(appBanner) == 0 {
		return
	}

	var sb strings.Builder
	if appBanner[0] != '\n' {
		sb.WriteString("\n")
	}

	maxLength := 0
	for s := range strings.SplitSeq(appBanner, "\n") {
		if len(s) > 0 {
			sb.WriteString("\x1b[36m") // ANSI code for cyan color
			sb.WriteString(s)
			sb.WriteString("\x1b[0m") // ANSI code to reset color
		}
		sb.WriteString("\n")
		if len(s) > maxLength {
			maxLength = len(s)
		}
	}

	if appBanner[len(appBanner)-1] != '\n' {
		sb.WriteString("\n")
	}

	// print version and website
	const info = Version + "  " + Website

	var padding []byte
	if n := (maxLength - len(info)) / 2; n > 0 {
		padding = make([]byte, n)
		for i := range padding {
			padding[i] = ' '
		}
	}
	sb.WriteString(string(padding))
	sb.WriteString(info)
	sb.WriteString("\n")
	fmt.Println(sb.String())
}
