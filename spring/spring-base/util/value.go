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

package util

import (
	"reflect"
	"runtime"
	"strings"
	"unsafe"
)

const (
	flagStickyRO = 1 << 5
	flagEmbedRO  = 1 << 6
	flagRO       = flagStickyRO | flagEmbedRO
)

// PatchValue makes an unexported field can be assignable.
func PatchValue(v reflect.Value) reflect.Value {
	rv := reflect.ValueOf(&v)
	flag := rv.Elem().FieldByName("flag")
	ptrFlag := (*uintptr)(unsafe.Pointer(flag.UnsafeAddr()))
	*ptrFlag = *ptrFlag &^ flagRO
	return v
}

// Indirect returns its element type when t is a pointer type.
func Indirect(t reflect.Type) reflect.Type {
	if t.Kind() != reflect.Ptr {
		return t
	}
	return t.Elem()
}

// FileLine returns a function's name, file name and line number.
func FileLine(fn interface{}) (file string, line int, fnName string) {

	fnPtr := reflect.ValueOf(fn).Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line = fnInfo.FileLine(fnPtr)

	s := fnInfo.Name()
	if ss := strings.Split(s, "/"); len(ss) > 0 {
		s = ss[len(ss)-1]
		i := strings.Index(s, ".")
		s = s[i+1:]
	}

	// method values are printed as "T.m-fm"
	s = strings.TrimRight(s, "-fm")
	return file, line, s
}
