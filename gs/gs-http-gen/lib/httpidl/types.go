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

package httpidl

import (
	"github.com/go-spring/gs-http-gen/lib/pathidl"
	"github.com/go-spring/gs-http-gen/lib/validate"
)

// MetaInfo represents metadata about the parsed document.
type MetaInfo struct {
	Name        string         `json:"name"`        // Project name
	Description string         `json:"description"` // Project description
	Version     string         `json:"version"`     // Version
	Import      []string       `json:"import"`      // External projects or IDL files
	Config      map[string]any `json:"config"`      // Other custom metadata
}

// Position represents the source line range of a parsed element.
// This allows tracing back to the original source code location.
type Position struct {
	StartLine int
	EndLine   int
}

// Comment represents a single comment block or line.
// Single == true means it was parsed from a single-line comment (e.g. //).
// Single == false means it was parsed from a multi-line block comment (e.g. /* ... */).
type Comment struct {
	Text     []string
	Single   bool
	Position Position
}

// Comments groups the two major comment placements:
//   - Above: comments located above a declaration.
//   - Right: comments located at the end of a declaration's line.
type Comments struct {
	Above []Comment
	Right *Comment
}

// Exists reports whether any comments (above or right) are associated with the node.
func (c Comments) Exists() bool {
	return len(c.Above) > 0 || c.Right != nil
}

// Document represents the root node of the parsed file.
// It contains all top-level definitions such as constants, enums, types, and RPCs.
// Additionally, it stores any global comments that are not attached to specific nodes.
type Document struct {
	Comments []Comment // Top-level comments (not associated with any element)
	Consts   []Const   // Constant definitions
	Enums    []Enum    // Enum definitions
	Types    []Type    // Type definitions (structs, generics, instantiations, etc.)
	RPCs     []RPC     // Function definitions (HTTP request/response RPCs)

	EnumTypes map[string]int      // Lookup: enum name → index in Enums
	TypeTypes map[string]int      // Lookup: type name → index in Types
	UserTypes map[string]struct{} // User-defined types referenced in this file
}

// Annotation represents a key-value metadata entry attached to a type,
// field, or RPC. The value is optional and represented as raw text.
type Annotation struct {
	Key      string   // Annotation key (e.g., "deprecated")
	Value    *string  // Optional annotation value
	Position Position // Location in the source file
	Comments Comments // Associated comments
}

// Const represents a constant definition in the parsed document.
type Const struct {
	Type     BaseType // Data type of the constant (base types only)
	Name     string   // Name of the constant
	Value    string   // Literal value
	Position Position // Location in the source file
	Comments Comments // Associated comments
}

type EnumKind int

const (
	EnumKindNormal EnumKind = iota
	EnumKindOneOf
	EnumKindError
	EnumKindExtends
)

// Enum represents an enum type definition.
type Enum struct {
	Name     string      // Name of the enum
	Kind     EnumKind    // Enum kind
	Fields   []EnumField // List of fields
	Position Position    // Location in the source file
	Comments Comments    // Associated comments
}

// KindError returns true if the enum is an error-code enum.
func (e Enum) KindError() bool {
	return e.Kind == EnumKindError
}

// EnumField represents a single field inside an enum definition.
type EnumField struct {
	Name         string       // Name of the enum field
	Value        int64        // Integer value assigned to the enum field
	ExtendsFrom  *string      // File name of the field is inherited from
	ErrorMessage *string      // Error message (only for error-code enums)
	Annotations  []Annotation // Attached annotations
	Position     Position     // Location in the source file
	Comments     Comments     // Associated comments
}

// TypeDefinition is implemented by all types representable in IDL.
// The Text method returns a human-readable textual representation.
type TypeDefinition interface {
	Text() string
}

// JSONTag represents metadata for a JSON field tag.
type JSONTag struct {
	Name      string // JSON field name
	HashKey   string // Hash key used for field matching during decoding
	OmitEmpty bool   // Do not serialize this field when it is null
}

// FormTag represents metadata for a form field binding.
type FormTag struct {
	Name    string // Key used in form submissions
	HashKey string // Hash key used for field matching during decoding
}

// Binding represents a binding between a struct field and an HTTP input
// (e.g., from a query string or from a path segment).
type Binding struct {
	Source string // Binding source ("path" or "query")
	Field  string // Field name from the source
}

type Encoding int8

const (
	EncodingJSON = Encoding(iota) // JSON encoding
	EncodingForm                  // Form encoding
)

// Type represents a user-defined type. It may function like a struct,
// a oneof union, or an alias to another instanced type.
type Type struct {
	Name         string      // Name of the type
	OneOf        bool        // True if representing a oneof type
	InstType     *InstType   // Represents a generic instantiation
	GenericParam *string     // Optional generic parameter
	RawFields    []TypeField // Original, unmodified fields
	Fields       []TypeField // Actual fields after processing
	Position     Position    // Location in the source file
	Comments     Comments    // Associated comments

	Embedded bool     // True if contains anonymous embedded fields
	Request  bool     // Indicates this type is used as an HTTP request (root) type
	Validate bool     // Indicates validation code should be generated for this type
	Encoding Encoding // Indicates this request type represents form-encoded data
}

// JSONEncoded returns true if this type represents JSON-encoded data.
func (t *Type) JSONEncoded() bool {
	return t.Encoding == EncodingJSON
}

// FormEncoded returns true if this type represents form-encoded data.
func (t *Type) FormEncoded() bool {
	return t.Encoding == EncodingForm
}

// FieldCount returns the total number of fields after all processing.
func (t *Type) FieldCount() int {
	return len(t.Fields)
}

// BodyCount returns the number of fields that are not bound to
// a specific HTTP input source (e.g., path or query).
func (t *Type) BodyCount() int {
	var count int
	for _, f := range t.Fields {
		if f.Binding == nil {
			count++
		}
	}
	return count
}

// BindingCount returns the number of fields that have explicit HTTP
// binding information, such as path or query bindings.
func (t *Type) BindingCount() int {
	var count int
	for _, f := range t.Fields {
		if f.Binding != nil {
			count++
		}
	}
	return count
}

// PathCount returns the number of fields that are explicitly bound
// to HTTP path parameters (i.e., Binding.Source == "path").
func (t *Type) PathCount() int {
	var count int
	for _, f := range t.Fields {
		if f.Binding != nil && f.Binding.Source == "path" {
			count++
		}
	}
	return count
}

// QueryCount returns the number of fields that are explicitly bound
// to HTTP query parameters (i.e., Binding.Source == "query").
func (t *Type) QueryCount() int {
	var count int
	for _, f := range t.Fields {
		if f.Binding != nil && f.Binding.Source == "query" {
			count++
		}
	}
	return count
}

// TypeField represents a field inside a user-defined type.
type TypeField struct {
	Name        string         // Name of the field
	Type        TypeDefinition // Type of the field
	Annotations []Annotation   // Attached annotations
	Position    Position       // Location in the source file
	Comments    Comments       // Associated comments

	Deprecated     bool          // True if marked as deprecated
	Required       bool          // Whether the field is required
	CompatDefault  *string       // Default value for compatibility
	JSONTag        JSONTag       // JSON serialization tag info
	FormTag        FormTag       // Form tag info
	Binding        *Binding      // Path/query parameter binding
	ValidateExpr   validate.Expr // Validation expression
	ValidateNested bool          // Whether recursive validation is required
	EnumAsString   bool          // Whether the enum should be marshaled as a string
}

// InstType represents an instantiation of a generic type.
type InstType struct {
	BaseName    string         // Name of the generic type being instantiated
	GenericType TypeDefinition // The concrete type argument applied to the generic
}

func (t InstType) Text() string {
	return t.BaseName + "<" + t.GenericType.Text() + ">"
}

// EmbedType represents an embedded type field (similar to embedded structs in Go).
type EmbedType struct {
	Name string // Name of the embedded type
}

func (t EmbedType) Text() string {
	return t.Name
}

// BaseType represents a primitive type (e.g., int, string, bool).
type BaseType struct {
	Name string // Name of the primitive type
}

func (t BaseType) Text() string {
	return t.Name
}

// UserType represents a reference to a user-defined type.
type UserType struct {
	Name string // Name of the user-defined type
}

func (t UserType) Text() string {
	return t.Name
}

// BytesType represents the "bytes" type (raw bytes).
type BytesType struct{}

func (t BytesType) Text() string {
	return "bytes"
}

// MarshalText implements encoding.TextMarshaler for BytesType.
func (t BytesType) MarshalText() ([]byte, error) {
	return []byte(t.Text()), nil
}

// MapType represents a key-value container type (map<K,V>).
type MapType struct {
	Key   string         // Key type (int or string only)
	Value TypeDefinition // Value type
}

func (t MapType) Text() string {
	return "map<" + t.Key + ", " + t.Value.Text() + ">"
}

// ListType represents a list container type (list<T>).
type ListType struct {
	Item TypeDefinition // Element type
}

func (t ListType) Text() string {
	return "list<" + t.Item.Text() + ">"
}

// RPC represents an HTTP-based remote procedure call definition.
type RPC struct {
	SSE bool // True if this RPC uses Server-Sent Events

	Name        string         // Name of the RPC
	Request     string         // Request type (user-defined types only)
	Response    TypeDefinition // Response type
	Annotations []Annotation   // Attached annotations
	Position    Position       // Location in the source file
	Comments    Comments       // Associated comments

	Path        string // HTTP request path
	Method      string // HTTP method (GET, POST, etc.)
	ContentType string // HTTP Content-Type

	ConnTimeout  int // Connection timeout in milliseconds
	ReadTimeout  int // Read timeout in milliseconds
	WriteTimeout int // Write timeout in milliseconds

	PathSegments []pathidl.Segment // Parsed path segments
	PathParams   map[string]string // Mapping: path variable → request field name
}
