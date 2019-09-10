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

package SpringLogger

import "os"

type LoggerAppender interface {
	Write(p []byte) (n int, err error)
}

//
// 输出到控制台
//
type ConsoleAppender struct {
}

func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{}
}

func (appender *ConsoleAppender) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

//
// 输出到文件
//
type FileAppender struct {
	File *os.File
}

func NewFileAppender(file *os.File) *FileAppender {
	return &FileAppender{
		File: file,
	}
}

func (appender *FileAppender) Write(p []byte) (n int, err error) {
	return appender.File.Write(p)
}
