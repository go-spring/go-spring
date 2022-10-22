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

// Package binding ...
package binding

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"

	"github.com/go-spring/spring-core/validate"
)

const (
	MIMEApplicationJSON     = "application/json"
	MIMEApplicationXML      = "application/xml"
	MIMETextXML             = "text/xml"
	MIMEApplicationForm     = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf = "application/protobuf"
	MIMEApplicationMsgpack  = "application/msgpack"
	MIMEMultipartForm       = "multipart/form-data"
)

type Request interface {
	ContentType() string
	Header(key string) string
	PathParam(name string) string
	QueryParam(name string) string
	FormParams() (url.Values, error)
	RequestBody() ([]byte, error)
}

type BindScope int

const (
	BindScopeURI BindScope = iota
	BindScopeQuery
	BindScopeHeader
	BindScopeBody
)

var scopeBinders = map[BindScope][]string{
	BindScopeURI:    {"uri", "path"},
	BindScopeQuery:  {"query", "param"},
	BindScopeHeader: {"header"},
}

var scopeGetters = map[BindScope]func(r Request, name string) string{
	BindScopeURI:    Request.PathParam,
	BindScopeQuery:  Request.QueryParam,
	BindScopeHeader: Request.Header,
}

type BodyBinder func(i interface{}, r Request) error

var bodyBinders = map[string]BodyBinder{
	MIMEApplicationForm: BindForm,
	MIMEMultipartForm:   BindForm,
	MIMEApplicationJSON: BindJSON,
	MIMEApplicationXML:  BindXML,
	MIMETextXML:         BindXML,
}

func RegisterScopeBinder(scope BindScope, tag string) {
	tags := scopeBinders[scope]
	for _, s := range tags {
		if s == tag {
			return
		}
	}
	tags = append(tags, tag)
	scopeBinders[scope] = tags
}

func RegisterBodyBinder(mime string, binder BodyBinder) {
	bodyBinders[mime] = binder
}

func Bind(i interface{}, r Request) error {
	if err := bindScope(i, r); err != nil {
		return err
	}
	if err := bindBody(i, r); err != nil {
		return err
	}
	return validate.Struct(i)
}

func bindBody(i interface{}, r Request) error {
	binder, ok := bodyBinders[r.ContentType()]
	if !ok {
		binder = bodyBinders[MIMEApplicationForm]
	}
	return binder(i, r)
}

func bindScope(i interface{}, r Request) error {
	t := reflect.TypeOf(i)
	if t.Kind() != reflect.Ptr {
		return nil
	}
	et := t.Elem()
	if et.Kind() != reflect.Struct {
		return nil
	}
	ev := reflect.ValueOf(i).Elem()
	for j := 0; j < ev.NumField(); j++ {
		fv := ev.Field(j)
		ft := et.Field(j)
		for scope := BindScopeURI; scope < BindScopeBody; scope++ {
			err := bindScopeField(scope, fv, ft, r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func bindScopeField(scope BindScope, v reflect.Value, field reflect.StructField, r Request) error {
	for _, tag := range scopeBinders[scope] {
		if name, ok := field.Tag.Lookup(tag); ok {
			if name == "-" {
				continue
			}
			val := scopeGetters[scope](r, name)
			err := bindData(v, val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func bindData(v reflect.Value, val string) error {
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 0, 0)
		if err != nil {
			return err
		}
		v.SetUint(u)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 0, 0)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		v.SetBool(b)
		return nil
	case reflect.String:
		v.SetString(val)
		return nil
	default:
		return fmt.Errorf("unsupported binding type %q", v.Type().String())
	}
}
