/*
 * Copyright 2024 The Go-Spring Authors.
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

package conf

import (
	"reflect"
	"time"

	"github.com/spf13/cast"
	"go-spring.org/spring/conf/decrypt"
	"go-spring.org/spring/conf/provider"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var converters = map[reflect.Type]any{}

func init() {
	RegisterConverter(func(s string) (time.Time, error) { return cast.ToTimeE(s) })
	RegisterConverter(func(s string) (time.Duration, error) { return time.ParseDuration(s) })
}

// Converter converts a string to a target type T.
type Converter[T any] func(string) (T, error)

// RegisterConverter registers a Converter for a type T, such as
// time.Time, time.Duration, or other user-defined types.
// Must be called in init functions only.
func RegisterConverter[T any](fn Converter[T]) {
	t := reflect.TypeFor[T]()
	if fn == nil {
		panic("converter for type " + t.String() + " cannot be nil")
	}
	if _, ok := converters[t]; ok {
		panic("converter for type " + t.String() + " already exists")
	}
	converters[t] = fn
}

// RegisterReader registers its Reader for some kind of file extension.
// For example, a YAML reader parses .yaml files into nested maps.
// Must be called in init functions only.
func RegisterReader(r reader.Reader, ext ...string) {
	reader.Register(r, ext...)
}

// RegisterProvider registers a Provider for a specific configuration source.
// For example, a file provider reads local files, an etcd provider fetches remote config.
// Must be called in init functions only.
func RegisterProvider(name string, p provider.Provider) {
	provider.Register(name, p)
}

// RegisterDecryptDriver registers a property-level decryptor driver, the seam
// through which a custom decryption scheme (an asymmetric cipher or a cloud
// KMS) replaces the built-in AES-GCM driver. Select the active driver with the
// GS_CONFIG_DECRYPT_DRIVER environment variable. Must be called in init
// functions only.
func RegisterDecryptDriver(name string, f decrypt.Factory) {
	decrypt.RegisterDriver(name, f)
}

// Load creates a Properties instance from a configuration source.
// The source format is [optional:]<provider>:<path> or just <path>.
// Returns an error if the file type is not supported or parsing fails.
func Load(source string) (*flatten.Properties, error) {
	data, err := provider.Load(source)
	if err != nil {
		return nil, err
	}
	return flatten.NewProperties(data), nil
}

// Bind maps property values from storage into the target object.
// The target must be a pointer to a struct or a reflect.Value.
// An optional tag specifies the root property key using ${key} syntax.
// Supports default values: ${key:=default}.
// If no tag is provided, uses ${ROOT} (binds from the root).
// Supports binding to structs, maps, slices, and primitive types.
func Bind(p flatten.Storage, i any, tag ...string) error {

	s := "${ROOT}"
	if len(tag) > 0 {
		s = tag[0]
	}
	if p == nil {
		err := errutil.Explain(nil, "p cannot be nil")
		return errutil.Explain(err, "conf: bind %q error", s)
	}

	var v reflect.Value
	{
		switch refVal := i.(type) {
		case reflect.Value:
			v = refVal
			if !v.IsValid() || !v.CanSet() {
				err := errutil.Explain(nil, "target should be a settable value")
				return errutil.Explain(err, "conf: bind %q error", s)
			}
		default:
			v = reflect.ValueOf(i)
			if !v.IsValid() || v.Kind() != reflect.Pointer {
				err := errutil.Explain(nil, "target should be a pointer to value type")
				return errutil.Explain(err, "conf: bind %q error", s)
			}
			if v.IsNil() {
				err := errutil.Explain(nil, "target should be a non-nil pointer to value type")
				return errutil.Explain(err, "conf: bind %q error", s)
			}
			v = v.Elem()
		}
	}

	t := v.Type()
	typeName := t.Name()
	if typeName == "" { // primitive types have no name
		typeName = t.String()
	}

	var param BindParam
	if err := param.BindTag(s, ""); err != nil {
		return errutil.Explain(err, "conf: bind %q error", s)
	}
	param.Path = typeName
	if err := BindValue(p, v, t, param, nil); err != nil {
		return errutil.Explain(err, "conf: bind %q error", s)
	}
	return nil
}

// Resolve expands property references of the form ${key}
// inside a string, recursively resolving nested expressions.
//
// Supported features:
// - References inside default values: e.g. "${key:=${fallback}}"
// - Default values: "${key:=fallback}"
// - Arbitrary string concatenation around references.
//
// Example:
//
//	Storage:
//	  "host" = "localhost"
//	  "port" = "8080"
//	Input:
//	  "http://${host}:${port}"
//	Output:
//	  "http://localhost:8080"
//
// Errors:
// - Returns invalid syntax if braces are unbalanced.
func Resolve(p flatten.Storage, s string) (string, error) {
	if p == nil {
		err := errutil.Explain(nil, "p cannot be nil")
		return "", errutil.Explain(err, "conf: resolve %q error", s)
	}
	v, err := resolveString(p, s)
	if err != nil {
		return "", errutil.Explain(err, "conf: resolve %q error", s)
	}
	return v, nil
}
