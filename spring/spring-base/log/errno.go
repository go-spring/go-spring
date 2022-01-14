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

package log

var (
	OK    = &errNo{project: 0, code: 0, message: "OK"}
	ERROR = &errNo{project: 999, code: 999, message: "ERROR"}
)

type ErrNo interface {
	Msg() string
	Code() uint32
}

type errNo struct {
	project uint32
	code    uint16
	message string
}

func NewErrNo(project uint32, code uint16, msg string) ErrNo {
	if project < 1000 {
		panic("project invalid, should be >= 1000")
	}
	if code < 1 || code > 999 {
		panic("code invalid, should be 1~999")
	}
	return &errNo{project: project, code: code, message: msg}
}

func (e *errNo) Msg() string  { return e.message }
func (e *errNo) Code() uint32 { return e.project*1000 + uint32(e.code) }
