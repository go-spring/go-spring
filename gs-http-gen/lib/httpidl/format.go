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
	"sort"
	"strconv"
	"strings"
)

const indent = "    "

// docItemKind represents the category of a top-level document element.
// It is used to determine rendering order and spacing during formatting.
type docItemKind int

const (
	// Single-line comment (e.g., // comment)
	docItemKindSLComment = docItemKind(iota)

	// Multi-line comment (e.g., /* ... */)
	docItemKindMLComment

	// Constant declaration
	docItemKindConst

	// Enum declaration
	docItemKindEnum

	// Type declaration (struct-like or oneof)
	docItemKindType

	// RPC declaration (rpc or sse)
	docItemKindRPC
)

// docItem represents one piece of a document (comment, const, type, etc.)
// with its source position and pre-rendered buffer content.
type docItem struct {
	kind docItemKind // Kind of the document item
	pos  int         // Starting line in the original source
	buf  string      // Rendered text of the document item
}

// Format converts a parsed Document AST back into a formatted string.
// It preserves the original order of items (using their source positions)
// and ensures consistent blank lines and comment placement.
func Format(doc Document) string {
	var items []docItem

	// Collect standalone comments
	for _, c := range doc.Comments {
		kind := docItemKindMLComment
		if c.Single {
			kind = docItemKindSLComment
		}
		items = append(items, docItem{
			kind: kind,
			pos:  c.Position.StartLine,
			buf:  strings.Join(c.Text, "\n"),
		})
	}

	// Collect constant declarations
	for _, c := range doc.Consts {
		items = append(items, docItem{
			kind: docItemKindConst,
			pos:  c.Position.StartLine,
			buf:  formatConst(c),
		})
	}

	// Collect enum declarations
	for _, e := range doc.Enums {
		if e.Kind == EnumKindOneOf {
			continue
		}
		items = append(items, docItem{
			kind: docItemKindEnum,
			pos:  e.Position.StartLine,
			buf:  formatEnum(e),
		})
	}

	// Collect type declarations
	for _, t := range doc.Types {
		items = append(items, docItem{
			kind: docItemKindType,
			pos:  t.Position.StartLine,
			buf:  formatType(t),
		})
	}

	// Collect RPC declarations
	for _, r := range doc.RPCs {
		items = append(items, docItem{
			kind: docItemKindRPC,
			pos:  r.Position.StartLine,
			buf:  formatRPC(r),
		})
	}

	// Sort items by their original starting line
	sort.Slice(items, func(i, j int) bool {
		return items[i].pos < items[j].pos
	})

	// Render items with proper spacing depending on their kind
	var sb strings.Builder
	lastKind := docItemKindSLComment
	for i, item := range items {
		switch lastKind {
		case docItemKindEnum, docItemKindType, docItemKindRPC:
			sb.WriteString("\n")
		default:
			// Insert a blank line when transitioning between different item kinds,
			// or before a multi-line comment.
			if i > 0 && (lastKind != item.kind || item.kind == docItemKindMLComment) {
				sb.WriteString("\n")
			}
		}
		sb.WriteString(item.buf)
		sb.WriteString("\n")
		lastKind = item.kind
	}
	return sb.String()
}

// formatAboveComments renders “above” comments associated with a declaration.
// Each line is prefixed with the given indentation prefix.
func formatAboveComments(comments []Comment, sb *strings.Builder, prefix string) {
	for _, c := range comments {

		if c.Single {
			// Single-line comment (`// ...`)
			sb.WriteString(prefix)
			sb.WriteString(c.Text[0])
			sb.WriteString("\n")
			continue
		}

		// Multi-line comment (`/* ... */`)
		for _, s := range c.Text {
			sb.WriteString(prefix)
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}
}

// formatRightComment renders a comment placed at the end of a line.
// For multi-line comments, subsequent lines are rendered with indentation.
func formatRightComment(c *Comment, sb *strings.Builder, prefix string) {
	if c == nil {
		return
	}

	if c.Single {
		// Inline single-line comment
		sb.WriteString(" ")
		sb.WriteString(c.Text[0])
		return
	}

	// Multi-line inline comment
	for i, s := range c.Text {
		if i == 0 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
			sb.WriteString(prefix)
		}
		sb.WriteString(s)
	}
}

// formatConst renders a constant declaration with its associated comments.
func formatConst(c Const) string {
	var sb strings.Builder
	formatAboveComments(c.Comments.Above, &sb, "")

	sb.WriteString("const ")
	sb.WriteString(c.Type.Name)
	sb.WriteString(" ")
	sb.WriteString(c.Name)
	sb.WriteString(" = ")
	sb.WriteString(c.Value)

	formatRightComment(c.Comments.Right, &sb, "")
	return sb.String()
}

// formatEnum renders an enum declaration and all of its fields.
func formatEnum(e Enum) string {
	var sb strings.Builder
	formatAboveComments(e.Comments.Above, &sb, "")

	sb.WriteString("enum ")
	if e.Kind == EnumKindExtends {
		sb.WriteString("extends ")
	}
	sb.WriteString(e.Name)
	sb.WriteString(" {")

	for _, f := range e.Fields {
		if f.ExtendsFrom != nil {
			continue
		}
		sb.WriteString("\n")
		formatAboveComments(f.Comments.Above, &sb, indent)

		sb.WriteString(indent)
		sb.WriteString(f.Name)
		sb.WriteString(" = ")
		sb.WriteString(strconv.FormatInt(f.Value, 10))

		formatFieldAnnotations(f.Annotations, &sb)
		formatRightComment(f.Comments.Right, &sb, indent)
	}

	sb.WriteString("\n}")
	return sb.String()
}

// formatType renders a type or oneof declaration, including its
// generic parameters, fields, and comments.
func formatType(t Type) string {
	var sb strings.Builder
	formatAboveComments(t.Comments.Above, &sb, "")

	if t.OneOf {
		sb.WriteString("oneof ")
	} else {
		sb.WriteString("type ")
	}

	sb.WriteString(t.Name)

	if t.InstType != nil {
		// Instantiated generic type
		sb.WriteString(" ")
		sb.WriteString(t.InstType.Text())
	} else {
		// Generic type definition
		if t.GenericParam != nil {
			sb.WriteString("<")
			sb.WriteString(*t.GenericParam)
			sb.WriteString(">")
		}

		sb.WriteString(" {")
		for i, f := range t.RawFields {
			if i == 0 && t.OneOf {
				continue
			}
			sb.WriteString("\n")
			formatTypeField(t, f, &sb)
		}
		sb.WriteString("\n}")
	}
	return sb.String()
}

// formatTypeField renders a single field of a type, including annotations,
// comments, and required/embedded modifiers.
func formatTypeField(t Type, f TypeField, sb *strings.Builder) {
	formatAboveComments(f.Comments.Above, sb, indent)

	sb.WriteString(indent)
	if f.Required {
		sb.WriteString("required ")
	}
	sb.WriteString(f.Type.Text())

	// Embedded fields have no explicit field name
	if _, ok := f.Type.(EmbedType); !ok && !t.OneOf {
		sb.WriteString(" ")
		sb.WriteString(f.Name)
	}

	formatFieldAnnotations(f.Annotations, sb)
	formatRightComment(f.Comments.Right, sb, indent)
}

// formatFieldAnnotations formats a field's annotations.
func formatFieldAnnotations(arr []Annotation, sb *strings.Builder) {
	if len(arr) > 0 {
		sb.WriteString(" (")
		for i, a := range arr {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(" ")
			sb.WriteString(a.Key)
			if a.Value != nil {
				sb.WriteString("=")
				sb.WriteString(*a.Value)
			}
		}
		sb.WriteString(" )")
	}
}

// formatRPC renders an RPC block, including request/response types
// and all associated annotations and comments.
func formatRPC(r RPC) string {
	var sb strings.Builder
	formatAboveComments(r.Comments.Above, &sb, "")

	if r.SSE {
		sb.WriteString("sse ")
	} else {
		sb.WriteString("rpc ")
	}

	sb.WriteString(r.Name)
	sb.WriteString("(")
	sb.WriteString(r.Request)
	sb.WriteString(") ")
	sb.WriteString(r.Response.Text())
	sb.WriteString(" {")

	for _, a := range r.Annotations {
		sb.WriteString("\n")
		formatAboveComments(a.Comments.Above, &sb, indent)

		sb.WriteString(indent)
		sb.WriteString(a.Key)
		if a.Value != nil {
			sb.WriteString("=")
			sb.WriteString(*a.Value)
		}

		formatRightComment(a.Comments.Right, &sb, indent)
	}

	sb.WriteString("\n}")
	return sb.String()
}
