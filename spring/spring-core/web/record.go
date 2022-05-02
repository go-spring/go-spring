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
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/net/recorder"
	"github.com/google/uuid"
)

// StartRecord 启动流量录制
func StartRecord(ctx Context) {
	r, err := uuid.NewRandom()
	if err != nil {
		logger.WithContext(ctx.Context()).Error(log.ERROR, err)
		return
	}
	recorder.StartRecord(ctx.Context(), r.String())
}

// StopRecord 停止流量录制
func StopRecord(ctx Context) {
	req := ctx.Request()
	resp := ctx.ResponseWriter()
	recorder.RecordInbound(req.Context(), recorder.HTTP, &recorder.SimpleAction{
		Request: func() string {
			var buf bytes.Buffer
			err := req.Write(&buf)
			if err != nil {
				return err.Error()
			}
			return buf.String()
		},
		Response: func() string {
			var buf bytes.Buffer
			is11 := req.ProtoAtLeast(1, 1)
			if is11 {
				buf.WriteString("HTTP/1.1 ")
			} else {
				buf.WriteString("HTTP/1.0 ")
			}
			buf.Write(strconv.AppendInt([]byte{}, int64(resp.Status()), 10))
			buf.WriteByte(' ')
			buf.WriteString(http.StatusText(resp.Status()))
			buf.WriteString("\r\n")
			err := resp.Header().WriteSubset(&buf, nil)
			if err != nil {
				return err.Error()
			}
			if resp.Header().Get("Content-Length") == "" {
				buf.WriteString("Content-Length: ")
				buf.WriteString(cast.ToString(resp.Size()))
				buf.WriteString("\r\n")
			}
			buf.WriteString("\r\n")
			buf.WriteString(resp.Body())
			return buf.String()
		},
	})
}
