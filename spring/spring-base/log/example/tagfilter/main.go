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

package main

import (
	"bytes"
	"fmt"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

var buffers = map[string]*bytes.Buffer{}

func init() {
	log.RegisterPlugin("Buffer", log.PluginTypeAppender, (*BufferAppender)(nil))
}

var _ log.Appender = (*BufferAppender)(nil)

type BufferAppender struct {
	log.BaseAppender
	BufferName string `PluginAttribute:"buffer"`
}

func (r *BufferAppender) Append(e *log.Event) {
	b, err := r.Layout.ToBytes(e)
	if err != nil {
		return
	}
	buf := getBuffer(r.BufferName)
	buf.Write(b)
}

func getBuffer(name string) *bytes.Buffer {
	v, ok := buffers[name]
	if !ok {
		v = bytes.NewBuffer(nil)
		buffers[name] = v
	}
	return v
}

func main() {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration>
			<Appenders>
				<Buffer name="buf_1" buffer="buf_1"/>
				<Buffer name="buf_2" buffer="buf_2"/>
				<Buffer name="buf_3" buffer="buf_3"/>
			</Appenders>
			<Loggers>
				<Root level="trace">
					<AppenderRef ref="buf_1">
						<TagFilter prefix="rpc_"/>
					</AppenderRef>
					<AppenderRef ref="buf_2">
						<TagFilter suffix="_rpc"/>
					</AppenderRef>
					<AppenderRef ref="buf_3">
						<TagFilter tag="request_in,request_out"/>
					</AppenderRef>
				</Root>
			</Loggers>
		</Configuration>
	`

	err := log.RefreshBuffer(config, ".xml")
	util.Panic(err).When(err != nil)

	logger := log.GetLogger("xxx")
	logger.WithTag("rpc_redis").Info("a=1")
	logger.WithTag("rpc_mysql").Info("b=1")
	logger.WithTag("request_in").Info("c=1")
	logger.WithTag("request_out").Info("d=1")
	logger.WithTag("redis_rpc").Info("e=1")
	logger.WithTag("mysql_rpc").Info("f=1")

	for _, buf := range buffers {
		fmt.Print(buf.String())
	}
}
