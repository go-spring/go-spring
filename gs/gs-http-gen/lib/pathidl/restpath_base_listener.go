// Code generated from RestPath.g4 by ANTLR 4.13.2. DO NOT EDIT.

package pathidl // RestPath
import "github.com/antlr4-go/antlr/v4"

// BaseRestPathListener is a complete listener for a parse tree produced by RestPathParser.
type BaseRestPathListener struct{}

var _ RestPathListener = &BaseRestPathListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseRestPathListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseRestPathListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseRestPathListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseRestPathListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterPath is called when production path is entered.
func (s *BaseRestPathListener) EnterPath(ctx *PathContext) {}

// ExitPath is called when production path is exited.
func (s *BaseRestPathListener) ExitPath(ctx *PathContext) {}

// EnterSegment is called when production segment is entered.
func (s *BaseRestPathListener) EnterSegment(ctx *SegmentContext) {}

// ExitSegment is called when production segment is exited.
func (s *BaseRestPathListener) ExitSegment(ctx *SegmentContext) {}

// EnterParamSegment is called when production paramSegment is entered.
func (s *BaseRestPathListener) EnterParamSegment(ctx *ParamSegmentContext) {}

// ExitParamSegment is called when production paramSegment is exited.
func (s *BaseRestPathListener) ExitParamSegment(ctx *ParamSegmentContext) {}

// EnterBracedParam is called when production bracedParam is entered.
func (s *BaseRestPathListener) EnterBracedParam(ctx *BracedParamContext) {}

// ExitBracedParam is called when production bracedParam is exited.
func (s *BaseRestPathListener) ExitBracedParam(ctx *BracedParamContext) {}
