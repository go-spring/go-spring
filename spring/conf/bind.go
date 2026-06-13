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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/typeutil"
)

// ParsedTag represents a parsed configuration tag that encodes
// metadata for binding configuration values from property sources.
//
// A tag string generally follows the pattern:
//
//	${key:=default}
//
// - "key":        the property key used to look up a value.
// - "default":    optional fallback value if the key does not exist.
//
// Examples:
//
//	"${db.host:=localhost}"       -> key=db.host, default=localhost
//	"${ports:=8080,9090}"         -> key=ports, default=8080,9090
//	"${:=foo}"                    -> empty key, only default value "foo"
//
// The parsing logic is strict; malformed tags will result in invalid syntax.
type ParsedTag struct {
	Key    string // short property key
	Def    string // default value string
	HasDef bool   // indicates whether a default value exists
}

func (tag ParsedTag) String() string {
	var sb strings.Builder
	sb.WriteString("${")
	sb.WriteString(tag.Key)
	if tag.HasDef {
		sb.WriteString(":=")
		sb.WriteString(tag.Def)
	}
	sb.WriteString("}")
	return sb.String()
}

// ParseTag parses a tag string into a ParsedTag struct.
// It intentionally only checks the outer ${...} wrapper. Earlier validation
// ensures the tag syntax is well-formed before parsing reaches this point.
func ParseTag(tag string) (ret ParsedTag, err error) {
	if !strings.HasSuffix(tag, "}") {
		err = errutil.Explain(nil, "invalid syntax tag '%s': missing closing brace", tag)
		return
	}
	if !strings.HasPrefix(tag, "${") {
		err = errutil.Explain(nil, "invalid syntax tag '%s': missing opening '${'", tag)
		return
	}
	ss := strings.SplitN(tag[2:len(tag)-1], ":=", 2)
	ret = ParsedTag{Key: strings.TrimSpace(ss[0])}
	if len(ss) > 1 {
		ret.HasDef = true
		ret.Def = strings.TrimSpace(ss[1])
	}
	return
}

// BindParam holds metadata needed to bind a single configuration value
// to a Go struct field, slice element, or map entry.
type BindParam struct {
	Key      string            // full property key
	Path     string            // full property path
	Tag      ParsedTag         // parsed tag
	Validate reflect.StructTag // original struct field tag for validation
}

// BindTag parses the tag string, stores the ParsedTag in BindParam,
// and resolves nested key expansion.
//
// Special cases:
// - "${:=default}" -> Key is empty, only default is set.
// - "${ROOT}"      -> explicitly resets Key to an empty string.
//
// If a BindParam already has a Key, new keys are appended hierarchically,
// e.g. parent Key="db", tag="${host}" -> final Key="db.host".
//
// Errors:
// - Returns invalid syntax if the tag string is malformed or empty without default.
func (param *BindParam) BindTag(tag string, validate reflect.StructTag) error {
	param.Validate = validate
	parsedTag, err := ParseTag(tag)
	if err != nil {
		return err
	}
	if parsedTag.Key == "" { // ${:=} default value syntax
		if parsedTag.HasDef {
			param.Tag = parsedTag
			return nil
		}
		return errutil.Explain(nil, "invalid syntax tag '%s': empty key without default value", tag)
	}
	if parsedTag.Key == "ROOT" {
		parsedTag.Key = ""
	}
	if param.Key == "" {
		param.Key = parsedTag.Key
	} else if parsedTag.Key != "" {
		param.Key = param.Key + "." + parsedTag.Key
	}
	param.Tag = parsedTag
	return nil
}

// Filter defines an interface for filtering configuration fields during binding.
// Dynamic configuration fields have the same syntax, they need to be filtered out.
type Filter interface {
	Do(i any, param BindParam) (bool, error)
}

// BindValue attempts to bind a property value from the property source `p`
// into the given reflect.Value `v`, based on metadata in `param`.
//
// Supported binding targets:
//   - Primitive types (string, int, float, bool, etc.).
//   - Structs (recursively bound field by field).
//   - Maps (bound by iterating subkeys).
//   - Slices (bound by either indexed keys or split strings).
//
// Errors:
//   - Validation failure if "expr" tag evaluation fails.
//   - Type mismatch (e.g., array instead of slice).
//   - Property not exist without default value.
//   - Type conversion errors during parsing.
//   - Custom converter function errors.
func BindValue(p flatten.Storage, v reflect.Value, t reflect.Type, param BindParam, filter Filter) (RetErr error) {

	if isNilStorage(p) {
		return errutil.Explain(nilStorageError(), "failed to bind at path %s", param.Path)
	}

	if !typeutil.IsPropBindingTarget(t) {
		err := errutil.Explain(nil, "target should be a value type")
		return errutil.Explain(err, "failed to bind at path %s", param.Path)
	}

	// run validation if "expr" tag is defined and no prior error
	defer func() {
		if RetErr == nil {
			tag, ok := param.Validate.Lookup("expr")
			if ok && len(tag) > 0 {
				if RetErr = validateField(tag, v.Interface()); RetErr != nil {
					RetErr = errutil.Explain(RetErr, "validation failed at path %s with expr %q", param.Path, tag)
				}
			}
		}
	}()

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, t, param, filter)
	case reflect.Slice:
		return bindSlice(p, v, t, param, filter)
	case reflect.Array:
		err := errutil.Explain(nil, "use slice instead of array")
		return errutil.Explain(err, "failed to bind at path %s", param.Path)
	default: // for linter
	}

	fn := converters[t]
	if fn == nil && v.Kind() == reflect.Struct {
		if err := bindStruct(p, v, t, param, filter); err != nil {
			return err
		}
		return nil
	}

	// resolve property value (with default and references)
	val, err := resolve(p, param)
	if err != nil {
		return errutil.Explain(err, "failed to resolve value at path %s", param.Path)
	}
	if val = strings.TrimSpace(val); val == "" {
		return nil
	}

	// try converter function first
	if fn != nil {
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(val)})
		if !out[1].IsNil() {
			err = out[1].Interface().(error)
			return errutil.Explain(err, "failed to convert value at path %s", param.Path)
		}
		v.Set(out[0])
		return nil
	}

	// fallback: parse string into basic types
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var u uint64
		if u, err = strconv.ParseUint(val, 0, 0); err == nil {
			v.SetUint(u)
			return nil
		}
		return errutil.Explain(err, "failed to parse uint at path %s", param.Path)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int64
		if i, err = strconv.ParseInt(val, 0, 0); err == nil {
			v.SetInt(i)
			return nil
		}
		return errutil.Explain(err, "failed to parse int at path %s", param.Path)
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(val, 64); err == nil {
			v.SetFloat(f)
			return nil
		}
		return errutil.Explain(err, "failed to parse float at path %s", param.Path)
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(val); err == nil {
			v.SetBool(b)
			return nil
		}
		return errutil.Explain(err, "failed to parse bool at path %s", param.Path)
	default:
		// treat everything else as string
		v.SetString(val)
		return nil
	}
}

// bindSlice binds configuration values into a slice of type []T.
//
// Supported input formats:
//  1. Indexed keys in the property source:
//     e.g. "list[0]=a", "list[1]=b"
//  2. A single delimited string:
//     e.g. "list=a,b,c"  (split by ",")
//
// The slice is always reset (v.Set(slice)) before return,
// even if binding fails midway.
func bindSlice(p flatten.Storage, v reflect.Value, t reflect.Type, param BindParam, filter Filter) error {

	elemType := t.Elem()
	p, err := getSlice(p, param)
	if err != nil {
		return errutil.Explain(err, "failed to bind slice at path %s", param.Path)
	}

	slice := reflect.MakeSlice(t, 0, 0)
	if p == nil {
		v.Set(slice)
		return nil
	}

	for i := 0; ; i++ {
		subValue := reflect.New(elemType).Elem()
		subParam := BindParam{
			Key:  fmt.Sprintf("%s[%d]", param.Key, i),
			Path: fmt.Sprintf("%s[%d]", param.Path, i),
		}
		if !p.Exists(subParam.Key) {
			break // stop when no more indexed elements
		}
		err = BindValue(p, subValue, elemType, subParam, filter)
		if err != nil {
			return err
		}
		slice = reflect.Append(slice, subValue)
	}
	v.Set(slice)
	return nil
}

// getSlice prepares a Storage object representing slice elements
// derived from either:
//
// - Explicit indexed properties (preferred).
// - A single delimited string property, split into multiple elements.
//
// Errors:
// - Returns not exist if property is missing and no default is provided.
func getSlice(p flatten.Storage, param BindParam) (flatten.Storage, error) {

	m := make(map[string]string)
	if p.SliceEntries(param.Key, m) {
		if err := validateSliceIndexes(param.Key, m); err != nil {
			return nil, err
		}
		for k, v := range m {
			s, err := resolveString(p, v)
			if err != nil {
				return nil, err
			}
			m[k] = s
		}
		return flatten.NewPropertiesStorage(flatten.NewProperties(m)), nil
	}

	// case 2: property is a single string -> split into slice
	strVal, ok := p.Value(param.Key)
	if !ok {
		if !param.Tag.HasDef {
			return nil, errutil.Explain(nil, "property %q does not exist", param.Key)
		}
		strVal = param.Tag.Def
	}
	s, err := resolveString(p, strVal)
	if err != nil {
		return nil, err
	}
	if strVal = strings.TrimSpace(s); strVal == "" {
		return nil, nil
	}

	arrVal := strings.Split(strVal, ",")
	for i := range arrVal {
		arrVal[i] = strings.TrimSpace(arrVal[i])
	}

	m = make(map[string]string)
	for i, s := range arrVal {
		k := fmt.Sprintf("%s[%d]", param.Key, i)
		m[k] = s
	}
	return flatten.NewPropertiesStorage(flatten.NewProperties(m)), nil
}

func validateSliceIndexes(key string, entries map[string]string) error {
	indexes := make(map[int]struct{})
	maxIndex := -1

	for entry := range entries {
		index, err := parseSliceEntryIndex(key, entry)
		if err != nil {
			return err
		}
		indexes[index] = struct{}{}
		if index > maxIndex {
			maxIndex = index
		}
	}

	for i := 0; i <= maxIndex; i++ {
		if _, ok := indexes[i]; !ok {
			return errutil.Explain(nil, "missing slice index %d for property %q", i, key)
		}
	}
	return nil
}

func parseSliceEntryIndex(key string, entry string) (int, error) {
	suffix, ok := strings.CutPrefix(entry, key)
	if !ok || suffix == "" || suffix[0] != '[' {
		return 0, errutil.Explain(nil, "invalid slice entry %q for property %q", entry, key)
	}

	end := strings.IndexByte(suffix, ']')
	if end < 0 {
		return 0, errutil.Explain(nil, "invalid slice entry %q for property %q", entry, key)
	}

	index, err := strconv.Atoi(suffix[1:end])
	if err != nil || index < 0 {
		return 0, errutil.Explain(nil, "invalid slice index %q for property %q", suffix[1:end], key)
	}
	return index, nil
}

// bindMap binds configuration properties into a Go map[K]V.
//
// Example:
//
//	Storage:
//	  "users.alice.age" = 20
//	  "users.bob.age"   = 30
//
//	Binding into map[string]User produces:
//	  {"alice": User{Age:20}, "bob": User{Age:30}}
//
// Errors:
// - Returns error if property is missing without default.
// - Propagates binding errors from element binding.
func bindMap(p flatten.Storage, v reflect.Value, t reflect.Type, param BindParam, filter Filter) error {

	if t.Key().Kind() != reflect.String {
		err := errutil.Explain(nil, "map key should be string")
		return errutil.Explain(err, "failed to bind map at path %s", param.Path)
	}

	if param.Tag.HasDef && param.Tag.Def != "" {
		err := errutil.Explain(nil, "map can't have a non-empty default value")
		return errutil.Explain(err, "failed to bind map at path %s", param.Path)
	}

	elemType := t.Elem()
	ret := reflect.MakeMap(t)

	// handle empty key as default value placeholder
	if param.Tag.Key == "" {
		if param.Tag.HasDef {
			v.Set(ret)
			return nil
		}
	}

	// Allow `param.Key` to be an empty string,
	// to retrieve all configuration items.
	keySet := make(map[string]struct{})
	p.MapKeys(param.Key, keySet)
	if len(keySet) == 0 {
		if param.Tag.HasDef {
			v.Set(ret)
			return nil
		}
		err := errutil.Explain(nil, "map property %q does not exist", param.Key)
		return errutil.Explain(err, "failed to bind map at path %s", param.Path)
	}

	for key := range keySet {
		subValue := reflect.New(elemType).Elem()
		subKey := key
		if param.Key != "" {
			subKey = param.Key + "." + key
		}
		subParam := BindParam{
			Key:  subKey,
			Path: param.Path,
		}
		if err := BindValue(p, subValue, elemType, subParam, filter); err != nil {
			return err
		}
		ret.SetMapIndex(reflect.ValueOf(key), subValue)
	}
	v.Set(ret)
	return nil
}

// bindStruct binds configuration properties into a struct.
//
// Example:
//
//	type Config struct {
//	    Host string `value:"${db.host:=localhost}"`
//	    Port int    `value:"${db.port:=3306}"`
//	}
//
//	With properties:
//	  db.host=127.0.0.1
//	Result:
//	  Config{Host:"127.0.0.1", Port:3306}
//
// Errors:
// - Invalid syntax in tag.
// - Binding or conversion failures in nested fields.
// - Infinite recursion is avoided for embedded pointer structs.
func bindStruct(p flatten.Storage, v reflect.Value, t reflect.Type, param BindParam, filter Filter) error {

	if param.Tag.HasDef && param.Tag.Def != "" {
		err := errutil.Explain(nil, "struct can't have a non-empty default value")
		return errutil.Explain(err, "failed to bind struct at path %s", param.Path)
	}

	for i := range t.NumField() {
		ft := t.Field(i)
		fv := v.Field(i)

		// skip unexported fields
		if !fv.CanInterface() {
			continue
		}

		subParam := BindParam{
			Key:  param.Key,
			Path: param.Path + "." + ft.Name,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			if err := subParam.BindTag(tag, ft.Tag); err != nil {
				return errutil.Explain(err, "failed to bind field %s.%s at path %s", t.Name(), ft.Name, param.Path)
			}
			if filter != nil {
				if ret, err := filter.Do(fv.Addr().Interface(), subParam); err != nil {
					return err
				} else if ret {
					continue
				}
			}
			if err := BindValue(p, fv, ft.Type, subParam, filter); err != nil {
				return err
			}
			continue
		}

		if ft.Anonymous {
			// embed pointer type may lead to infinite recursion.
			if ft.Type.Kind() != reflect.Struct {
				continue
			}
			if err := bindStruct(p, fv, ft.Type, subParam, filter); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolve fetches the final string value of a property key,
// applying default values and resolving references recursively.
//
// Example:
//
//	Storage:
//	  "host" = "localhost"
//	  "url"  = "http://${host}:8080"
//
//	resolve(url) -> "http://localhost:8080"
func resolve(p flatten.Storage, param BindParam) (string, error) {
	return resolveWithStack(p, param, make(map[string]struct{}))
}

func resolveWithStack(p flatten.Storage, param BindParam, stack map[string]struct{}) (string, error) {
	if val, ok := p.Value(param.Key); ok {
		if param.Key != "" {
			if _, ok := stack[param.Key]; ok {
				return "", errutil.Explain(nil, "circular property reference %q", param.Key)
			}
			stack[param.Key] = struct{}{}
			defer delete(stack, param.Key)
		}
		return resolveStringWithStack(p, val, stack)
	}
	if p.Exists(param.Key) {
		return "", errutil.Explain(nil, "property %q is not a simple value", param.Key)
	}
	if param.Tag.HasDef {
		return resolveStringWithStack(p, param.Tag.Def, stack)
	}
	return "", errutil.Explain(nil, "property %q does not exist", param.Key)
}

// resolveString resolves a single string value,
// applying default values and resolving references recursively.
func resolveString(p flatten.Storage, s string) (string, error) {
	return resolveStringWithStack(p, s, make(map[string]struct{}))
}

func resolveStringWithStack(p flatten.Storage, s string, stack map[string]struct{}) (string, error) {

	// If there is no property reference, return the original string.
	start := strings.Index(s, "${")
	if start < 0 {
		return s, nil
	}

	var (
		level = 1
		end   = -1
	)

	// scan for matching closing brace, handling nested references
	for i := start + 2; i < len(s); i++ {
		if s[i] == '$' {
			if i+1 < len(s) && s[i+1] == '{' {
				level++
			}
		} else if s[i] == '}' {
			level--
			if level == 0 {
				end = i
				break
			}
		}
	}

	if end < 0 {
		return "", errutil.Explain(nil, "invalid syntax: unmatched braces in '%s'", s)
	}

	var param BindParam
	err := param.BindTag(s[start:end+1], "")
	if err != nil {
		return "", err
	}

	// resolve the referenced property
	resolved, err := resolveWithStack(p, param, stack)
	if err != nil {
		return "", err
	}

	// resolve the remaining part of the string
	suffix, err := resolveStringWithStack(p, s[end+1:], stack)
	if err != nil {
		return "", err
	}

	// combine: prefix + resolved + suffix
	return s[:start] + resolved + suffix, nil
}
