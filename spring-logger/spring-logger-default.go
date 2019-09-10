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

import "fmt"

type DefaultLogger struct {
}

func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *DefaultLogger) Debugln(args ...interface{}) {
	fmt.Println(args...)
}

func (l *DefaultLogger) Debug(args ...interface{}) {
	fmt.Print(args...)
}

func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *DefaultLogger) Infoln(args ...interface{}) {
	fmt.Println(args...)
}

func (l *DefaultLogger) Info(args ...interface{}) {
	fmt.Print(args...)
}

func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *DefaultLogger) Warnln(args ...interface{}) {
	fmt.Println(args...)
}

func (l *DefaultLogger) Warn(args ...interface{}) {
	fmt.Print(args...)
}

func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *DefaultLogger) Errorln(args ...interface{}) {
	fmt.Println(args...)
}

func (l *DefaultLogger) Error(args ...interface{}) {
	fmt.Print(args...)
}

func (l *DefaultLogger) Fatalf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *DefaultLogger) Fatalln(args ...interface{}) {
	fmt.Println(args...)
}

func (l *DefaultLogger) Fatal(args ...interface{}) {
	fmt.Print(args...)
}

func (l *DefaultLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
