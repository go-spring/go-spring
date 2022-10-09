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

package redis_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-core/redis"
)

func runCase(t *testing.T, c *redis.Case) {
	ctx := context.Background()
	client := redis.NewClient(&driver{})
	client.FlushAll(ctx)
	c.Func(t, ctx, client)
}

type driver struct{}

func (p *driver) Exec(ctx context.Context, args []interface{}) (interface{}, error) {
	str := encodeTTY(args)
	c := exec.Command("/bin/bash", "-c", fmt.Sprintf("redis-cli --csv --quoted-input %s", str))
	output, err := c.CombinedOutput()
	if err != nil {
		fmt.Println(string(output[:len(output)-1]))
		return nil, err
	}
	csv, err := decodeCSV(string(output[:len(output)-1]))
	if err != nil {
		return nil, err
	}
	if len(csv) == 1 {
		if csv[0] == "NULL" {
			return nil, redis.ErrNil()
		}
	} else if len(csv) > 1 {
		if csv[0] == "ERROR" {
			return nil, errors.New(csv[1])
		}
	}
	return &redis.Result{Data: csv}, nil
}

func encodeTTY(data []interface{}) string {
	var buf bytes.Buffer
	for i, arg := range data {
		switch s := arg.(type) {
		case string:
			if c := ttyQuoteCount(s); c > 0 {
				buf.WriteByte('\'')
				buf.WriteString(strconv.Quote(s))
				buf.WriteByte('\'')
			} else {
				buf.WriteString(s)
			}
		default:
			buf.WriteString(cast.ToString(arg))
		}
		if i < len(data)-1 {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}

func ttyQuoteCount(s string) int {
	if len(s) == 0 {
		return 1
	}
	ok := (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_'
	if !ok {
		return 1
	}
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b <= 0x20 || b >= 0x7E {
				// ASCII printable characters (character code 32-127)
				return 1
			}
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			return 2
		}
		i += size
	}
	return 0
}

func decodeCSV(data string) ([]string, error) {
	var (
		ret []string
		buf bytes.Buffer
	)
	for i := 0; ; {
		if i >= len(data) {
			return ret, nil
		}
		buf.Reset()
		var (
			done          bool
			inQuote       bool
			inSingleQuote bool
		)
		for ; !done; i++ {
			if i >= len(data) && (inQuote || inSingleQuote) {
				return nil, errors.New("invalid syntax")
			}
			if c := data[i]; inQuote {
				if c == '\\' && i < len(data)-3 && data[i+1] == 'x' && cast.IsHexDigit(data[i+2]) && cast.IsHexDigit(data[i+3]) {
					b1 := cast.HexDigitToInt(data[i+2]) * 16
					b2 := cast.HexDigitToInt(data[i+3])
					b := byte(b1 + b2)
					buf.WriteByte(b)
					i += 3
				} else if c == '\\' && i < len(data)-1 {
					i++
					switch c = data[i]; c {
					case 'n':
						c = '\n'
					case 'r':
						c = '\r'
					case 't':
						c = '\t'
					case 'b':
						c = '\b'
					case 'a':
						c = '\a'
					}
					buf.WriteByte(c)
				} else if c == '"' {
					done = true
				} else {
					buf.WriteByte(c)
				}
			} else if inSingleQuote {
				if c == '\\' && i < len(data)-1 && data[i+1] == '\'' {
					i++
					buf.WriteByte('\'')
				} else if c == '\'' {
					done = true
				} else {
					buf.WriteByte(c)
				}
			} else {
				switch c {
				case ',':
					if inQuote {
						return nil, errors.New("invalid syntax")
					}
					done = true
				case '"':
					inQuote = true
				case '\'':
					inSingleQuote = true
				default:
					buf.WriteByte(c)
				}
				if i == len(data)-1 {
					done = true
				}
			}
		}
		if buf.Len() > 0 {
			ret = append(ret, buf.String())
		}
	}
}
