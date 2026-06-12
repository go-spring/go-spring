/*
 * Copyright 2025 The Go-Spring Authors.
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

package log

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
)

var (
	typeConverters = map[reflect.Type]any{}
	pluginRegistry = map[string]*Plugin{}
)

// Converter defines a function that converts a string to type T.
type Converter[T any] func(string) (T, error)

// RegisterConverter registers a custom converter for type T.
func RegisterConverter[T any](fn Converter[T]) {
	t := reflect.TypeFor[T]()
	typeConverters[t] = fn
}

func init() {
	RegisterConverter(time.ParseDuration)
}

// Lifecycle is an optional interface for plugin lifecycle hooks.
type Lifecycle interface {
	Start() error
	Stop()
}

// Plugin represents metadata about a plugin type.
type Plugin struct {
	Name  string       // Name of the plugin
	Class reflect.Type // Underlying struct type
	File  string       // File where plugin was registered
	Line  int          // Line number where plugin was registered
}

// RegisterPlugin registers a plugin struct type with a given name and plugin type.
func RegisterPlugin[T any](name string) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		panic("T must be struct")
	}
	_, file, line, _ := runtime.Caller(1)
	if p, ok := pluginRegistry[name]; ok {
		err := errutil.Explain(nil, "duplicate plugin name %q in %s:%d and %s:%d",
			name, p.File, p.Line, file, line)
		panic(err)
	}
	pluginRegistry[name] = &Plugin{
		Name:  name,
		Class: t,
		File:  file,
		Line:  line,
	}
}

// newPlugin creates a new plugin instance and injects configuration values.
func newPlugin(t reflect.Type, prefix string, s flatten.Storage) (reflect.Value, error) {
	v := reflect.New(t)
	if err := inject(v.Elem(), t, prefix, s); err != nil {
		return reflect.Value{}, err
	}
	return v, nil
}

// PluginTag is a wrapper for parsing struct field tags.
type PluginTag string

// Get returns the value for a key or the first unnamed value.
func (tag PluginTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value of a key in a tag and a boolean indicating existence.
func (tag PluginTag) Lookup(key string) (value string, ok bool) {
	kvs := strings.Split(string(tag), ",")
	if key == "" {
		return kvs[0], true
	}
	for i := 1; i < len(kvs); i++ {
		ss := strings.SplitN(kvs[i], "=", 2)
		if len(ss) != 2 {
			return "", false
		} else if ss[0] == key {
			return ss[1], true
		}
	}
	return "", false
}

// inject recursively sets struct fields based on `PluginAttribute` and `PluginElement` tags.
func inject(v reflect.Value, t reflect.Type, prefix string, s flatten.Storage) error {
	for i := range v.NumField() {
		ft := t.Field(i)
		fv := v.Field(i)

		// Skip unexported fields
		if !fv.CanInterface() {
			continue
		}

		// Inject from `PluginAttribute` tag
		if tag, ok := ft.Tag.Lookup("PluginAttribute"); ok {
			if err := injectAttribute(fv, ft, prefix, tag, s); err != nil {
				return errutil.Stack(err, "inject field %s.%s error", t.Name(), ft.Name)
			}
			continue
		}

		// Inject from `PluginElement` tag
		if tag, ok := ft.Tag.Lookup("PluginElement"); ok {
			if err := injectElement(fv, ft, prefix, tag, s); err != nil {
				return errutil.Stack(err, "inject field %s.%s error", t.Name(), ft.Name)
			}
			continue
		}

		// Recursively inject anonymous embedded structs
		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := inject(fv, fv.Type(), prefix, s); err != nil {
				return err
			}
		}
	}
	return nil
}

// injectAttribute injects an attribute into a struct field.
func injectAttribute(fv reflect.Value, ft reflect.StructField, prefix string, tag string, s flatten.Storage) error {

	attrTag := PluginTag(tag)
	attrName := attrTag.Get("")
	if attrName == "" {
		return errutil.Explain(nil, "PluginAttribute tag is empty for field at %s", prefix)
	}

	// Special handling for "name" attribute
	if attrName == "name" {
		name := prefix[strings.LastIndex(prefix, ".")+1:]
		fv.SetString(name)
		return nil
	}

	elemKey := prefix + "." + attrName

	switch fv.Kind() {
	case reflect.Slice:
		return injectArrayAttribute(fv, ft, elemKey, attrTag, s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String, reflect.Struct:
		return injectSingleAttribute(fv, ft, elemKey, attrTag, s)
	default:
		return errutil.Explain(nil, "unsupported inject type %s for field at %s", ft.Type.String(), prefix)
	}
}

// injectSingleAttribute injects a single attribute into a struct field.
func injectSingleAttribute(fv reflect.Value, ft reflect.StructField, prefix string, tag PluginTag, s flatten.Storage) error {
	val, ok := s.Value(prefix)
	if !ok {
		val, ok = tag.Lookup("default")
		if !ok {
			return errutil.Explain(nil, "no value configured and no default specified")
		}
	}
	val, err := resolveProperty(s, val)
	if err != nil {
		return err
	}
	v, err := convertAttributeValue(ft.Type, val)
	if err != nil {
		return err
	}
	fv.Set(v)
	return nil
}

// injectArrayAttribute injects an array attribute into a struct field.
func injectArrayAttribute(fv reflect.Value, ft reflect.StructField, prefix string, tag PluginTag, s flatten.Storage) error {
	values, err := getArrayValues(s, prefix, tag)
	if err != nil {
		return err
	}
	elemType := ft.Type.Elem()
	slice := reflect.MakeSlice(ft.Type, len(values), len(values))
	for i, str := range values {
		elemVal, err := convertAttributeValue(elemType, str)
		if err != nil {
			return errutil.Stack(err, "inject %s[%d] error", ft.Name, i)
		}
		slice.Index(i).Set(elemVal)
	}
	fv.Set(slice)
	return nil
}

// getArrayValues retrieves an array of values from storage.
func getArrayValues(s flatten.Storage, prefix string, tag PluginTag) ([]string, error) {

	m := make(map[string]string)
	if s.SliceEntries(prefix, m) {
		if err := validateArrayValueIndexes(prefix, m); err != nil {
			return nil, err
		}
	}

	var values []string
	for i := 0; ; i++ {
		// 简单数组一定是叶子结点，不支持结构体嵌套
		subKey := fmt.Sprintf("%s[%d]", prefix, i)
		strVal, ok := s.Value(subKey)
		if !ok {
			break
		}
		strVal, err := resolveProperty(s, strVal)
		if err != nil {
			return nil, errutil.Stack(err, "resolve property reference error for field at %s", subKey)
		}
		values = append(values, strVal)
	}
	if len(values) > 0 {
		return values, nil
	}

	// Fallback to single string value
	strVal, ok := s.Value(prefix)
	if !ok {
		strVal, ok = tag.Lookup("default")
		if !ok {
			return nil, errutil.Explain(nil, "no array values found and no default specified for field at %s", prefix)
		}
	}
	strVal, err := resolveProperty(s, strVal)
	if err != nil {
		return nil, errutil.Stack(err, "resolve property reference error for field at %s", prefix)
	}
	for str := range strings.SplitSeq(strVal, ",") {
		values = append(values, strings.TrimSpace(str))
	}
	return values, nil
}

func validateArrayValueIndexes(prefix string, values map[string]string) error {
	return validateSliceIndexes(prefix, values, "array value")
}

// convertAttributeValue converts a string value to the specified type.
func convertAttributeValue(t reflect.Type, val string) (reflect.Value, error) {

	// Try to use a registered type converter
	if fn := typeConverters[t]; fn != nil {
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(val)})
		if !out[1].IsNil() {
			err := out[1].Interface().(error)
			return reflect.Value{}, err
		}
		return out[0], nil
	}

	v := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 0, t.Bits())
		if err != nil {
			return reflect.Value{}, errutil.Explain(err, "parse %q to %s error", val, t.String())
		}
		v.SetUint(u)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 0, t.Bits())
		if err != nil {
			return reflect.Value{}, errutil.Explain(err, "parse %q to %s error", val, t.String())
		}
		v.SetInt(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, t.Bits())
		if err != nil {
			return reflect.Value{}, errutil.Explain(err, "parse %q to %s error", val, t.String())
		}
		v.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return reflect.Value{}, errutil.Explain(err, "parse %q to %s error", val, t.String())
		}
		v.SetBool(b)
	case reflect.String:
		v.SetString(val)
	default:
		return reflect.Value{}, errutil.Explain(nil, "unsupported inject type %s", t.String())
	}
	return v, nil
}

// resolveProperty resolves a property reference in a string value.
func resolveProperty(p flatten.Storage, s string) (string, error) {
	// If there is no property reference, return the original string.
	start := strings.Index(s, "${")
	if start < 0 {
		return s, nil
	}

	var (
		level = 1 // nesting level for ${...}
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
		return "", errutil.Explain(nil, "malformed property reference syntax %q", s)
	}

	key := s[start+2 : end]
	val, ok := p.Value(key)
	if !ok {
		if p.Exists(key) {
			return "", errutil.Explain(nil, "property reference %q is not a simple value", s[start:end+1])
		}
		return "", errutil.Explain(nil, "property reference %q does not exist", s[start:end+1])
	}

	resolved, err := resolveProperty(p, val)
	if err != nil {
		return "", err
	}

	// resolve the remaining part of the string
	suffix, err := resolveProperty(p, s[end+1:])
	if err != nil {
		return "", err
	}

	// combine: prefix + resolved + suffix
	return s[:start] + resolved + suffix, nil
}

// injectElement injects child plugin elements into a struct field.
func injectElement(fv reflect.Value, ft reflect.StructField, prefix string, tag string, s flatten.Storage) error {

	elemKey := PluginTag(tag).Get("")
	if elemKey == "" {
		return errutil.Explain(nil, "PluginElement tag is empty")
	}

	elemKey, nullable := strings.CutSuffix(elemKey, "?")
	elemKey = prefix + "." + elemKey

	switch fv.Kind() {
	case reflect.Slice:
		return injectArrayElement(fv, ft, elemKey, nullable, PluginTag(tag), s)
	case reflect.Interface, reflect.Pointer:
		return injectSingleElement(fv, ft, elemKey, nullable, PluginTag(tag), s)
	default:
		return errutil.Explain(nil, "unsupported inject type %s", ft.Type.String())
	}
}

// injectSingleElement injects a single plugin element into a struct field.
func injectSingleElement(fv reflect.Value, ft reflect.StructField, prefix string, nullable bool,
	tag PluginTag, s flatten.Storage) error {
	plugin, ok := s.Value(prefix + ".type")
	if !ok {
		plugin, ok = tag.Lookup("default")
		if !ok {
			if nullable {
				return nil
			}
			return errutil.Explain(nil, "no plugin type configured and no default specified")
		}
	}
	v, err := createPlugin(ft.Type, prefix, plugin, s)
	if err != nil {
		return err
	}
	fv.Set(v)
	return nil
}

// injectArrayElement injects an array of plugin elements into a struct field.
func injectArrayElement(fv reflect.Value, ft reflect.StructField, prefix string, nullable bool,
	tag PluginTag, s flatten.Storage) error {
	slice := reflect.MakeSlice(ft.Type, 0, 1)

	// Case 1: Multiple indexed elements
	m := make(map[string]string)
	if s.SliceEntries(prefix, m) {
		if err := validateSliceElementIndexes(prefix, m); err != nil {
			return err
		}
		for k, v := range m {
			newVal, err := resolveProperty(s, v)
			if err != nil {
				return errutil.Stack(err, "resolve property reference error for field at %s", prefix)
			}
			m[k] = newVal
		}
		s = flatten.NewPropertiesStorage(flatten.NewProperties(m))
		for i := 0; ; i++ {
			elemKey := prefix + "[" + strconv.Itoa(i) + "]"
			if !s.Exists(elemKey) { // No more elements
				break
			}
			plugin, _ := s.Value(elemKey + ".type")
			v, err := createPlugin(ft.Type.Elem(), elemKey, plugin, s)
			if err != nil {
				return err
			}
			slice = reflect.Append(slice, v)
		}
		fv.Set(slice)
		return nil
	}

	// Case 2: Single element
	if s.Exists(prefix) {
		plugin, _ := s.Value(prefix + ".type")
		v, err := createPlugin(ft.Type.Elem(), prefix, plugin, s)
		if err != nil {
			return err
		}
		slice = reflect.Append(slice, v)
		fv.Set(slice)
		return nil
	}

	// Case 3: Default values
	defVal, ok := tag.Lookup("default")
	if !ok {
		if nullable {
			return nil
		}
		return errutil.Explain(nil, "no plugin type configured and no default specified")
	}
	for i, plugin := range strings.Split(defVal, ",") {
		if plugin = strings.TrimSpace(plugin); plugin == "" {
			return errutil.Explain(nil, "empty plugin name in default value at index %d", i)
		}
		elemKey := prefix + "[" + strconv.Itoa(i) + "]"
		v, err := createPlugin(ft.Type.Elem(), elemKey, plugin, s)
		if err != nil {
			return err
		}
		slice = reflect.Append(slice, v)
	}
	fv.Set(slice)
	return nil
}

func validateSliceElementIndexes(prefix string, values map[string]string) error {
	return validateSliceIndexes(prefix, values, "plugin element")
}

func validateSliceIndexes(prefix string, values map[string]string, name string) error {
	indexes := make(map[int]struct{})
	for key := range values {
		str, ok := strings.CutPrefix(key, prefix)
		if !ok || str == "" || str[0] != '[' {
			continue
		}
		end := strings.IndexByte(str, ']')
		if end < 0 {
			return errutil.Explain(nil, "invalid %s index %s", name, key)
		}
		index, err := strconv.Atoi(str[1:end])
		if err != nil || index < 0 {
			return errutil.Explain(nil, "invalid %s index %s", name, key)
		}
		indexes[index] = struct{}{}
	}
	for i := range len(indexes) {
		if _, ok := indexes[i]; !ok {
			return errutil.Explain(nil, "missing %s %s[%d]", name, prefix, i)
		}
	}
	return nil
}

// createPlugin creates a plugin instance based on its declared type.
func createPlugin(t reflect.Type, prefix string, plugin string, s flatten.Storage) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.Interface:
		p, ok := pluginRegistry[plugin]
		if !ok {
			return reflect.Value{}, errutil.Explain(nil, "plugin %s not found", plugin)
		}
		if pluginType := reflect.PointerTo(p.Class); !pluginType.AssignableTo(t) {
			return reflect.Value{}, errutil.Explain(nil, "plugin %s does not implement %s", plugin, t.String())
		}
		return newPlugin(p.Class, prefix, s)
	case reflect.Pointer:
		elemType := t.Elem()
		if elemType.Kind() != reflect.Struct {
			return reflect.Value{}, errutil.Explain(nil, "point field must point to a struct")
		}
		p, ok := pluginRegistry[plugin]
		if !ok {
			if plugin != "" {
				return reflect.Value{}, errutil.Explain(nil, "plugin %s not found", plugin)
			}
			p = &Plugin{Class: elemType}
		}
		return newPlugin(p.Class, prefix, s)
	default:
		return reflect.Value{}, errutil.Explain(nil, "unsupported inject type %s", t.String())
	}
}
