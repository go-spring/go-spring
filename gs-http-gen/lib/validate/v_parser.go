// Code generated from VParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package validate // VParser
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr4-go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type VParser struct {
	*antlr.BaseParser
}

var VParserParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func vparserParserInit() {
	staticData := &VParserParserStaticData
	staticData.LiteralNames = []string{
		"", "'$'", "'nil'", "'=='", "'!='", "'<'", "'>'", "'<='", "'>='", "'&&'",
		"'||'", "'!'", "'('", "')'", "','",
	}
	staticData.SymbolicNames = []string{
		"", "KW_DOLLAR", "KW_NIL", "EQUAL", "NOT_EQUAL", "LESS_THAN", "GREATER_THAN",
		"LESS_OR_EQUAL", "GREATER_OR_EQUAL", "LOGICAL_AND", "LOGICAL_OR", "LOGICAL_NOT",
		"LEFT_PAREN", "RIGHT_PAREN", "COMMA", "STRING", "INTEGER", "FLOAT",
		"IDENTIFIER", "WHITESPACE",
	}
	staticData.RuleNames = []string{
		"validateExpr", "logicalOrExpr", "logicalAndExpr", "equalityExpr", "relationalExpr",
		"unaryExpr", "primaryExpr", "functionCall",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 19, 77, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 5,
		1, 22, 8, 1, 10, 1, 12, 1, 25, 9, 1, 1, 2, 1, 2, 1, 2, 5, 2, 30, 8, 2,
		10, 2, 12, 2, 33, 9, 2, 1, 3, 1, 3, 1, 3, 3, 3, 38, 8, 3, 1, 4, 1, 4, 1,
		4, 3, 4, 43, 8, 4, 1, 5, 1, 5, 1, 5, 3, 5, 48, 8, 5, 1, 6, 1, 6, 1, 6,
		1, 6, 1, 6, 1, 6, 1, 6, 1, 6, 1, 6, 1, 6, 1, 6, 3, 6, 61, 8, 6, 1, 7, 1,
		7, 1, 7, 1, 7, 1, 7, 5, 7, 68, 8, 7, 10, 7, 12, 7, 71, 9, 7, 3, 7, 73,
		8, 7, 1, 7, 1, 7, 1, 7, 0, 0, 8, 0, 2, 4, 6, 8, 10, 12, 14, 0, 2, 1, 0,
		3, 4, 1, 0, 5, 8, 82, 0, 16, 1, 0, 0, 0, 2, 18, 1, 0, 0, 0, 4, 26, 1, 0,
		0, 0, 6, 34, 1, 0, 0, 0, 8, 39, 1, 0, 0, 0, 10, 47, 1, 0, 0, 0, 12, 60,
		1, 0, 0, 0, 14, 62, 1, 0, 0, 0, 16, 17, 3, 2, 1, 0, 17, 1, 1, 0, 0, 0,
		18, 23, 3, 4, 2, 0, 19, 20, 5, 10, 0, 0, 20, 22, 3, 4, 2, 0, 21, 19, 1,
		0, 0, 0, 22, 25, 1, 0, 0, 0, 23, 21, 1, 0, 0, 0, 23, 24, 1, 0, 0, 0, 24,
		3, 1, 0, 0, 0, 25, 23, 1, 0, 0, 0, 26, 31, 3, 6, 3, 0, 27, 28, 5, 9, 0,
		0, 28, 30, 3, 6, 3, 0, 29, 27, 1, 0, 0, 0, 30, 33, 1, 0, 0, 0, 31, 29,
		1, 0, 0, 0, 31, 32, 1, 0, 0, 0, 32, 5, 1, 0, 0, 0, 33, 31, 1, 0, 0, 0,
		34, 37, 3, 8, 4, 0, 35, 36, 7, 0, 0, 0, 36, 38, 3, 8, 4, 0, 37, 35, 1,
		0, 0, 0, 37, 38, 1, 0, 0, 0, 38, 7, 1, 0, 0, 0, 39, 42, 3, 10, 5, 0, 40,
		41, 7, 1, 0, 0, 41, 43, 3, 10, 5, 0, 42, 40, 1, 0, 0, 0, 42, 43, 1, 0,
		0, 0, 43, 9, 1, 0, 0, 0, 44, 45, 5, 11, 0, 0, 45, 48, 3, 10, 5, 0, 46,
		48, 3, 12, 6, 0, 47, 44, 1, 0, 0, 0, 47, 46, 1, 0, 0, 0, 48, 11, 1, 0,
		0, 0, 49, 61, 5, 18, 0, 0, 50, 61, 5, 1, 0, 0, 51, 61, 5, 2, 0, 0, 52,
		61, 5, 16, 0, 0, 53, 61, 5, 17, 0, 0, 54, 61, 5, 15, 0, 0, 55, 61, 3, 14,
		7, 0, 56, 57, 5, 12, 0, 0, 57, 58, 3, 0, 0, 0, 58, 59, 5, 13, 0, 0, 59,
		61, 1, 0, 0, 0, 60, 49, 1, 0, 0, 0, 60, 50, 1, 0, 0, 0, 60, 51, 1, 0, 0,
		0, 60, 52, 1, 0, 0, 0, 60, 53, 1, 0, 0, 0, 60, 54, 1, 0, 0, 0, 60, 55,
		1, 0, 0, 0, 60, 56, 1, 0, 0, 0, 61, 13, 1, 0, 0, 0, 62, 63, 5, 18, 0, 0,
		63, 72, 5, 12, 0, 0, 64, 69, 3, 0, 0, 0, 65, 66, 5, 14, 0, 0, 66, 68, 3,
		0, 0, 0, 67, 65, 1, 0, 0, 0, 68, 71, 1, 0, 0, 0, 69, 67, 1, 0, 0, 0, 69,
		70, 1, 0, 0, 0, 70, 73, 1, 0, 0, 0, 71, 69, 1, 0, 0, 0, 72, 64, 1, 0, 0,
		0, 72, 73, 1, 0, 0, 0, 73, 74, 1, 0, 0, 0, 74, 75, 5, 13, 0, 0, 75, 15,
		1, 0, 0, 0, 8, 23, 31, 37, 42, 47, 60, 69, 72,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// VParserInit initializes any static state used to implement VParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewVParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func VParserInit() {
	staticData := &VParserParserStaticData
	staticData.once.Do(vparserParserInit)
}

// NewVParser produces a new parser instance for the optional input antlr.TokenStream.
func NewVParser(input antlr.TokenStream) *VParser {
	VParserInit()
	this := new(VParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &VParserParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "VParser.g4"

	return this
}

// VParser tokens.
const (
	VParserEOF              = antlr.TokenEOF
	VParserKW_DOLLAR        = 1
	VParserKW_NIL           = 2
	VParserEQUAL            = 3
	VParserNOT_EQUAL        = 4
	VParserLESS_THAN        = 5
	VParserGREATER_THAN     = 6
	VParserLESS_OR_EQUAL    = 7
	VParserGREATER_OR_EQUAL = 8
	VParserLOGICAL_AND      = 9
	VParserLOGICAL_OR       = 10
	VParserLOGICAL_NOT      = 11
	VParserLEFT_PAREN       = 12
	VParserRIGHT_PAREN      = 13
	VParserCOMMA            = 14
	VParserSTRING           = 15
	VParserINTEGER          = 16
	VParserFLOAT            = 17
	VParserIDENTIFIER       = 18
	VParserWHITESPACE       = 19
)

// VParser rules.
const (
	VParserRULE_validateExpr   = 0
	VParserRULE_logicalOrExpr  = 1
	VParserRULE_logicalAndExpr = 2
	VParserRULE_equalityExpr   = 3
	VParserRULE_relationalExpr = 4
	VParserRULE_unaryExpr      = 5
	VParserRULE_primaryExpr    = 6
	VParserRULE_functionCall   = 7
)

// IValidateExprContext is an interface to support dynamic dispatch.
type IValidateExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LogicalOrExpr() ILogicalOrExprContext

	// IsValidateExprContext differentiates from other interfaces.
	IsValidateExprContext()
}

type ValidateExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyValidateExprContext() *ValidateExprContext {
	var p = new(ValidateExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_validateExpr
	return p
}

func InitEmptyValidateExprContext(p *ValidateExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_validateExpr
}

func (*ValidateExprContext) IsValidateExprContext() {}

func NewValidateExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ValidateExprContext {
	var p = new(ValidateExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_validateExpr

	return p
}

func (s *ValidateExprContext) GetParser() antlr.Parser { return s.parser }

func (s *ValidateExprContext) LogicalOrExpr() ILogicalOrExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ILogicalOrExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ILogicalOrExprContext)
}

func (s *ValidateExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ValidateExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ValidateExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterValidateExpr(s)
	}
}

func (s *ValidateExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitValidateExpr(s)
	}
}

func (p *VParser) ValidateExpr() (localctx IValidateExprContext) {
	localctx = NewValidateExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, VParserRULE_validateExpr)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(16)
		p.LogicalOrExpr()
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ILogicalOrExprContext is an interface to support dynamic dispatch.
type ILogicalOrExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllLogicalAndExpr() []ILogicalAndExprContext
	LogicalAndExpr(i int) ILogicalAndExprContext
	AllLOGICAL_OR() []antlr.TerminalNode
	LOGICAL_OR(i int) antlr.TerminalNode

	// IsLogicalOrExprContext differentiates from other interfaces.
	IsLogicalOrExprContext()
}

type LogicalOrExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyLogicalOrExprContext() *LogicalOrExprContext {
	var p = new(LogicalOrExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_logicalOrExpr
	return p
}

func InitEmptyLogicalOrExprContext(p *LogicalOrExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_logicalOrExpr
}

func (*LogicalOrExprContext) IsLogicalOrExprContext() {}

func NewLogicalOrExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LogicalOrExprContext {
	var p = new(LogicalOrExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_logicalOrExpr

	return p
}

func (s *LogicalOrExprContext) GetParser() antlr.Parser { return s.parser }

func (s *LogicalOrExprContext) AllLogicalAndExpr() []ILogicalAndExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ILogicalAndExprContext); ok {
			len++
		}
	}

	tst := make([]ILogicalAndExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ILogicalAndExprContext); ok {
			tst[i] = t.(ILogicalAndExprContext)
			i++
		}
	}

	return tst
}

func (s *LogicalOrExprContext) LogicalAndExpr(i int) ILogicalAndExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ILogicalAndExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(ILogicalAndExprContext)
}

func (s *LogicalOrExprContext) AllLOGICAL_OR() []antlr.TerminalNode {
	return s.GetTokens(VParserLOGICAL_OR)
}

func (s *LogicalOrExprContext) LOGICAL_OR(i int) antlr.TerminalNode {
	return s.GetToken(VParserLOGICAL_OR, i)
}

func (s *LogicalOrExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LogicalOrExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *LogicalOrExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterLogicalOrExpr(s)
	}
}

func (s *LogicalOrExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitLogicalOrExpr(s)
	}
}

func (p *VParser) LogicalOrExpr() (localctx ILogicalOrExprContext) {
	localctx = NewLogicalOrExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, VParserRULE_logicalOrExpr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(18)
		p.LogicalAndExpr()
	}
	p.SetState(23)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == VParserLOGICAL_OR {
		{
			p.SetState(19)
			p.Match(VParserLOGICAL_OR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(20)
			p.LogicalAndExpr()
		}

		p.SetState(25)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ILogicalAndExprContext is an interface to support dynamic dispatch.
type ILogicalAndExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllEqualityExpr() []IEqualityExprContext
	EqualityExpr(i int) IEqualityExprContext
	AllLOGICAL_AND() []antlr.TerminalNode
	LOGICAL_AND(i int) antlr.TerminalNode

	// IsLogicalAndExprContext differentiates from other interfaces.
	IsLogicalAndExprContext()
}

type LogicalAndExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyLogicalAndExprContext() *LogicalAndExprContext {
	var p = new(LogicalAndExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_logicalAndExpr
	return p
}

func InitEmptyLogicalAndExprContext(p *LogicalAndExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_logicalAndExpr
}

func (*LogicalAndExprContext) IsLogicalAndExprContext() {}

func NewLogicalAndExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LogicalAndExprContext {
	var p = new(LogicalAndExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_logicalAndExpr

	return p
}

func (s *LogicalAndExprContext) GetParser() antlr.Parser { return s.parser }

func (s *LogicalAndExprContext) AllEqualityExpr() []IEqualityExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IEqualityExprContext); ok {
			len++
		}
	}

	tst := make([]IEqualityExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IEqualityExprContext); ok {
			tst[i] = t.(IEqualityExprContext)
			i++
		}
	}

	return tst
}

func (s *LogicalAndExprContext) EqualityExpr(i int) IEqualityExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEqualityExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEqualityExprContext)
}

func (s *LogicalAndExprContext) AllLOGICAL_AND() []antlr.TerminalNode {
	return s.GetTokens(VParserLOGICAL_AND)
}

func (s *LogicalAndExprContext) LOGICAL_AND(i int) antlr.TerminalNode {
	return s.GetToken(VParserLOGICAL_AND, i)
}

func (s *LogicalAndExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LogicalAndExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *LogicalAndExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterLogicalAndExpr(s)
	}
}

func (s *LogicalAndExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitLogicalAndExpr(s)
	}
}

func (p *VParser) LogicalAndExpr() (localctx ILogicalAndExprContext) {
	localctx = NewLogicalAndExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, VParserRULE_logicalAndExpr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(26)
		p.EqualityExpr()
	}
	p.SetState(31)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == VParserLOGICAL_AND {
		{
			p.SetState(27)
			p.Match(VParserLOGICAL_AND)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(28)
			p.EqualityExpr()
		}

		p.SetState(33)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IEqualityExprContext is an interface to support dynamic dispatch.
type IEqualityExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllRelationalExpr() []IRelationalExprContext
	RelationalExpr(i int) IRelationalExprContext
	EQUAL() antlr.TerminalNode
	NOT_EQUAL() antlr.TerminalNode

	// IsEqualityExprContext differentiates from other interfaces.
	IsEqualityExprContext()
}

type EqualityExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEqualityExprContext() *EqualityExprContext {
	var p = new(EqualityExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_equalityExpr
	return p
}

func InitEmptyEqualityExprContext(p *EqualityExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_equalityExpr
}

func (*EqualityExprContext) IsEqualityExprContext() {}

func NewEqualityExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EqualityExprContext {
	var p = new(EqualityExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_equalityExpr

	return p
}

func (s *EqualityExprContext) GetParser() antlr.Parser { return s.parser }

func (s *EqualityExprContext) AllRelationalExpr() []IRelationalExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IRelationalExprContext); ok {
			len++
		}
	}

	tst := make([]IRelationalExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IRelationalExprContext); ok {
			tst[i] = t.(IRelationalExprContext)
			i++
		}
	}

	return tst
}

func (s *EqualityExprContext) RelationalExpr(i int) IRelationalExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRelationalExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRelationalExprContext)
}

func (s *EqualityExprContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(VParserEQUAL, 0)
}

func (s *EqualityExprContext) NOT_EQUAL() antlr.TerminalNode {
	return s.GetToken(VParserNOT_EQUAL, 0)
}

func (s *EqualityExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EqualityExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EqualityExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterEqualityExpr(s)
	}
}

func (s *EqualityExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitEqualityExpr(s)
	}
}

func (p *VParser) EqualityExpr() (localctx IEqualityExprContext) {
	localctx = NewEqualityExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, VParserRULE_equalityExpr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(34)
		p.RelationalExpr()
	}
	p.SetState(37)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == VParserEQUAL || _la == VParserNOT_EQUAL {
		{
			p.SetState(35)
			_la = p.GetTokenStream().LA(1)

			if !(_la == VParserEQUAL || _la == VParserNOT_EQUAL) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(36)
			p.RelationalExpr()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IRelationalExprContext is an interface to support dynamic dispatch.
type IRelationalExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllUnaryExpr() []IUnaryExprContext
	UnaryExpr(i int) IUnaryExprContext
	LESS_THAN() antlr.TerminalNode
	LESS_OR_EQUAL() antlr.TerminalNode
	GREATER_THAN() antlr.TerminalNode
	GREATER_OR_EQUAL() antlr.TerminalNode

	// IsRelationalExprContext differentiates from other interfaces.
	IsRelationalExprContext()
}

type RelationalExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRelationalExprContext() *RelationalExprContext {
	var p = new(RelationalExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_relationalExpr
	return p
}

func InitEmptyRelationalExprContext(p *RelationalExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_relationalExpr
}

func (*RelationalExprContext) IsRelationalExprContext() {}

func NewRelationalExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *RelationalExprContext {
	var p = new(RelationalExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_relationalExpr

	return p
}

func (s *RelationalExprContext) GetParser() antlr.Parser { return s.parser }

func (s *RelationalExprContext) AllUnaryExpr() []IUnaryExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IUnaryExprContext); ok {
			len++
		}
	}

	tst := make([]IUnaryExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IUnaryExprContext); ok {
			tst[i] = t.(IUnaryExprContext)
			i++
		}
	}

	return tst
}

func (s *RelationalExprContext) UnaryExpr(i int) IUnaryExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUnaryExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUnaryExprContext)
}

func (s *RelationalExprContext) LESS_THAN() antlr.TerminalNode {
	return s.GetToken(VParserLESS_THAN, 0)
}

func (s *RelationalExprContext) LESS_OR_EQUAL() antlr.TerminalNode {
	return s.GetToken(VParserLESS_OR_EQUAL, 0)
}

func (s *RelationalExprContext) GREATER_THAN() antlr.TerminalNode {
	return s.GetToken(VParserGREATER_THAN, 0)
}

func (s *RelationalExprContext) GREATER_OR_EQUAL() antlr.TerminalNode {
	return s.GetToken(VParserGREATER_OR_EQUAL, 0)
}

func (s *RelationalExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *RelationalExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *RelationalExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterRelationalExpr(s)
	}
}

func (s *RelationalExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitRelationalExpr(s)
	}
}

func (p *VParser) RelationalExpr() (localctx IRelationalExprContext) {
	localctx = NewRelationalExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, VParserRULE_relationalExpr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(39)
		p.UnaryExpr()
	}
	p.SetState(42)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&480) != 0 {
		{
			p.SetState(40)
			_la = p.GetTokenStream().LA(1)

			if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&480) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(41)
			p.UnaryExpr()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IUnaryExprContext is an interface to support dynamic dispatch.
type IUnaryExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LOGICAL_NOT() antlr.TerminalNode
	UnaryExpr() IUnaryExprContext
	PrimaryExpr() IPrimaryExprContext

	// IsUnaryExprContext differentiates from other interfaces.
	IsUnaryExprContext()
}

type UnaryExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUnaryExprContext() *UnaryExprContext {
	var p = new(UnaryExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_unaryExpr
	return p
}

func InitEmptyUnaryExprContext(p *UnaryExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_unaryExpr
}

func (*UnaryExprContext) IsUnaryExprContext() {}

func NewUnaryExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UnaryExprContext {
	var p = new(UnaryExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_unaryExpr

	return p
}

func (s *UnaryExprContext) GetParser() antlr.Parser { return s.parser }

func (s *UnaryExprContext) LOGICAL_NOT() antlr.TerminalNode {
	return s.GetToken(VParserLOGICAL_NOT, 0)
}

func (s *UnaryExprContext) UnaryExpr() IUnaryExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUnaryExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUnaryExprContext)
}

func (s *UnaryExprContext) PrimaryExpr() IPrimaryExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IPrimaryExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IPrimaryExprContext)
}

func (s *UnaryExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UnaryExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *UnaryExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterUnaryExpr(s)
	}
}

func (s *UnaryExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitUnaryExpr(s)
	}
}

func (p *VParser) UnaryExpr() (localctx IUnaryExprContext) {
	localctx = NewUnaryExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, VParserRULE_unaryExpr)
	p.SetState(47)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case VParserLOGICAL_NOT:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(44)
			p.Match(VParserLOGICAL_NOT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(45)
			p.UnaryExpr()
		}

	case VParserKW_DOLLAR, VParserKW_NIL, VParserLEFT_PAREN, VParserSTRING, VParserINTEGER, VParserFLOAT, VParserIDENTIFIER:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(46)
			p.PrimaryExpr()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IPrimaryExprContext is an interface to support dynamic dispatch.
type IPrimaryExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	KW_DOLLAR() antlr.TerminalNode
	KW_NIL() antlr.TerminalNode
	INTEGER() antlr.TerminalNode
	FLOAT() antlr.TerminalNode
	STRING() antlr.TerminalNode
	FunctionCall() IFunctionCallContext
	LEFT_PAREN() antlr.TerminalNode
	ValidateExpr() IValidateExprContext
	RIGHT_PAREN() antlr.TerminalNode

	// IsPrimaryExprContext differentiates from other interfaces.
	IsPrimaryExprContext()
}

type PrimaryExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyPrimaryExprContext() *PrimaryExprContext {
	var p = new(PrimaryExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_primaryExpr
	return p
}

func InitEmptyPrimaryExprContext(p *PrimaryExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_primaryExpr
}

func (*PrimaryExprContext) IsPrimaryExprContext() {}

func NewPrimaryExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PrimaryExprContext {
	var p = new(PrimaryExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_primaryExpr

	return p
}

func (s *PrimaryExprContext) GetParser() antlr.Parser { return s.parser }

func (s *PrimaryExprContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(VParserIDENTIFIER, 0)
}

func (s *PrimaryExprContext) KW_DOLLAR() antlr.TerminalNode {
	return s.GetToken(VParserKW_DOLLAR, 0)
}

func (s *PrimaryExprContext) KW_NIL() antlr.TerminalNode {
	return s.GetToken(VParserKW_NIL, 0)
}

func (s *PrimaryExprContext) INTEGER() antlr.TerminalNode {
	return s.GetToken(VParserINTEGER, 0)
}

func (s *PrimaryExprContext) FLOAT() antlr.TerminalNode {
	return s.GetToken(VParserFLOAT, 0)
}

func (s *PrimaryExprContext) STRING() antlr.TerminalNode {
	return s.GetToken(VParserSTRING, 0)
}

func (s *PrimaryExprContext) FunctionCall() IFunctionCallContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFunctionCallContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFunctionCallContext)
}

func (s *PrimaryExprContext) LEFT_PAREN() antlr.TerminalNode {
	return s.GetToken(VParserLEFT_PAREN, 0)
}

func (s *PrimaryExprContext) ValidateExpr() IValidateExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValidateExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValidateExprContext)
}

func (s *PrimaryExprContext) RIGHT_PAREN() antlr.TerminalNode {
	return s.GetToken(VParserRIGHT_PAREN, 0)
}

func (s *PrimaryExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PrimaryExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *PrimaryExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterPrimaryExpr(s)
	}
}

func (s *PrimaryExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitPrimaryExpr(s)
	}
}

func (p *VParser) PrimaryExpr() (localctx IPrimaryExprContext) {
	localctx = NewPrimaryExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, VParserRULE_primaryExpr)
	p.SetState(60)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 5, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(49)
			p.Match(VParserIDENTIFIER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(50)
			p.Match(VParserKW_DOLLAR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(51)
			p.Match(VParserKW_NIL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 4:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(52)
			p.Match(VParserINTEGER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 5:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(53)
			p.Match(VParserFLOAT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 6:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(54)
			p.Match(VParserSTRING)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 7:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(55)
			p.FunctionCall()
		}

	case 8:
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(56)
			p.Match(VParserLEFT_PAREN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(57)
			p.ValidateExpr()
		}
		{
			p.SetState(58)
			p.Match(VParserRIGHT_PAREN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IFunctionCallContext is an interface to support dynamic dispatch.
type IFunctionCallContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	LEFT_PAREN() antlr.TerminalNode
	RIGHT_PAREN() antlr.TerminalNode
	AllValidateExpr() []IValidateExprContext
	ValidateExpr(i int) IValidateExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFunctionCallContext differentiates from other interfaces.
	IsFunctionCallContext()
}

type FunctionCallContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFunctionCallContext() *FunctionCallContext {
	var p = new(FunctionCallContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_functionCall
	return p
}

func InitEmptyFunctionCallContext(p *FunctionCallContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = VParserRULE_functionCall
}

func (*FunctionCallContext) IsFunctionCallContext() {}

func NewFunctionCallContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FunctionCallContext {
	var p = new(FunctionCallContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = VParserRULE_functionCall

	return p
}

func (s *FunctionCallContext) GetParser() antlr.Parser { return s.parser }

func (s *FunctionCallContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(VParserIDENTIFIER, 0)
}

func (s *FunctionCallContext) LEFT_PAREN() antlr.TerminalNode {
	return s.GetToken(VParserLEFT_PAREN, 0)
}

func (s *FunctionCallContext) RIGHT_PAREN() antlr.TerminalNode {
	return s.GetToken(VParserRIGHT_PAREN, 0)
}

func (s *FunctionCallContext) AllValidateExpr() []IValidateExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IValidateExprContext); ok {
			len++
		}
	}

	tst := make([]IValidateExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IValidateExprContext); ok {
			tst[i] = t.(IValidateExprContext)
			i++
		}
	}

	return tst
}

func (s *FunctionCallContext) ValidateExpr(i int) IValidateExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValidateExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValidateExprContext)
}

func (s *FunctionCallContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(VParserCOMMA)
}

func (s *FunctionCallContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(VParserCOMMA, i)
}

func (s *FunctionCallContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionCallContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FunctionCallContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.EnterFunctionCall(s)
	}
}

func (s *FunctionCallContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(VParserListener); ok {
		listenerT.ExitFunctionCall(s)
	}
}

func (p *VParser) FunctionCall() (localctx IFunctionCallContext) {
	localctx = NewFunctionCallContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, VParserRULE_functionCall)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(62)
		p.Match(VParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(63)
		p.Match(VParserLEFT_PAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(72)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&497670) != 0 {
		{
			p.SetState(64)
			p.ValidateExpr()
		}
		p.SetState(69)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == VParserCOMMA {
			{
				p.SetState(65)
				p.Match(VParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(66)
				p.ValidateExpr()
			}

			p.SetState(71)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}
	{
		p.SetState(74)
		p.Match(VParserRIGHT_PAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}
