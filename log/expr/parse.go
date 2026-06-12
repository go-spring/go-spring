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

package expr

import (
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"go-spring.org/stdlib/errutil"
)

// Parse parses an expression string into a flat map representation.
//
// Example:
//
//	Input:  Logger { level = "info", path = /var/log/app.log }
//	Output: map[string]string{
//	           "type": "Logger",
//	           "level": "info",
//	           "path":  "/var/log/app.log",
//	        }
func Parse(data string) (ret map[string]string, err error) {
	if data = strings.TrimSpace(data); data == "" {
		return nil, nil
	}

	e := &ErrorListener{Data: data}

	// Recover from parser panics to provide better error reporting
	defer func() {
		if r := recover(); r != nil {
			ret = nil
			err = errutil.Explain(nil, "[PANIC]: %v\n%s", r, debug.Stack())
			if e.Error != nil {
				err = errutil.Explain(e.Error, "%s", err.Error())
			}
		}
	}()

	// Step 1: Create lexer and token stream
	input := antlr.NewInputStream(data)
	lexer := NewExprLexer(input)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(e)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Step 2: Create parser and attach custom error listener
	p := NewExprParser(tokens)
	p.RemoveErrorListeners()
	p.AddErrorListener(e)

	// Step 3: Walk parse tree with custom listener
	root := p.Root()
	if e.Error != nil {
		return nil, e.Error
	}
	l := &ParseTreeListener{
		Result: make(map[string]string),
	}
	antlr.ParseTreeWalkerDefault.Walk(l, root)

	// Step 4: Return the final result or error
	if e.Error != nil {
		return nil, e.Error
	}
	if l.Error != nil {
		return nil, l.Error
	}
	return l.Result, nil
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
		l.Error = errutil.Explain(nil, "line %d:%d %s >> text: %q", line, column, msg, l.Data)
		return
	}
	l.Error = errutil.Explain(l.Error, "line %d:%d %s >> text: %q", line, column, msg, l.Data)
}

// ParseTreeListener walks the parse tree and builds the key-value map.
type ParseTreeListener struct {
	BaseExprListener
	Result map[string]string
	Error  error
}

// ExitRoot is called when exiting the root node of the parse tree.
// It starts recursive parsing of the main expression.
func (l *ParseTreeListener) ExitRoot(ctx *RootContext) {
	l.parseExpr("", ctx.Expr())
}

// parseExpr processes a type expression block and traverses its inner expressions.
func (l *ParseTreeListener) parseExpr(key string, ctx IExprContext) {
	if l.Error != nil {
		return
	}
	typeKey := "type"
	if key != "" {
		typeKey = key + ".type"
	}
	l.setValue(typeKey, ctx.IDENT().GetText())
	if x := ctx.InnerExprList(); x != nil {
		for _, innerExpr := range x.AllInnerExpr() {
			l.parseInnerExpr(key, innerExpr)
		}
	}
}

// parseInnerExpr processes a single key-value assignment inside an expression block.
func (l *ParseTreeListener) parseInnerExpr(key string, ctx IInnerExprContext) {
	if l.Error != nil {
		return
	}
	fieldKey := ctx.FieldAccess().GetText()
	if key != "" {
		fieldKey = key + "." + fieldKey
	}
	switch {
	case ctx.Value().STRING() != nil:
		s, err := strconv.Unquote(ctx.Value().STRING().GetText())
		if err != nil {
			panic(err)
		}
		l.setValue(fieldKey, s)
	case ctx.Value().IDENT() != nil:
		l.setValue(fieldKey, ctx.Value().IDENT().GetText())
	case ctx.Value().INTEGER() != nil:
		l.setValue(fieldKey, ctx.Value().INTEGER().GetText())
	case ctx.Value().FLOAT() != nil:
		l.setValue(fieldKey, ctx.Value().FLOAT().GetText())
	case ctx.Value().Expr() != nil:
		l.parseExpr(fieldKey, ctx.Value().Expr())
	default: // for linter
	}
}

func (l *ParseTreeListener) setValue(key string, value string) {
	if _, ok := l.Result[key]; ok {
		l.Error = errutil.Explain(nil, "duplicate key %q", key)
		return
	}
	l.Result[key] = value
}
