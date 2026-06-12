// Code generated from VParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package validate // VParser
import "github.com/antlr4-go/antlr/v4"

// BaseVParserListener is a complete listener for a parse tree produced by VParser.
type BaseVParserListener struct{}

var _ VParserListener = &BaseVParserListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseVParserListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseVParserListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseVParserListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseVParserListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterValidateExpr is called when production validateExpr is entered.
func (s *BaseVParserListener) EnterValidateExpr(ctx *ValidateExprContext) {}

// ExitValidateExpr is called when production validateExpr is exited.
func (s *BaseVParserListener) ExitValidateExpr(ctx *ValidateExprContext) {}

// EnterLogicalOrExpr is called when production logicalOrExpr is entered.
func (s *BaseVParserListener) EnterLogicalOrExpr(ctx *LogicalOrExprContext) {}

// ExitLogicalOrExpr is called when production logicalOrExpr is exited.
func (s *BaseVParserListener) ExitLogicalOrExpr(ctx *LogicalOrExprContext) {}

// EnterLogicalAndExpr is called when production logicalAndExpr is entered.
func (s *BaseVParserListener) EnterLogicalAndExpr(ctx *LogicalAndExprContext) {}

// ExitLogicalAndExpr is called when production logicalAndExpr is exited.
func (s *BaseVParserListener) ExitLogicalAndExpr(ctx *LogicalAndExprContext) {}

// EnterEqualityExpr is called when production equalityExpr is entered.
func (s *BaseVParserListener) EnterEqualityExpr(ctx *EqualityExprContext) {}

// ExitEqualityExpr is called when production equalityExpr is exited.
func (s *BaseVParserListener) ExitEqualityExpr(ctx *EqualityExprContext) {}

// EnterRelationalExpr is called when production relationalExpr is entered.
func (s *BaseVParserListener) EnterRelationalExpr(ctx *RelationalExprContext) {}

// ExitRelationalExpr is called when production relationalExpr is exited.
func (s *BaseVParserListener) ExitRelationalExpr(ctx *RelationalExprContext) {}

// EnterUnaryExpr is called when production unaryExpr is entered.
func (s *BaseVParserListener) EnterUnaryExpr(ctx *UnaryExprContext) {}

// ExitUnaryExpr is called when production unaryExpr is exited.
func (s *BaseVParserListener) ExitUnaryExpr(ctx *UnaryExprContext) {}

// EnterPrimaryExpr is called when production primaryExpr is entered.
func (s *BaseVParserListener) EnterPrimaryExpr(ctx *PrimaryExprContext) {}

// ExitPrimaryExpr is called when production primaryExpr is exited.
func (s *BaseVParserListener) ExitPrimaryExpr(ctx *PrimaryExprContext) {}

// EnterFunctionCall is called when production functionCall is entered.
func (s *BaseVParserListener) EnterFunctionCall(ctx *FunctionCallContext) {}

// ExitFunctionCall is called when production functionCall is exited.
func (s *BaseVParserListener) ExitFunctionCall(ctx *FunctionCallContext) {}
