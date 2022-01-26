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
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/google/uuid"
)

const (
	HTTP  = "HTTP"
	SQL   = "SQL"
	REDIS = "REDIS"
	APCU  = "APCU"
)

var (
	labelStrategyMap = map[string]LabelStrategy{}
)

type LabelStrategy interface {
	GetLabel(str string) string
}

func GetLabelStrategy(protocol string) LabelStrategy {
	strategy, ok := labelStrategyMap[protocol]
	if !ok {
		strategy = LengthLabelStrategy{length: 4}
	}
	return strategy
}

func RegisterLabelStrategy(protocol string, labelStrategy LabelStrategy) {
	labelStrategyMap[protocol] = labelStrategy
}

type SplitLabelStrategy struct {
	split string // 分隔符
	count int    // 前几段
}

func (s SplitLabelStrategy) GetLabel(str string) string {
	ss := strings.SplitN(str, s.split, s.count+1)
	if len(ss) < s.count {
		return strings.Join(ss, "@")
	}
	return strings.Join(ss[0:s.count], "@")
}

type LengthLabelStrategy struct {
	length int // 固定长度
}

func (s LengthLabelStrategy) GetLabel(str string) string {
	if len(str) < s.length {
		return str
	}
	return str[0:s.length]
}

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
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return
		}
	}
	panic(errors.New("must call under test mode"))
}
