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

//
// 标准的 Logger 接口
//
type StdLogger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

//
// 带前缀名的 Logger 接口
//
type PrefixLogger interface {
	LogDebug(args ...interface{})
	LogDebugf(format string, args ...interface{})

	LogInfo(args ...interface{})
	LogInfof(format string, args ...interface{})

	LogWarn(args ...interface{})
	LogWarnf(format string, args ...interface{})

	LogError(args ...interface{})
	LogErrorf(format string, args ...interface{})

	LogFatal(args ...interface{})
	LogFatalf(format string, args ...interface{})
}
