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
// 日志接口
//
type LoggerInterface interface {
	Debugf(format string, args ...interface{})
	Debugln(args ...interface{})
	Debug(args ...interface{})

	Infof(format string, args ...interface{})
	Infoln(args ...interface{})
	Info(args ...interface{})

	Warnf(format string, args ...interface{})
	Warnln(args ...interface{})
	Warn(args ...interface{})

	Errorf(format string, args ...interface{})
	Errorln(args ...interface{})
	Error(args ...interface{})

	Fatalf(format string, args ...interface{})
	Fatalln(args ...interface{})
	Fatal(args ...interface{})

	Printf(format string, args ...interface{})
}

var (
	Logger LoggerInterface = new(DefaultLogger)
)

//
// 注册日志接口
//
func SetLogger(l LoggerInterface) {
	Logger = l
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

func Debugln(args ...interface{}) {
	Logger.Debugln(args...)
}

func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

func Infoln(args ...interface{}) {
	Logger.Infoln(args...)
}

func Info(args ...interface{}) {
	Logger.Info(args...)
}

func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

func Warnln(args ...interface{}) {
	Logger.Warnln(args...)
}

func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

func Errorln(args ...interface{}) {
	Logger.Errorln(args...)
}

func Error(args ...interface{}) {
	Logger.Error(args...)
}

func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}

func Fatalln(args ...interface{}) {
	Logger.Fatalln(args...)
}

func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}
