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

package pathidl

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/go-spring/stdlib/errutil"
)

// SegmentStyle represents the style of path parameters
// (Colon :param :param* or Brace {param} {param...}).
type SegmentStyle int

const (
	Colon SegmentStyle = iota
	Brace
)

// SegmentType represents the type of a path segment.
type SegmentType int

const (
	Static   SegmentType = iota // fixed segment, e.g., "users"
	Param                       // named parameter, e.g., ":id" or "{id}"
	Wildcard                    // wildcard parameter, e.g., ":id*" or "{id...}"
)

// Segment represents a single path segment.
type Segment struct {
	Type  SegmentType
	Value string
}

// Format formats a slice of segments into a path string using the given style.
func Format(path []Segment, style SegmentStyle) string {
	var sb strings.Builder
	for _, s := range path {
		sb.WriteString("/")
		switch s.Type {
		case Static:
			sb.WriteString(s.Value)
		case Param:
			if style == Brace {
				sb.WriteString("{")
				sb.WriteString(s.Value)
				sb.WriteString("}")
			} else if style == Colon {
				sb.WriteString(":")
				sb.WriteString(s.Value)
			}
		case Wildcard:
			if style == Brace {
				sb.WriteString("{")
				sb.WriteString(s.Value)
				sb.WriteString("...}")
			} else if style == Colon {
				sb.WriteString(":")
				sb.WriteString(s.Value)
				sb.WriteString("*")
			}
		}
	}
	return sb.String()
}

// Parse parses a path string into a slice of Segment.
func Parse(data string) (path []Segment, err error) {
	if data = strings.TrimSpace(data); data == "" {
		return nil, errutil.Explain(nil, "empty path")
	}

	e := &ErrorListener{Data: data}

	// Recover from parser panics to provide better error reporting
	defer func() {
		if r := recover(); r != nil {
			path = nil
			err = errutil.Explain(nil, "[PANIC]: %v\n%s", r, debug.Stack())
			if e.Error != nil {
				err = errutil.Explain(nil, "%w\n%w", e.Error, err)
			}
		}
	}()

	// Step 1: Create lexer and token stream
	input := antlr.NewInputStream(data)
	lexer := NewRestPathLexer(input)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(e)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Step 2: Create parser and attach custom error listener
	p := NewRestPathParser(tokens)
	p.RemoveErrorListeners()
	p.AddErrorListener(e)
	tree := p.Path()

	// 检查最后是否有异常字符
	if ts := tokens.GetAllTokens(); len(ts) == 0 {
		e.Error = errutil.Explain(nil, "empty path")
		return nil, e.Error
	} else {
		if c := ts[len(ts)-1]; c.GetTokenType() != antlr.TokenEOF {
			e.Error = errutil.Explain(nil, "unexpected character at the end of path: %q", c.GetText())
			return nil, e.Error
		}
	}

	// Step 3: Walk parse tree with custom listener
	l := &ParseTreeListener{Tokens: tokens}
	antlr.ParseTreeWalkerDefault.Walk(l, tree)

	// Step 4: Return parsed expression or error
	if e.Error != nil {
		return nil, e.Error
	}
	return l.Path, nil
}

// ErrorListener implements a custom ANTLR error listener that records syntax errors.
type ErrorListener struct {
	*antlr.DefaultErrorListener
	Error error
	Data  string
}

// SyntaxError is called by ANTLR when a syntax error occurs.
func (l *ErrorListener) SyntaxError(_ antlr.Recognizer, _ any, line, column int, msg string, e antlr.RecognitionException) {
	if l.Error == nil {
		l.Error = errutil.Explain(nil, "line %d:%d %s << text: %q", line, column, msg, l.Data)
		return
	}
	l.Error = errutil.Explain(nil, "%w\nline %d:%d %s << text: %q", l.Error, line, column, msg, l.Data)
}

// ParseTreeListener walks the parse tree and constructs the slice of Segment.
type ParseTreeListener struct {
	BaseRestPathListener
	Tokens *antlr.CommonTokenStream
	Path   []Segment
}

// ExitPath is called when exiting the Path rule
func (l *ParseTreeListener) ExitPath(ctx *PathContext) {
	for _, s := range ctx.AllSegment() {
		switch {
		case s.STATIC_SEGMENT() != nil:
			val := s.STATIC_SEGMENT().GetText()
			l.Path = append(l.Path, Segment{Static, val})

		case s.ParamSegment() != nil:
			val := s.ParamSegment().GetName().GetText()
			if c := val[0]; c >= '0' && c <= '9' {
				panic(fmt.Sprintf("invalid path parameter name: %q", val))
			}
			if s.ParamSegment().GetWildcard() != nil {
				l.Path = append(l.Path, Segment{Wildcard, val})
			} else {
				l.Path = append(l.Path, Segment{Param, val})
			}

		case s.BracedParam() != nil:
			val := s.BracedParam().GetName().GetText()
			if c := val[0]; c >= '0' && c <= '9' {
				panic(fmt.Sprintf("invalid path parameter name: %q", val))
			}
			if s.BracedParam().GetWildcard() != nil {
				l.Path = append(l.Path, Segment{Wildcard, val})
			} else {
				l.Path = append(l.Path, Segment{Param, val})
			}
		}
	}
}
