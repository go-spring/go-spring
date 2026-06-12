/*
 * Copyright 2024 The Go-Spring Authors.
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

package gs_conf

import (
	"os"
	"strings"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
)

// CommandArgsPrefix defines the environment variable name used to override
// the default option prefix. This allows users to customize the prefix used
// for command-line options if needed.
const CommandArgsPrefix = "GS_ARGS_PREFIX"

// extractCmdArgs extracts command-line parameters as key-value pairs.
//
// Supported formats:
//
//	<prefix> key=value
//	<prefix> key              (defaults to "true")
//	<prefix>key=value
//	<prefix>key               (defaults to "true")
//
// The default prefix is "-D", which can be overridden by the
// environment variable `GS_ARGS_PREFIX`.
//
// Arguments that do not match the configured prefix are ignored.
func extractCmdArgs() (*flatten.Properties, error) {

	p := flatten.NewProperties(nil)
	if len(os.Args) <= 1 {
		return p, nil
	}

	// Determine the option prefix.
	option := "-D"
	if s := strings.TrimSpace(os.Getenv(CommandArgsPrefix)); s != "" {
		option = s
	}

	cmdArgs := os.Args[1:]
	for i := 0; i < len(cmdArgs); i++ {
		var str string
		if cmdArgs[i] == option {
			// separated form: <prefix> key=value
			if i+1 >= len(cmdArgs) {
				return nil, errutil.Explain(nil, "cmd option %s requires an argument", option)
			}
			i++
			str = cmdArgs[i]
		} else if s, ok := strings.CutPrefix(cmdArgs[i], option); ok {
			// inline form: <prefix>key=value
			str = s
		} else {
			// not a Go-Spring command-line option
			continue
		}
		if str = strings.TrimSpace(str); str == "" {
			return nil, errutil.Explain(nil, "cmd option %s requires an argument", option)
		}
		ss := strings.SplitN(str, "=", 2)
		if strings.TrimSpace(ss[0]) == "" {
			return nil, errutil.Explain(nil, "cmd option %s has empty key", option)
		}
		if len(ss) == 1 {
			ss = append(ss, "true")
		}
		p.Set(ss[0], ss[1])
	}
	return p, nil
}
