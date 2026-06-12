// Code generated from Expr.g4 by ANTLR 4.13.2. DO NOT EDIT.

package expr // Expr
import "github.com/antlr4-go/antlr/v4"

// BaseExprListener is a complete listener for a parse tree produced by ExprParser.
type BaseExprListener struct{}

var _ ExprListener = &BaseExprListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseExprListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseExprListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseExprListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseExprListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterRoot is called when production root is entered.
func (s *BaseExprListener) EnterRoot(ctx *RootContext) {}

// ExitRoot is called when production root is exited.
func (s *BaseExprListener) ExitRoot(ctx *RootContext) {}

// EnterExpr is called when production expr is entered.
func (s *BaseExprListener) EnterExpr(ctx *ExprContext) {}

// ExitExpr is called when production expr is exited.
func (s *BaseExprListener) ExitExpr(ctx *ExprContext) {}

// EnterInnerExprList is called when production innerExprList is entered.
func (s *BaseExprListener) EnterInnerExprList(ctx *InnerExprListContext) {}

// ExitInnerExprList is called when production innerExprList is exited.
func (s *BaseExprListener) ExitInnerExprList(ctx *InnerExprListContext) {}

// EnterInnerExpr is called when production innerExpr is entered.
func (s *BaseExprListener) EnterInnerExpr(ctx *InnerExprContext) {}

// ExitInnerExpr is called when production innerExpr is exited.
func (s *BaseExprListener) ExitInnerExpr(ctx *InnerExprContext) {}

// EnterFieldAccess is called when production fieldAccess is entered.
func (s *BaseExprListener) EnterFieldAccess(ctx *FieldAccessContext) {}

// ExitFieldAccess is called when production fieldAccess is exited.
func (s *BaseExprListener) ExitFieldAccess(ctx *FieldAccessContext) {}

// EnterValue is called when production value is entered.
func (s *BaseExprListener) EnterValue(ctx *ValueContext) {}

// ExitValue is called when production value is exited.
func (s *BaseExprListener) ExitValue(ctx *ValueContext) {}
