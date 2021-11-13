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
	"bytes"
	"net/http"
	"strconv"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

// StartRecord 启动流量录制
func StartRecord(ctx Context) {
	if !fastdev.RecordMode() {
		return
	}
	session := fastdev.NewSessionID()
	err := knife.Set(ctx.Context(), fastdev.RecordSessionIDKey, session)
	util.Panic(err).When(err != nil)
}

// StopRecord 停止流量录制
func StopRecord(ctx Context) {

	if !fastdev.RecordMode() {
		return
	}

	req := ctx.Request()
	resp := ctx.ResponseWriter()

	var bufReq bytes.Buffer
	err := req.Write(&bufReq)
	if err != nil {
		log.Ctx(ctx.Context()).Error(err)
		return
	}

	var bufResp bytes.Buffer

	is11 := req.ProtoAtLeast(1, 1)
	writeStatusLine(&bufResp, is11, resp.Status())
	err = resp.Header().WriteSubset(&bufResp, nil)
	if err != nil {
		log.Ctx(ctx.Context()).Error(err)
		return
	}

	if resp.Header().Get("Content-Length") == "" {
		bufResp.WriteString("Content-Length: ")
		bufResp.WriteString(cast.ToString(resp.Size()))
		bufResp.WriteString("\r\n")
	}

	bufResp.WriteString("\r\n")
	bufResp.WriteString(resp.Body())

	fastdev.RecordInbound(ctx.Request().Context(), &fastdev.Action{
		Protocol: fastdev.HTTP,
		Request:  bufReq.String(),
		Response: bufResp.String(),
	})
}

func writeStatusLine(buf *bytes.Buffer, is11 bool, code int) {
	if is11 {
		buf.WriteString("HTTP/1.1 ")
	} else {
		buf.WriteString("HTTP/1.0 ")
	}
	text := http.StatusText(code)
	buf.Write(strconv.AppendInt([]byte{}, int64(code), 10))
	buf.WriteByte(' ')
	buf.WriteString(text)
	buf.WriteString("\r\n")
}
