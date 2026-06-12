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

package validate

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"go-spring.org/stdlib/errutil"
)

// Expr represents a generic expression node.
type Expr interface {
	Text() string
}

// BinaryExpr represents a binary expression (e.g., a && b, x == y).
type BinaryExpr struct {
	Left  Expr   // Left-hand side expression
	Op    string // Binary operator (e.g., &&, ||, ==, <)
	Right Expr   // Right-hand side expression
}

func (e BinaryExpr) Text() string {
	if e.Left == nil {
		return ""
	}
	if e.Right == nil {
		return e.Left.Text()
	}
	return fmt.Sprintf("%s %s %s", e.Left.Text(), e.Op, e.Right.Text())
}

// UnaryExpr represents a unary expression (e.g., !x).
type UnaryExpr struct {
	Op   string // Unary operator (e.g., !)
	Expr Expr   // Operand expression
}

func (e UnaryExpr) Text() string {
	if e.Expr == nil {
		return ""
	}
	return fmt.Sprintf("%s%s", e.Op, e.Expr.Text())
}

// PrimaryExpr represents an atomic expression:
// a literal, identifier, function call, or a parenthesized expression.
type PrimaryExpr struct {
	Value string     // Literal or identifier text
	Call  *FuncCall  // Optional function call
	Inner *InnerExpr // Optional parenthesized expression
}

func (e PrimaryExpr) Text() string {
	if e.Inner != nil {
		return e.Inner.Text()
	}
	if e.Call != nil {
		return e.Call.Text()
	}
	return e.Value
}

// FuncCall represents a function call expression with arguments.
type FuncCall struct {
	Name string // Function name
	Args []Expr // Function arguments
}

func (f FuncCall) Text() string {
	if len(f.Args) == 0 {
		return f.Name + "()"
	}
	var args []string
	for _, arg := range f.Args {
		args = append(args, arg.Text())
	}
	return fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
}

// InnerExpr represents a parenthesized subexpression, e.g., "(a && b)".
type InnerExpr struct {
	Expr Expr
}

func (e InnerExpr) Text() string {
	if e.Expr == nil {
		return ""
	}
	return fmt.Sprintf("(%s)", e.Expr.Text())
}

// Parse parses the input string and returns an Expr AST.
func Parse(data string) (expr Expr, err error) {
	if data = strings.TrimSpace(data); data == "" {
		return nil, nil
	}

	e := &ErrorListener{Data: data}

	// Recover from parser panics to provide better error reporting
	defer func() {
		if r := recover(); r != nil {
			expr = nil
			err = errutil.Explain(nil, "[PANIC]: %v\n%s", r, debug.Stack())
			if e.Error != nil {
				err = errutil.Explain(nil, "%w\n%w", e.Error, err)
			}
		}
	}()

	// Step 1: Create lexer and token stream
	input := antlr.NewInputStream(data)
	lexer := NewVLexer(input)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(e)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Step 2: Create parser and attach custom error listener
	p := NewVParser(tokens)
	p.RemoveErrorListeners()
	p.AddErrorListener(e)

	tree := p.ValidateExpr()
	if e.Error != nil {
		return nil, e.Error
	}
	if c := tokens.LT(1); c.GetTokenType() != antlr.TokenEOF {
		return nil, errutil.Explain(nil, "line %d:%d unexpected trailing token %q << text: %q", c.GetLine(), c.GetColumn(), c.GetText(), data)
	}

	// Step 3: Walk parse tree with custom listener
	l := &ParseTreeListener{Tokens: tokens}
	antlr.ParseTreeWalkerDefault.Walk(l, tree)

	// Step 4: Return parsed expression or error
	if e.Error != nil {
		return nil, e.Error
	}
	return l.Expr, nil
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

// ParseTreeListener walks the parse tree and constructs the expression AST.
type ParseTreeListener struct {
	BaseVParserListener
	Tokens *antlr.CommonTokenStream
	Expr   Expr
}

func (l *ParseTreeListener) ExitValidateExpr(ctx *ValidateExprContext) {
	l.Expr = parseValidateExpr(ctx)
}

// parseValidateExpr converts a ValidateExprContext into an Expr.
func parseValidateExpr(ctx IValidateExprContext) Expr {
	if ctx.LogicalOrExpr() == nil {
		return nil
	}
	return parseLogicalOrExpr(ctx.LogicalOrExpr())
}

// parseLogicalOrExpr handles logical OR expressions (e.g., a || b || c).
func parseLogicalOrExpr(ctx ILogicalOrExprContext) Expr {
	left := parseLogicalAndExpr(ctx.LogicalAndExpr(0))
	for i, o := range ctx.AllLOGICAL_OR() {
		left = BinaryExpr{
			Left:  left,
			Op:    o.GetText(),
			Right: parseLogicalAndExpr(ctx.LogicalAndExpr(i + 1)),
		}
	}
	return left
}

// parseLogicalAndExpr handles logical AND expressions (e.g., a && b && c).
func parseLogicalAndExpr(ctx ILogicalAndExprContext) Expr {
	left := parseEqualityExpr(ctx.EqualityExpr(0))
	for i, o := range ctx.AllLOGICAL_AND() {
		left = BinaryExpr{
			Left:  left,
			Op:    o.GetText(),
			Right: parseEqualityExpr(ctx.EqualityExpr(i + 1)),
		}
	}
	return left
}

// parseEqualityExpr handles equality and inequality comparisons (==, !=).
func parseEqualityExpr(ctx IEqualityExprContext) Expr {
	left := parseRelationalExpr(ctx.RelationalExpr(0))

	var op antlr.TerminalNode
	if ctx.EQUAL() != nil {
		op = ctx.EQUAL()
	} else if ctx.NOT_EQUAL() != nil {
		op = ctx.NOT_EQUAL()
	} else {
		return left
	}

	return BinaryExpr{
		Left:  left,
		Op:    op.GetText(),
		Right: parseRelationalExpr(ctx.RelationalExpr(1)),
	}
}

// parseRelationalExpr handles <, <=, >, >= expressions.
func parseRelationalExpr(ctx IRelationalExprContext) Expr {
	left := parseUnaryExpr(ctx.UnaryExpr(0))

	var op antlr.TerminalNode
	if ctx.LESS_THAN() != nil {
		op = ctx.LESS_THAN()
	} else if ctx.LESS_OR_EQUAL() != nil {
		op = ctx.LESS_OR_EQUAL()
	} else if ctx.GREATER_THAN() != nil {
		op = ctx.GREATER_THAN()
	} else if ctx.GREATER_OR_EQUAL() != nil {
		op = ctx.GREATER_OR_EQUAL()
	} else {
		return left
	}

	return BinaryExpr{
		Left:  left,
		Op:    op.GetText(),
		Right: parseUnaryExpr(ctx.UnaryExpr(1)),
	}
}

// parseUnaryExpr handles unary operators like !expr.
func parseUnaryExpr(ctx IUnaryExprContext) Expr {
	if ctx.LOGICAL_NOT() != nil {
		return UnaryExpr{
			Op:   ctx.LOGICAL_NOT().GetText(),
			Expr: parseUnaryExpr(ctx.UnaryExpr()),
		}
	}
	return parsePrimaryExpr(ctx.PrimaryExpr())
}

// parsePrimaryExpr handles literals, identifiers, function calls,
// and parenthesized expressions.
func parsePrimaryExpr(ctx IPrimaryExprContext) Expr {
	if ctx == nil {
		return nil
	}
	if ctx.IDENTIFIER() != nil {
		return PrimaryExpr{
			Value: ctx.IDENTIFIER().GetText(),
		}
	}
	if ctx.KW_DOLLAR() != nil {
		return PrimaryExpr{
			Value: ctx.KW_DOLLAR().GetText(),
		}
	}
	if ctx.KW_NIL() != nil {
		return PrimaryExpr{
			Value: ctx.KW_NIL().GetText(),
		}
	}
	if ctx.INTEGER() != nil {
		return PrimaryExpr{
			Value: ctx.INTEGER().GetText(),
		}
	}
	if ctx.FLOAT() != nil {
		return PrimaryExpr{
			Value: ctx.FLOAT().GetText(),
		}
	}
	if ctx.STRING() != nil {
		return PrimaryExpr{
			Value: ctx.STRING().GetText(),
		}
	}
	if ctx.FunctionCall() != nil {
		return PrimaryExpr{
			Call: parseFunctionCall(ctx.FunctionCall()),
		}
	}
	if ctx.LEFT_PAREN() != nil {
		return PrimaryExpr{
			Inner: &InnerExpr{
				Expr: parseValidateExpr(ctx.ValidateExpr()),
			},
		}
	}
	return nil
}

// parseFunctionCall converts a FunctionCallContext into a FuncCall AST node.
func parseFunctionCall(ctx IFunctionCallContext) *FuncCall {
	var args []Expr
	for _, arg := range ctx.AllValidateExpr() {
		args = append(args, parseValidateExpr(arg))
	}
	return &FuncCall{
		Name: ctx.IDENTIFIER().GetText(),
		Args: args,
	}
}
