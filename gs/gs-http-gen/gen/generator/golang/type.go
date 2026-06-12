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

package golang

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-spring/gs-http-gen/gen/generator"
	"github.com/go-spring/gs-http-gen/lib/httpidl"
	"github.com/go-spring/gs-http-gen/lib/validate"
	"github.com/go-spring/stdlib/errutil"
)

// formatFile formats Go source code using `go format`
// and writes the formatted code to the given file.
func formatFile(fileName string, b []byte) error {
	b, err := format.Source(b)
	if err != nil {
		return errutil.Explain(nil, "format source for file %s error: %w", fileName, err)
	}
	err = os.WriteFile(fileName, b, os.ModePerm)
	if err != nil {
		return errutil.Explain(nil, "write file %s error: %w", fileName, err)
	}
	return nil
}

// formatComments converts a tidl.Comments into Go comments.
func formatComments(c httpidl.Comments) string {
	var lines []string
	for _, s := range c.Above {
		lines = append(lines, s.Text...)
	}
	if c.Right != nil {
		lines = append(lines, c.Right.Text...)
	}
	return strings.Join(lines, "\n")
}

// decodePathValue generates Go code to decode a field value from path parameter.
func decodePathValue(fieldName string, typeName string, typeKind []TypeKind, pathName string) string {
	var sb strings.Builder
	switch typeKind[0] {
	case TypeKindInt:
		sb.WriteString(fmt.Sprintf(`if i, err := strconv.ParseInt(s, 10, 64); err != nil {
				return errutil.Explain(err, "parse path parameter \"%s\" value error")
			} else {
				%s = %s(i)
			}`, pathName, fieldName, typeName))
	case TypeKindUint:
		sb.WriteString(fmt.Sprintf(`if u, err := strconv.ParseUint(s, 10, 64); err != nil {
				return errutil.Explain(err, "parse path parameter \"%s\" value error")
			} else {
				%s = %s(u)
			}`, pathName, fieldName, typeName))
	case TypeKindString:
		sb.WriteString(fmt.Sprintf(`%s = s`, fieldName))
	default:
		panic(fmt.Sprintf(
			"unsupported path param type %s for field %s (path %s)",
			typeName, fieldName, pathName,
		))
	}
	return sb.String()
}

// formEncoder returns the name of the formutil encoder function
// corresponding to the given field type.
func formEncoder(fieldName string, typeName string, typeKind []TypeKind, formName string) string {
	switch typeKind[0] {
	case TypeKindBool:
		return "formutil.EncodeBool"
	case TypeKindBoolPtr:
		return "formutil.EncodeBoolPtr"
	case TypeKindInt, TypeKindEnum:
		return "formutil.EncodeInt"
	case TypeKindIntPtr, TypeKindEnumPtr:
		return "formutil.EncodeIntPtr"
	case TypeKindUint:
		return "formutil.EncodeUint"
	case TypeKindUintPtr:
		return "formutil.EncodeUintPtr"
	case TypeKindFloat:
		return "formutil.EncodeFloat"
	case TypeKindFloatPtr:
		return "formutil.EncodeFloatPtr"
	case TypeKindString:
		return "formutil.EncodeString"
	case TypeKindStringPtr:
		return "formutil.EncodeStringPtr"
	case TypeKindBytes:
		return "formutil.EncodeBytes"
	case TypeKindEnumAsString, TypeKindEnumAsStringPtr, TypeKindStructPtr, TypeKindMap:
		return "formutil.EncodeJSON"
	case TypeKindList:
		return "formutil.EncodeList"
	default:
		panic(errutil.Explain(nil, "unsupported type %s for field %s (form %s)", typeName, fieldName, formName))
	}
}

// encodeFormValue generates Go source code that encodes a struct field
// into application/x-www-form-urlencoded form data.
func encodeFormValue(fieldName string, typeName string, typeKind []TypeKind, formName string) string {
	var sb strings.Builder
	switch typeKind[0] {
	case TypeKindBool, TypeKindBoolPtr, TypeKindInt, TypeKindIntPtr,
		TypeKindUint, TypeKindUintPtr, TypeKindFloat, TypeKindFloatPtr,
		TypeKindString, TypeKindStringPtr, TypeKindBytes,
		TypeKindEnum, TypeKindEnumPtr, TypeKindEnumAsString,
		TypeKindEnumAsStringPtr, TypeKindStructPtr, TypeKindMap:
		encoder := formEncoder(fieldName, typeName, typeKind, formName)
		sb.WriteString(fmt.Sprintf(`if err := %s(form, "%s", %s); err != nil {
			return "", errutil.Explain(err, "encode form field \"%s\" error")
		}`, encoder, formName, fieldName, formName))
	case TypeKindList:
		itemType := strings.TrimPrefix(typeName, "[]")
		itemEncoder := formEncoder(fieldName, itemType, typeKind[1:], formName)
		sb.WriteString(fmt.Sprintf(`if err := formutil.EncodeList(form, "%s", %s, %s); err != nil {
			return "", errutil.Explain(err, "encode form field \"%s\" error")
		}`, formName, fieldName, itemEncoder, formName))
	default:
		panic(errutil.Explain(nil, "unsupported type %s for field %s (form %s)", typeName, fieldName, formName))
	}
	return sb.String()
}

// formDecoder returns the name of the formutil decoder function
// corresponding for the given field type.
func formDecoder(fieldName string, typeName string, typeKind []TypeKind, formName string) string {
	switch typeKind[0] {
	case TypeKindBool:
		return "formutil.DecodeBool"
	case TypeKindBoolPtr:
		return "formutil.DecodeBoolPtr"
	case TypeKindInt, TypeKindEnum:
		return fmt.Sprintf("formutil.DecodeInt[%s]", typeName)
	case TypeKindIntPtr, TypeKindEnumPtr:
		return fmt.Sprintf("formutil.DecodeIntPtr[%s]", strings.TrimPrefix(typeName, "*"))
	case TypeKindUint:
		return fmt.Sprintf("formutil.DecodeUint[%s]", typeName)
	case TypeKindUintPtr:
		return fmt.Sprintf("formutil.DecodeUintPtr[%s]", strings.TrimPrefix(typeName, "*"))
	case TypeKindFloat:
		return fmt.Sprintf("formutil.DecodeFloat[%s]", typeName)
	case TypeKindFloatPtr:
		return fmt.Sprintf("formutil.DecodeFloatPtr[%s]", strings.TrimPrefix(typeName, "*"))
	case TypeKindString:
		return fmt.Sprintf("formutil.DecodeString")
	case TypeKindStringPtr:
		return fmt.Sprintf("formutil.DecodeStringPtr")
	case TypeKindBytes:
		return fmt.Sprintf("formutil.DecodeBytes")
	case TypeKindEnumAsString, TypeKindEnumAsStringPtr, TypeKindStructPtr, TypeKindMap:
		return fmt.Sprintf("formutil.DecodeJSON[%s]", typeName)
	default:
		panic(errutil.Explain(nil, "unsupported type %s for field %s (form %s)", typeName, fieldName, formName))
	}
}

// decodeFormValue generates Go code that decodes a single form field into a struct field.
func decodeFormValue(fieldName string, typeName string, typeKind []TypeKind, formName string) string {
	var sb strings.Builder
	switch typeKind[0] {
	case TypeKindBool, TypeKindBoolPtr, TypeKindInt, TypeKindIntPtr,
		TypeKindUint, TypeKindUintPtr, TypeKindFloat, TypeKindFloatPtr,
		TypeKindString, TypeKindStringPtr, TypeKindBytes,
		TypeKindEnum, TypeKindEnumPtr, TypeKindEnumAsString,
		TypeKindEnumAsStringPtr, TypeKindStructPtr, TypeKindMap:
		decoder := formDecoder(fieldName, typeName, typeKind, formName)
		sb.WriteString(fmt.Sprintf(`if %s, err = %s(key, values); err != nil {
			return errutil.Explain(err, "decode form field \"%s\" error")
		}`, fieldName, decoder, formName))
	case TypeKindList:
		itemType := strings.TrimPrefix(typeName, "[]")
		itemDecoder := formDecoder(fieldName, itemType, typeKind[1:], formName)
		sb.WriteString(fmt.Sprintf(`if %s, err = formutil.DecodeList(key, values, %s); err != nil {
			return errutil.Explain(err, "decode form field \"%s\" error")
		}`, fieldName, itemDecoder, formName))
	default:
		panic(errutil.Explain(nil, "unsupported type %s for field %s (form %s)", typeName, fieldName, formName))
	}
	return sb.String()
}

// genValidateExpr generates the Go code for a validation expression
func genValidateExpr(receiverType, fieldName, fieldType string, expr validate.Expr) (string, error) {
	receiverType = strings.TrimSuffix(receiverType, "Body") // todo

	// 对于结构体而言，只应当验证字段非空，其内部字段的验证应当由自己完成
	fieldType, pointer := strings.CutPrefix(fieldType, "*")
	dollar := "x." + fieldName
	if pointer {
		dollar = "*" + dollar
	}

	// Generate the Go expression for validation
	str, err := compileValidateExpr(dollar, fieldType, expr)
	if err != nil {
		return "", errutil.Explain(err, `failed to generate validate expression for %s.%s`, receiverType, fieldName)
	}

	// Wrap in an if statement returning an error on failure
	str = fmt.Sprintf(`if !(%s) {
		return errutil.Explain(nil, "validate failed on \"%s.%s\"")
	}`, str, receiverType, fieldName)

	if pointer {
		str = fmt.Sprintf(`if x.%s != nil { %s }`, fieldName, str)
	}
	return str, nil
}

// compileValidateExpr recursively generates Go code for a validation expression
func compileValidateExpr(fieldName, fieldType string, expr validate.Expr) (string, error) {
	switch x := expr.(type) {
	case validate.BinaryExpr:
		left, err := compileValidateExpr(fieldName, fieldType, x.Left)
		if err != nil {
			return "", err
		}
		right, err := compileValidateExpr(fieldName, fieldType, x.Right)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s %s", left, x.Op, right), nil

	case validate.UnaryExpr:
		str, err := compileValidateExpr(fieldName, fieldType, x.Expr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%s", x.Op, str), nil

	case *validate.FuncCall:
		if len(x.Args) == 0 {
			return x.Name + "()", nil
		}
		var args []string
		for _, arg := range x.Args {
			str, err := compileValidateExpr(fieldName, fieldType, arg)
			if err != nil {
				return "", err
			}
			args = append(args, str)
		}
		return fmt.Sprintf("%s(%s)", x.Name, strings.Join(args, ", ")), nil

	case *validate.InnerExpr:
		str, err := compileValidateExpr(fieldName, fieldType, x.Expr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s)", str), nil

	case validate.PrimaryExpr:
		if x.Inner != nil {
			return compileValidateExpr(fieldName, fieldType, x.Inner)
		}
		if x.Call != nil {
			return compileValidateExpr(fieldName, fieldType, x.Call)
		}
		if x.Value == "$" {
			return fieldName, nil
		}
		if strings.HasPrefix(x.Value, "'") {
			return quoteValidateString(x.Value)
		}
		return x.Value, nil

	default:
		return "", errutil.Explain(nil, "unknown expression type: %s", x.Text())
	}
}

func quoteValidateString(s string) (string, error) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", errutil.Explain(nil, "invalid validate string literal %s", s)
	}

	var quoted strings.Builder
	quoted.WriteByte('"')
	for i := 1; i < len(s)-1; i++ {
		c := s[i]
		if c == '\\' {
			if i+1 >= len(s)-1 {
				return "", errutil.Explain(nil, "invalid validate string literal %s", s)
			}
			i++
			next := s[i]
			if next == '\'' {
				quoted.WriteByte(next)
			} else {
				quoted.WriteByte(c)
				quoted.WriteByte(next)
			}
			continue
		}
		if c == '"' {
			quoted.WriteByte('\\')
		}
		quoted.WriteByte(c)
	}
	quoted.WriteByte('"')

	v, err := strconv.Unquote(quoted.String())
	if err != nil {
		return "", errutil.Explain(err, "invalid validate string literal %s", s)
	}
	return strconv.Quote(v), nil
}

// genValidateNested generates the nested validation code for a Go struct field.
func genValidateNested(receiverType, fieldName string, itemName string, typeKind []TypeKind, depth int) string {
	receiverType = strings.TrimSuffix(receiverType, "Body") // todo
	childName := fmt.Sprintf("v%d", depth)
	switch typeKind[0] {
	case TypeKindList:
		str := genValidateNested(receiverType, fieldName, childName, typeKind[1:], depth+1)
		if str == "" {
			return ""
		}
		str = fmt.Sprintf(`for _, %s := range %s {
				%s
			}`, childName, itemName, str)
		return str
	case TypeKindMap:
		str := genValidateNested(receiverType, fieldName, childName, typeKind[2:], depth+1)
		if str == "" {
			return ""
		}
		str = fmt.Sprintf(`for _, %s := range %s {
				%s
			}`, childName, itemName, str)
		return str
	case TypeKindStructPtr:
		str := fmt.Sprintf(`if %s != nil {
				if err := %s.Validate(); err != nil {
					return errutil.Explain(err, "validate failed on \"%s.%s\"")
				}
			}`, itemName, itemName, receiverType, fieldName)
		return str
	default:
		return ""
	}
}

// genDecodeJSON generates the JSON decoding code for a Go struct field.
func genDecodeJSON(typeName string, typeKind []TypeKind) string {
	switch typeKind[0] {
	case TypeKindBool:
		return "jsonflow.DecodeBool"
	case TypeKindBoolPtr:
		return "jsonflow.DecodeBoolPtr"
	case TypeKindInt:
		return "jsonflow.DecodeInt[" + typeName + "]"
	case TypeKindIntPtr:
		return "jsonflow.DecodeIntPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindUint:
		return "jsonflow.DecodeUint[" + typeName + "]"
	case TypeKindUintPtr:
		return "jsonflow.DecodeUintPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindFloat:
		return "jsonflow.DecodeFloat[" + typeName + "]"
	case TypeKindFloatPtr:
		return "jsonflow.DecodeFloatPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindString:
		return "jsonflow.DecodeString"
	case TypeKindStringPtr:
		return "jsonflow.DecodeStringPtr"
	case TypeKindBytes:
		return "jsonflow.DecodeBytes"
	case TypeKindAny:
		return "jsonflow.DecodeAny[" + typeName + "]"
	case TypeKindEnum:
		return "jsonflow.DecodeInt[" + typeName + "]"
	case TypeKindEnumPtr:
		return "jsonflow.DecodeIntPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindEnumAsString:
		return "jsonflow.DecodeAny[" + typeName + "]"
	case TypeKindEnumAsStringPtr:
		return "jsonflow.DecodeAny[" + typeName + "]"
	case TypeKindStructPtr:
		return "jsonflow.DecodeObject(New" + strings.TrimPrefix(typeName, "*") + ")"
	case TypeKindList:
		e := genDecodeJSON(strings.TrimPrefix(typeName, "[]"), typeKind[1:])
		return "jsonflow.DecodeArray(" + e + ")"
	case TypeKindMap:
		s := strings.TrimPrefix(typeName, "map[")
		i := strings.Index(s, "]")
		k, v := s[:i], s[i+1:]
		ks := genDecodeJSONKey(k, typeKind[1:2])
		vs := genDecodeJSON(v, typeKind[2:])
		return "jsonflow.DecodeMap(" + ks + ", " + vs + ")"
	default:
		panic("unsupported type")
	}
}

// genDecodeJSONKey generates the JSON decoding code for a JSON object key.
func genDecodeJSONKey(typeName string, typeKind []TypeKind) string {
	switch typeKind[0] {
	case TypeKindString:
		return "jsonflow.DecodeString"
	case TypeKindInt, TypeKindEnum:
		return "jsonflow.DecodeIntKey[" + typeName + "]"
	case TypeKindUint:
		return "jsonflow.DecodeUintKey[" + typeName + "]"
	default:
		panic("unsupported map key type")
	}
}

// genDecodeJSONRequiredCheck generates extra non-null checks for required
// fields whose decoder accepts JSON null.
func genDecodeJSONRequiredCheck(fieldName string, typeKind []TypeKind, jsonName string) string {
	switch typeKind[0] {
	case TypeKindBytes, TypeKindAny, TypeKindList, TypeKindMap:
		return fmt.Sprintf(`if %s == nil {
			return errutil.Explain(nil, "field \"%s\" must not be null")
		}`, fieldName, jsonName)
	default:
		return ""
	}
}

// genEncodeJSON generates the JSON encoding function for a Go struct field.
func genEncodeJSON(typeName string, typeKind []TypeKind) string {
	switch typeKind[0] {
	case TypeKindBool:
		return "jsonflow.EncodeBool[" + typeName + "]"
	case TypeKindBoolPtr:
		return "jsonflow.EncodeBoolPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindInt:
		return "jsonflow.EncodeInt[" + typeName + "]"
	case TypeKindIntPtr:
		return "jsonflow.EncodeIntPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindUint:
		return "jsonflow.EncodeUint[" + typeName + "]"
	case TypeKindUintPtr:
		return "jsonflow.EncodeUintPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindFloat:
		return "jsonflow.EncodeFloat[" + typeName + "]"
	case TypeKindFloatPtr:
		return "jsonflow.EncodeFloatPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindString:
		return "jsonflow.EncodeString[" + typeName + "]"
	case TypeKindStringPtr:
		return "jsonflow.EncodeStringPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindBytes:
		return "jsonflow.EncodeBytes"
	case TypeKindAny:
		return "jsonflow.EncodeAny[" + typeName + "]"
	case TypeKindEnum:
		return "jsonflow.EncodeInt[" + typeName + "]"
	case TypeKindEnumPtr:
		return "jsonflow.EncodeIntPtr[" + strings.TrimPrefix(typeName, "*") + "]"
	case TypeKindEnumAsString:
		return "jsonflow.EncodeAny[" + typeName + "]"
	case TypeKindEnumAsStringPtr:
		return "jsonflow.EncodeAny[" + typeName + "]"
	case TypeKindStructPtr:
		return "jsonflow.EncodeObject[" + typeName + "]"
	case TypeKindList:
		e := genEncodeJSON(strings.TrimPrefix(typeName, "[]"), typeKind[1:])
		return "jsonflow.EncodeArray(" + e + ")"
	case TypeKindMap:
		s := strings.TrimPrefix(typeName, "map[")
		i := strings.Index(s, "]")
		k, v := s[:i], s[i+1:]
		ks := genEncodeJSONKey(k, typeKind[1:2])
		vs := genEncodeJSON(v, typeKind[2:])
		return "jsonflow.EncodeMap(" + ks + ", " + vs + ")"
	default:
		panic("unsupported type")
	}
}

// genEncodeJSONKey generates the JSON encoding function for a map key.
func genEncodeJSONKey(typeName string, typeKind []TypeKind) string {
	switch typeKind[0] {
	case TypeKindString:
		return "jsonflow.EncodeStringKey[" + typeName + "]"
	case TypeKindInt, TypeKindEnum:
		return "jsonflow.EncodeIntKey[" + typeName + "]"
	case TypeKindUint:
		return "jsonflow.EncodeUintKey[" + typeName + "]"
	default:
		panic("unsupported map key type")
	}
}

// genIsEmptyJSON generates a Go expression that reports whether a field should
// be omitted for a JSON tag with omitempty.
func genIsEmptyJSON(fieldName string, typeKind []TypeKind) string {
	switch typeKind[0] {
	case TypeKindBool:
		return "!" + fieldName
	case TypeKindBoolPtr, TypeKindIntPtr, TypeKindUintPtr,
		TypeKindFloatPtr, TypeKindStringPtr, TypeKindEnumPtr,
		TypeKindEnumAsStringPtr, TypeKindStructPtr, TypeKindAny:
		return fieldName + " == nil"
	case TypeKindInt, TypeKindUint, TypeKindFloat,
		TypeKindEnum, TypeKindEnumAsString:
		return fieldName + " == 0"
	case TypeKindString:
		return fieldName + ` == ""`
	case TypeKindBytes, TypeKindList, TypeKindMap:
		return "len(" + fieldName + ") == 0"
	default:
		panic("unsupported type")
	}
}

// typeTmpl is a Go template used to generate Go source code from IDL definitions.
var typeTmpl = template.Must(template.New("type").
	Funcs(map[string]any{
		"formatComments":             formatComments,
		"decodePathValue":            decodePathValue,
		"encodeFormValue":            encodeFormValue,
		"decodeFormValue":            decodeFormValue,
		"genValidateExpr":            genValidateExpr,
		"genValidateNested":          genValidateNested,
		"genDecodeJSON":              genDecodeJSON,
		"genDecodeJSONRequiredCheck": genDecodeJSONRequiredCheck,
		"genEncodeJSON":              genEncodeJSON,
		"genIsEmptyJSON":             genIsEmptyJSON,
	}).
	Parse(`
// Code generated by gs-http-gen compiler. DO NOT EDIT.

package {{.Package}}

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/hashutil"
	"github.com/go-spring/stdlib/httpsvr"
	"github.com/go-spring/stdlib/formutil"
	"github.com/go-spring/stdlib/jsonflow"
)

var _ = strings.Index
var _ = url.ParseQuery
var _ = strconv.FormatInt
var _ = base64.StdEncoding
var _ = http.StatusNotFound
var _ = (*httpsvr.Router)(nil)
var _ = formutil.EncodeInt[int]

{{ range $c := .Consts }}
	{{- if $c.Comments.Exists }}
		{{formatComments $c.Comments}}
	{{- end}}
	const {{$c.Name}} {{$c.Type}} = {{$c.Value}}
{{end}}

{{ range $e := .Enums }}
	{{- if $e.Comments.Exists }}
		{{formatComments $e.Comments}}
	{{- end}}
	type {{$e.Name}} int32

	const (
		{{ range $f := $e.Fields }}
			{{- if $f.Comments.Exists }}
				{{formatComments $f.Comments}}
			{{- end}}
			{{$e.Name}}_{{$f.Name}} {{$e.Name}} = {{$f.Value}}
		{{- end}}
	)

	var (
		{{$e.Name}}_name = map[{{$e.Name}}]string{
			{{- range $f := $e.Fields }}
				{{$f.Value}} : "{{$f.Name}}",
			{{- end}}
		}
		{{$e.Name}}_value = map[string]{{$e.Name}}{
			{{- range $f := $e.Fields }}
				"{{$f.Name}}" : {{$f.Value}},
			{{- end}}
		}
		{{- if $e.KindError }} {{- /* only for error */}}
			{{$e.Name}}_message = map[{{$e.Name}}]string{
				{{- range $f := $e.Fields }}
					{{- if $f.ErrorMessage }}
						{{$f.Value}} : "{{$f.ErrorMessage}}",
					{{- else}}
						{{$f.Value}} : "{{$f.Name}}",
					{{- end}}
				{{- end}}
			}
		{{- end}}
	)

	// OneOf{{$e.Name}} reports whether it's a valid {{$e.Name}}.
	func OneOf{{$e.Name}}(i {{$e.Name}}) bool {
		_, ok := {{$e.Name}}_name[i]
		return ok
	}

	// OneOf{{$e.Name}}AsString reports whether it's a valid {{$e.Name}}AsString.
	func OneOf{{$e.Name}}AsString(i {{$e.Name}}AsString) bool {
		_, ok := {{$e.Name}}_name[{{$e.Name}}(i)]
		return ok
	}

	// {{$e.Name}}AsString wraps {{$e.Name}} to encode/decode as a JSON string.
	type {{$e.Name}}AsString {{$e.Name}}

	// MarshalJSON encodes the enum value as its string name.
	func (x {{$e.Name}}AsString) MarshalJSON() ([]byte, error) {
		if s, ok := {{$e.Name}}_name[{{$e.Name}}(x)]; ok {
			return []byte(fmt.Sprintf("\"%s\"", s)), nil
		}
		return nil, errutil.Explain(nil,"invalid {{$e.Name}}AsString: %d", x)
	}

	// UnmarshalJSON decodes the enum value from its string name.
	func (x *{{$e.Name}}AsString) UnmarshalJSON(data []byte) error {
		str, err := strconv.Unquote(string(data))
		if err != nil {
			return errutil.Explain(err,"invalid {{$e.Name}}AsString JSON string")
		}
		if v, ok := {{$e.Name}}_value[str]; ok {
			*x = {{$e.Name}}AsString(v)
			return nil
		}
		return errutil.Explain(nil,"invalid {{$e.Name}}AsString value: %q", str)
	}
{{end}}

{{ range $s := .Structs }}
	{{- if not $s.Request }}
		{{- if $s.Comments.Exists }}
			{{formatComments $s.Comments}}
		{{- end}}
		type {{$s.Name}} struct {
			{{- range $f := $s.Fields }}
				{{- if $f.Comments.Exists }}
					{{formatComments $f.Comments}}
				{{- end}}
				{{$f.Name}} {{$f.Type}} {{$f.FieldTag}}
			{{- end}}
		}

		// New{{$s.Name}} creates a new {{$s.Name}} instance
		// and initializes fields with default values.
		func New{{$s.Name}}() *{{$s.Name}} {
			return &{{$s.Name}}{
				{{- range $f := $s.Fields }}
					{{- if $f.CompatDefault}}
						{{$f.Name}}: {{$f.Default}},
					{{- end}}
				{{- end}}
			}
		}

		// DecodeJSON decodes a JSON object into {{$s.Name}} using a hash-based
		// field dispatch mechanism for high-performance parsing.
		func (r *{{$s.Name}}) DecodeJSON(d jsonflow.Decoder) (err error) {
			{{- if $s.FieldCount }}
				const (
					{{- range $f := $s.Fields }}
						hash{{$f.Name}} = {{$f.JSONTag.HashKey}} // HashKey("{{$f.JSONTag.Name}}")
					{{- end}}
				)
			{{- end}}

			{{- $HasRequired := false }}
			{{- range $f := $s.Fields }}
				{{- if and $f.Required (not $f.CompatDefault) }}
					{{ $HasRequired = true }}
				{{- end}}
			{{- end}}

			{{- if $HasRequired }}
				var (
					{{- range $f := $s.Fields }}
						{{- if and $f.Required (not $f.CompatDefault) }}
							has{{$f.Name}} bool
						{{- end}}
					{{- end}}
				)
			{{- end}}

			if err = jsonflow.DecodeObjectBegin(d); err != nil {
				return err
			}

			for {
				if d.PeekKind() == '}' {
					break
				}

				var key string
				key, err = jsonflow.DecodeString(d)
				if err != nil {
					return err
				}

				switch hashutil.FNV1a64(key) {
				{{- range $f := $s.Fields }}
					case hash{{$f.Name}}:
						if key != "{{$f.JSONTag.Name}}" {
							if err = d.SkipValue(); err != nil {
								return err
							}
							continue
						}
						{{- if and $f.Required (not $f.CompatDefault) }}
							has{{$f.Name}} = true
						{{- end}}
						if r.{{$f.Name}}, err = {{genDecodeJSON $f.Type $f.TypeKind}}(d); err != nil {
							return err
						}
						{{- if $f.Required }}
							{{- $fieldName := printf "r.%s" $f.Name}}
							{{genDecodeJSONRequiredCheck $fieldName $f.TypeKind $f.JSONTag.Name}}
						{{- end}}
				{{- end}}
				default:
					if err = d.SkipValue(); err != nil {
						return err
					}
				}
			}

			if err = jsonflow.DecodeObjectEnd(d); err != nil {
				return err
			}

			{{- if $HasRequired }}
				{{ range $f := $s.Fields }}
					{{- if and $f.Required (not $f.CompatDefault) }}
						if !has{{$f.Name}} {
							return errutil.Explain(err, "missing required field \"{{$f.JSONTag.Name}}\"")
						}
					{{- end}}
				{{- end}}
			{{- end}}
			{{- if $s.OneOf }}
				if err = r.validateOneOf(); err != nil {
					return err
				}
			{{- end}}
			return 
		}

		// EncodeJSON encodes {{$s.Name}} into a JSON object using streaming
		// token writes.
		func (x *{{$s.Name}}) EncodeJSON(e jsonflow.Encoder) error {
			if x == nil {
				return jsonflow.EncodeNull(e)
			}
			{{- if $s.OneOf }}
				if err := x.validateOneOf(); err != nil {
					return err
				}
			{{- end}}
			if err := e.WriteObjectBegin(); err != nil {
				return err
			}
			{{- range $f := $s.Fields }}
				{{- $fieldName := printf "x.%s" $f.Name}}
				{{- if $f.JSONTag.OmitEmpty }}
					if !({{genIsEmptyJSON $fieldName $f.TypeKind}}) {
						if err := e.WriteString("{{$f.JSONTag.Name}}"); err != nil {
							return err
						}
						if err := {{genEncodeJSON $f.Type $f.TypeKind}}(e, {{$fieldName}}); err != nil {
							return err
						}
					}
				{{- else}}
					if err := e.WriteString("{{$f.JSONTag.Name}}"); err != nil {
						return err
					}
					if err := {{genEncodeJSON $f.Type $f.TypeKind}}(e, {{$fieldName}}); err != nil {
						return err
					}
				{{- end}}
			{{- end}}
			return e.WriteObjectEnd()
		}

		{{- if $s.OneOf }}
			func (x *{{$s.Name}}) validateOneOf() error {
				if x == nil {
					return nil
				}
				var (
					count int
					fieldType {{$s.Name}}TypeAsString
				)
				{{- range $f := $s.Fields }}
					{{- if ne $f.Name "FieldType" }}
						if x.{{$f.Name}} != nil {
							count++
							fieldType = {{$s.Name}}TypeAsString({{$s.Name}}Type_{{$f.Name}})
						}
					{{- end}}
				{{- end}}
				if count != 1 {
					return errutil.Explain(nil, "oneof {{$s.Name}} must have exactly one member set")
				}
				if x.FieldType != fieldType {
					return errutil.Explain(nil, "oneof {{$s.Name}} field type does not match active member")
				}
				return nil
			}
		{{- end}}

		{{- $Validate := false}}
		{{- if $s.Validate }}
			{{- $Validate = true}}
		{{- end}}
		{{- if $s.OneOf }}
			{{- $Validate = true}}
		{{- end}}
		{{- range $f := $s.Fields }}
			{{- if $f.ValidateExpr }}
				{{- $Validate = true}}
			{{- end}}
			{{- if $f.ValidateNested }}
				{{- $Validate = true}}
			{{- end}}
		{{- end}}

		{{- if $Validate}}
			// Validate checks field values using generated validation expressions.
			func (x *{{$s.Name}}) Validate() error {
				{{- if $s.OneOf }}
					if err := x.validateOneOf(); err != nil {
						return err
					}
				{{- end}}
				{{- range $f := $s.Fields }}
					{{- if $f.ValidateExpr }}
						{{genValidateExpr $s.Name $f.Name $f.Type $f.ValidateExpr}}
					{{- end}}
					{{- if $f.ValidateNested }}
						{{- $fieldName := printf "x.%s" $f.Name}}
						{{genValidateNested $s.Name $f.Name $fieldName $f.TypeKind 0}}
					{{- end}}
				{{- end}}
				return nil
			}
		{{- end}}
	{{- end}} {{- /* end of struct (not request) */}}

	{{- if $s.Request }}
		{{- if $s.Comments.Exists }}
			{{formatComments $s.Comments}}
		{{- end}}
		type {{$s.Name}} struct {
			{{$s.Name}}Body
			{{- range $f := $s.Fields }}
				{{- if $f.Binding }}
					{{- if $f.Comments.Exists }}
						{{formatComments $f.Comments}}
					{{- end}}
					{{$f.Name}} {{$f.Type}} {{$f.FieldTag}}
				{{- end}}
			{{- end}}
		}

		// New{{$s.Name}} creates a new {{$s.Name}} instance
		// and initializes fields with default values.
		func New{{$s.Name}}() *{{$s.Name}} {
			return &{{$s.Name}}{
				{{- range $f := $s.Fields }}
					{{- if $f.Binding }}
						{{- if $f.CompatDefault}}
							{{$f.Name}}: {{$f.Default}},
						{{- end}}
					{{- end}}
				{{- end}}
			}
		}

		// QueryForm encodes query-bound fields into URL-encoded form data.
		func (x *{{$s.Name}}) QueryForm() (string, error) {
			{{- if $s.QueryCount }}
				form := make(url.Values)
				{{- range $f := $s.Fields }}
					{{- if $f.Binding }}
						{{- if eq $f.Binding.Source "query" }}
							{{$fieldName := printf "x.%s" $f.Name}}
							{{- encodeFormValue $fieldName $f.Type $f.TypeKind $f.Binding.Field }}
						{{- end}}
					{{- end}}
				{{- end}}
				return form.Encode(), nil
			{{- else}}
				return "", nil
			{{- end}}
		}

		// Bind extracts path and query parameters from the HTTP request
		// and assigns them to the corresponding struct fields.
		func (x *{{$s.Name}}) Bind(r *http.Request) error {
			{{- if $s.BindingCount }}
				{{- if $s.PathCount }}
					c := httpsvr.GetRequestContext(r.Context())
					{{- range $f := $s.Fields }}
						{{- if and $f.Binding (eq $f.Binding.Source "path") }}
							if s := c.PathValue("{{$f.Binding.Field}}"); s == "" {
								return errutil.Explain(nil, "required path parameter \"{{$f.Binding.Field}}\" is missing")
							} else {
								{{- $fieldName := printf "x.%s" $f.Name}}
								{{decodePathValue $fieldName $f.Type $f.TypeKind $f.Binding.Field}}
							}
						{{- end}}
					{{- end}}
				{{- end}}

				{{- if $s.QueryCount }}
					form, err := url.ParseQuery(r.URL.RawQuery)
					if err != nil {
						return errutil.Explain(err, "parse query error")
					}

					{{- $HasRequired := false }}
					{{- range $f := $s.Fields }}
						{{- if and $f.Binding (eq $f.Binding.Source "query") }}
							{{- if and $f.Required (not $f.CompatDefault) }}
								{{ $HasRequired = true }}
							{{- end}}
						{{- end}}
					{{- end}}

					{{- if $HasRequired }}
						var (
							{{- range $f := $s.Fields }}
								{{- if and $f.Binding (eq $f.Binding.Source "query") }}
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} bool
									{{- end}}
								{{- end}}
							{{- end}}
						)
					{{end}}

					for key, values := range form {
						if len(values) == 0 {
							continue
						}
						switch key {
						{{- range $f := $s.Fields }}
							{{- if and $f.Binding (eq $f.Binding.Source "query") }}
								case "{{$f.Binding.Field}}":
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} = true
									{{- end}}
									{{- $fieldName := printf "x.%s" $f.Name}}
									{{decodeFormValue $fieldName $f.Type $f.TypeKind $f.Binding.Field}}
							{{- end}}
						{{- end}}
						}
					}

					{{/* newline */}}
					{{- if $HasRequired }}
						{{- range $f := $s.Fields }}
							{{- if and $f.Binding (eq $f.Binding.Source "query") }}
								{{- if and $f.Required (not $f.CompatDefault) }}
									if !has{{$f.Name}} {
										err = errutil.Explain(err, "missing required field \"{{$f.Binding.Field}}\"")
									}
								{{- end}}
							{{- end}}
						{{- end}}
					{{- end}}
				{{- end}}
				return nil
			{{- else}}
				return nil
			{{- end}}
		}

		// Validate validates both bound parameters and request body fields.
		func (x *{{$s.Name}}) Validate() error {
			{{- range $f := $s.Fields }}
				{{- if $f.Binding }}
					{{- if $f.ValidateExpr }}
						{{genValidateExpr $s.Name $f.Name $f.Type $f.ValidateExpr}}
					{{- end}}
					{{- if $f.ValidateNested }}
						{{- $fieldName := printf "x.%s" $f.Name}}
						{{genValidateNested $s.Name $f.Name $fieldName $f.TypeKind 0}}
					{{- end}}
				{{- end}}
			{{- end}}
			if err := x.{{$s.Name}}Body.Validate(); err != nil {
				return errutil.Explain(err, "validate failed on \"{{$s.Name}}\"")
			}
			return nil
		}

		// {{$s.Name}}Body represents the request body payload,
		// excluding path and query parameters.
		type {{$s.Name}}Body struct {
			{{- range $f := $s.Fields }}
				{{- if not $f.Binding }}
					{{- if $f.Comments.Exists }}
						{{formatComments $f.Comments}}
					{{- end}}
					{{$f.Name}} {{$f.Type}} {{$f.FieldTag}}
				{{- end}}
			{{- end}}
		}

		// New{{$s.Name}}Body creates a new {{$s.Name}}Body instance
		// and initializes fields with default values.
		func New{{$s.Name}}Body() *{{$s.Name}}Body {
			return &{{$s.Name}}Body{
				{{- range $f := $s.Fields }}
					{{- if not $f.Binding }}
						{{- if $f.CompatDefault}}
							{{$f.Name}}: {{$f.Default}},
						{{- end}}
					{{- end}}
				{{- end}}
			}
		}

		{{- if $s.FormEncoded }}
			// EncodeForm encodes the request body as application/x-www-form-urlencoded data.
			func (x *{{$s.Name}}Body) EncodeForm() (string, error) {
				{{- if $s.BodyCount }}
					form := make(url.Values)
					{{- range $f := $s.Fields }}
						{{- if not $f.Binding }}
							{{$fieldName := printf "x.%s" $f.Name}}
							{{- encodeFormValue $fieldName $f.Type $f.TypeKind $f.FormTag.Name }}
						{{- end}}
					{{- end}}
					return form.Encode(), nil
				{{- else}}
					return "", nil
				{{- end}}
			}

			// DecodeForm decodes application/x-www-form-urlencoded data into the request body.
			func (x *{{$s.Name}}Body) DecodeForm(b []byte) error {
				{{- if $s.BodyCount }}
					form, err := url.ParseQuery(string(b))
					if err != nil {
						return errutil.Explain(err, "parse query error")
					}

					{{- $HasRequired := false }}
					{{- range $f := $s.Fields }}
						{{- if not $f.Binding }}
							{{- if and $f.Required (not $f.CompatDefault) }}
								{{ $HasRequired = true }}
							{{- end}}
						{{- end}}
					{{- end}}

					{{- if $HasRequired }}
						var (
							{{- range $f := $s.Fields }}
								{{- if not $f.Binding }}
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} bool
									{{- end}}
								{{- end}}
							{{- end}}
						)
					{{end}}

					for key, values := range form {
						if len(values) == 0 {
							continue
						}
						switch key {
						{{- range $f := $s.Fields }}
							{{- if not $f.Binding }}
								case "{{$f.FormTag.Name}}":
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} = true
									{{- end}}
									{{- $fieldName := printf "x.%s" $f.Name}}
									{{decodeFormValue $fieldName $f.Type $f.TypeKind $f.FormTag.Name}}
							{{- end}}
						{{- end}}
						}
					}

					{{- if $HasRequired }}
						{{- range $f := $s.Fields }}
							{{- if not $f.Binding }}
								{{- if and $f.Required (not $f.CompatDefault) }}
									if !has{{$f.Name}} {
										return errutil.Explain(err, "missing required field \"{{$f.FormTag.Name}}\"")
									}
								{{- end}}
							{{- end}}
						{{- end}}
					{{- end}}

					return nil
				{{- else}}
					return nil
				{{- end}}
			}
		{{- end}} {{- /* end of form encoded */}}

		{{- if $s.JSONEncoded }}
			// DecodeJSON decodes a JSON object into {{$s.Name}}Body using a hash-based
			// field dispatch mechanism for high-performance parsing.
			func (r *{{$s.Name}}Body) DecodeJSON(d jsonflow.Decoder) (err error) {
				{{- if $s.BodyCount }}
					const (
						{{- range $f := $s.Fields }}
							{{- if not $f.Binding }}
								hash{{$f.Name}} = {{$f.JSONTag.HashKey}} // HashKey("{{$f.JSONTag.Name}}")
							{{- end}}
						{{- end}}
					)

					{{- $HasRequired := false }}
					{{- range $f := $s.Fields }}
						{{- if not $f.Binding }}
							{{- if and $f.Required (not $f.CompatDefault) }}
								{{ $HasRequired = true }}
							{{- end}}
						{{- end}}
					{{- end}}

					{{- if $HasRequired }}
						var (
							{{- range $f := $s.Fields }}
								{{- if not $f.Binding }}
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} bool
									{{- end}}
								{{- end}}
							{{- end}}
						)
					{{- end}}

					if err = jsonflow.DecodeObjectBegin(d); err != nil {
						return err
					}

					for {
						if d.PeekKind() == '}' {
							break
						}
	
						var key string
						key, err = jsonflow.DecodeString(d)
						if err != nil {
							return err
						}
	
						switch hashutil.FNV1a64(key) {
						{{- range $f := $s.Fields }}
							{{- if not $f.Binding }}
								case hash{{$f.Name}}:
									if key != "{{$f.JSONTag.Name}}" {
										if err = d.SkipValue(); err != nil {
											return err
										}
										continue
									}
									{{- if and $f.Required (not $f.CompatDefault) }}
										has{{$f.Name}} = true
									{{- end}}
									if r.{{$f.Name}}, err = {{genDecodeJSON $f.Type $f.TypeKind}}(d); err != nil {
										return err
									}
									{{- if $f.Required }}
										{{- $fieldName := printf "r.%s" $f.Name}}
										{{genDecodeJSONRequiredCheck $fieldName $f.TypeKind $f.JSONTag.Name}}
									{{- end}}
							{{- end}}
						{{- end}}
						default:
							if err = d.SkipValue(); err != nil {
								return err
							}
						}
					}

					if err = jsonflow.DecodeObjectEnd(d); err != nil {
						return err
					}
	
					{{- if $HasRequired }}
						{{ range $f := $s.Fields }}
							{{- if not $f.Binding }}
								{{- if and $f.Required (not $f.CompatDefault) }}
									if !has{{$f.Name}} {
										return errutil.Explain(err, "missing required field \"{{$f.JSONTag.Name}}\"")
									}
								{{- end}}
							{{- end}}
						{{- end}}
					{{- end}}
				{{- end}}
				return 
			}

			// EncodeJSON encodes {{$s.Name}}Body into a JSON object using streaming
			// token writes.
			func (x *{{$s.Name}}Body) EncodeJSON(e jsonflow.Encoder) error {
				if x == nil {
					return jsonflow.EncodeNull(e)
				}
				if err := e.WriteObjectBegin(); err != nil {
					return err
				}
				{{- range $f := $s.Fields }}
					{{- if not $f.Binding }}
						{{- $fieldName := printf "x.%s" $f.Name}}
						{{- if $f.JSONTag.OmitEmpty }}
							if !({{genIsEmptyJSON $fieldName $f.TypeKind}}) {
								if err := e.WriteString("{{$f.JSONTag.Name}}"); err != nil {
									return err
								}
								if err := {{genEncodeJSON $f.Type $f.TypeKind}}(e, {{$fieldName}}); err != nil {
									return err
								}
							}
						{{- else}}
							if err := e.WriteString("{{$f.JSONTag.Name}}"); err != nil {
								return err
							}
							if err := {{genEncodeJSON $f.Type $f.TypeKind}}(e, {{$fieldName}}); err != nil {
								return err
							}
						{{- end}}
					{{- end}}
				{{- end}}
				return e.WriteObjectEnd()
			}
		{{end}} {{- /* end of json encoded */}}

		// Validate checks field values using generated validation expressions.
		func (x *{{$s.Name}}Body) Validate() error {
			{{- range $f := $s.Fields }}
				{{- if not $f.Binding }}
					{{- if $f.ValidateExpr }}
						{{genValidateExpr $s.Name $f.Name $f.Type $f.ValidateExpr}}
					{{- end}}
					{{- if $f.ValidateNested }}
						{{- $fieldName := printf "x.%s" $f.Name}}
						{{genValidateNested $s.Name $f.Name $fieldName $f.TypeKind 0}}
					{{- end}}
				{{- end}}
			{{- end}}
			return nil
		}

	{{end}} {{- /* end of struct (request) */}}
{{end}}
`))

// genType generates a Go source file corresponding to the IDL file.
// It includes constants, enums, and struct types.
func (g *Generator) genType(config *generator.Config, fileName string, spec GoSpec) error {
	buf := &bytes.Buffer{}
	err := typeTmpl.Execute(buf, map[string]any{
		"Package": config.GoPackage,
		"Consts":  spec.Consts[fileName],
		"Enums":   spec.Enums[fileName],
		"Structs": spec.Types[fileName],
	})
	if err != nil {
		return errutil.Explain(nil, "execute type template error: %w", err)
	}
	fileName = fileName[:strings.LastIndex(fileName, ".")] + ".go"
	fileName = filepath.Join(config.OutputDir, fileName)
	return formatFile(fileName, buf.Bytes())
}
