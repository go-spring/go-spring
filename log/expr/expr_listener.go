// Code generated from Expr.g4 by ANTLR 4.13.2. DO NOT EDIT.

package expr // Expr
import "github.com/antlr4-go/antlr/v4"

// ExprListener is a complete listener for a parse tree produced by ExprParser.
type ExprListener interface {
	antlr.ParseTreeListener

	// EnterRoot is called when entering the root production.
	EnterRoot(c *RootContext)

	// EnterExpr is called when entering the expr production.
	EnterExpr(c *ExprContext)

	// EnterInnerExprList is called when entering the innerExprList production.
	EnterInnerExprList(c *InnerExprListContext)

	// EnterInnerExpr is called when entering the innerExpr production.
	EnterInnerExpr(c *InnerExprContext)

	// EnterFieldAccess is called when entering the fieldAccess production.
	EnterFieldAccess(c *FieldAccessContext)

	// EnterValue is called when entering the value production.
	EnterValue(c *ValueContext)

	// ExitRoot is called when exiting the root production.
	ExitRoot(c *RootContext)

	// ExitExpr is called when exiting the expr production.
	ExitExpr(c *ExprContext)

	// ExitInnerExprList is called when exiting the innerExprList production.
	ExitInnerExprList(c *InnerExprListContext)

	// ExitInnerExpr is called when exiting the innerExpr production.
	ExitInnerExpr(c *InnerExprContext)

	// ExitFieldAccess is called when exiting the fieldAccess production.
	ExitFieldAccess(c *FieldAccessContext)

	// ExitValue is called when exiting the value production.
	ExitValue(c *ValueContext)
}
