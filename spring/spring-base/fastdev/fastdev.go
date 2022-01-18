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

package fastdev

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev/internal/json"
	"github.com/google/uuid"
)

const (
	HTTP  = "HTTP"
	REDIS = "REDIS"
	APCU  = "APCU"
	SQL   = "SQL"
)

// NewSessionID 使用 uuid 算法生成新的 Session ID 。
func NewSessionID() string {
	u := uuid.New()
	buf := make([]byte, 32)
	hex.Encode(buf, u[:4])
	hex.Encode(buf[8:12], u[4:6])
	hex.Encode(buf[12:16], u[6:8])
	hex.Encode(buf[16:20], u[8:10])
	hex.Encode(buf[20:], u[10:])
	return string(buf)
}

// CheckTestMode 检查是否是测试模式
func CheckTestMode() {
	var testMode bool
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			testMode = true
			break
		}
	}
	if !testMode {
		panic(errors.New("must call under test mode"))
	}
}

// needQuote 判断是否需要双引号包裹。
func needQuote(s string) bool {
	for _, c := range s {
		switch c {
		case '"', '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			return true
		}
	}
	return len(s) == 0
}

func quoteString(s string) string {
	if needQuote(s) || json.NeedQuote(s) {
		return strconv.Quote(s)
	}
	return s
}

// CmdString 格式化命令行有效的字符串。
func CmdString(args []interface{}) string {
	var buf bytes.Buffer
	for i, arg := range args {
		switch s := arg.(type) {
		case string:
			buf.WriteString(quoteString(s))
		default:
			buf.WriteString(cast.ToString(arg))
		}
		if i < len(args)-1 {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}
