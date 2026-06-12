// Code generated from RestPath.g4 by ANTLR 4.13.2. DO NOT EDIT.

package pathidl // RestPath
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

type RestPathParser struct {
	*antlr.BaseParser
}

var RestPathParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func restpathParserInit() {
	staticData := &RestPathParserStaticData
	staticData.LiteralNames = []string{
		"", "'/'", "':'", "'*'", "'{'", "'...'", "'}'",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "STATIC_SEGMENT",
	}
	staticData.RuleNames = []string{
		"path", "segment", "paramSegment", "bracedParam",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 7, 35, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 1, 0, 1, 0,
		1, 0, 1, 0, 5, 0, 13, 8, 0, 10, 0, 12, 0, 16, 9, 0, 1, 1, 1, 1, 1, 1, 3,
		1, 21, 8, 1, 1, 2, 1, 2, 1, 2, 3, 2, 26, 8, 2, 1, 3, 1, 3, 1, 3, 3, 3,
		31, 8, 3, 1, 3, 1, 3, 1, 3, 0, 0, 4, 0, 2, 4, 6, 0, 0, 35, 0, 8, 1, 0,
		0, 0, 2, 20, 1, 0, 0, 0, 4, 22, 1, 0, 0, 0, 6, 27, 1, 0, 0, 0, 8, 9, 5,
		1, 0, 0, 9, 14, 3, 2, 1, 0, 10, 11, 5, 1, 0, 0, 11, 13, 3, 2, 1, 0, 12,
		10, 1, 0, 0, 0, 13, 16, 1, 0, 0, 0, 14, 12, 1, 0, 0, 0, 14, 15, 1, 0, 0,
		0, 15, 1, 1, 0, 0, 0, 16, 14, 1, 0, 0, 0, 17, 21, 5, 7, 0, 0, 18, 21, 3,
		4, 2, 0, 19, 21, 3, 6, 3, 0, 20, 17, 1, 0, 0, 0, 20, 18, 1, 0, 0, 0, 20,
		19, 1, 0, 0, 0, 21, 3, 1, 0, 0, 0, 22, 23, 5, 2, 0, 0, 23, 25, 5, 7, 0,
		0, 24, 26, 5, 3, 0, 0, 25, 24, 1, 0, 0, 0, 25, 26, 1, 0, 0, 0, 26, 5, 1,
		0, 0, 0, 27, 28, 5, 4, 0, 0, 28, 30, 5, 7, 0, 0, 29, 31, 5, 5, 0, 0, 30,
		29, 1, 0, 0, 0, 30, 31, 1, 0, 0, 0, 31, 32, 1, 0, 0, 0, 32, 33, 5, 6, 0,
		0, 33, 7, 1, 0, 0, 0, 4, 14, 20, 25, 30,
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

// RestPathParserInit initializes any static state used to implement RestPathParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewRestPathParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func RestPathParserInit() {
	staticData := &RestPathParserStaticData
	staticData.once.Do(restpathParserInit)
}

// NewRestPathParser produces a new parser instance for the optional input antlr.TokenStream.
func NewRestPathParser(input antlr.TokenStream) *RestPathParser {
	RestPathParserInit()
	this := new(RestPathParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &RestPathParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "RestPath.g4"

	return this
}

// RestPathParser tokens.
const (
	RestPathParserEOF            = antlr.TokenEOF
	RestPathParserT__0           = 1
	RestPathParserT__1           = 2
	RestPathParserT__2           = 3
	RestPathParserT__3           = 4
	RestPathParserT__4           = 5
	RestPathParserT__5           = 6
	RestPathParserSTATIC_SEGMENT = 7
)

// RestPathParser rules.
const (
	RestPathParserRULE_path         = 0
	RestPathParserRULE_segment      = 1
	RestPathParserRULE_paramSegment = 2
	RestPathParserRULE_bracedParam  = 3
)

// IPathContext is an interface to support dynamic dispatch.
type IPathContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllSegment() []ISegmentContext
	Segment(i int) ISegmentContext

	// IsPathContext differentiates from other interfaces.
	IsPathContext()
}

type PathContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyPathContext() *PathContext {
	var p = new(PathContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_path
	return p
}

func InitEmptyPathContext(p *PathContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_path
}

func (*PathContext) IsPathContext() {}

func NewPathContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PathContext {
	var p = new(PathContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = RestPathParserRULE_path

	return p
}

func (s *PathContext) GetParser() antlr.Parser { return s.parser }

func (s *PathContext) AllSegment() []ISegmentContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ISegmentContext); ok {
			len++
		}
	}

	tst := make([]ISegmentContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ISegmentContext); ok {
			tst[i] = t.(ISegmentContext)
			i++
		}
	}

	return tst
}

func (s *PathContext) Segment(i int) ISegmentContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISegmentContext); ok {
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

	return t.(ISegmentContext)
}

func (s *PathContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PathContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *PathContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.EnterPath(s)
	}
}

func (s *PathContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.ExitPath(s)
	}
}

func (p *RestPathParser) Path() (localctx IPathContext) {
	localctx = NewPathContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, RestPathParserRULE_path)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(8)
		p.Match(RestPathParserT__0)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(9)
		p.Segment()
	}
	p.SetState(14)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == RestPathParserT__0 {
		{
			p.SetState(10)
			p.Match(RestPathParserT__0)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(11)
			p.Segment()
		}

		p.SetState(16)
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

// ISegmentContext is an interface to support dynamic dispatch.
type ISegmentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	STATIC_SEGMENT() antlr.TerminalNode
	ParamSegment() IParamSegmentContext
	BracedParam() IBracedParamContext

	// IsSegmentContext differentiates from other interfaces.
	IsSegmentContext()
}

type SegmentContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySegmentContext() *SegmentContext {
	var p = new(SegmentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_segment
	return p
}

func InitEmptySegmentContext(p *SegmentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_segment
}

func (*SegmentContext) IsSegmentContext() {}

func NewSegmentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SegmentContext {
	var p = new(SegmentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = RestPathParserRULE_segment

	return p
}

func (s *SegmentContext) GetParser() antlr.Parser { return s.parser }

func (s *SegmentContext) STATIC_SEGMENT() antlr.TerminalNode {
	return s.GetToken(RestPathParserSTATIC_SEGMENT, 0)
}

func (s *SegmentContext) ParamSegment() IParamSegmentContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IParamSegmentContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IParamSegmentContext)
}

func (s *SegmentContext) BracedParam() IBracedParamContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBracedParamContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBracedParamContext)
}

func (s *SegmentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SegmentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SegmentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.EnterSegment(s)
	}
}

func (s *SegmentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.ExitSegment(s)
	}
}

func (p *RestPathParser) Segment() (localctx ISegmentContext) {
	localctx = NewSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, RestPathParserRULE_segment)
	p.SetState(20)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case RestPathParserSTATIC_SEGMENT:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(17)
			p.Match(RestPathParserSTATIC_SEGMENT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case RestPathParserT__1:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(18)
			p.ParamSegment()
		}

	case RestPathParserT__3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(19)
			p.BracedParam()
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

// IParamSegmentContext is an interface to support dynamic dispatch.
type IParamSegmentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetName returns the name token.
	GetName() antlr.Token

	// GetWildcard returns the wildcard token.
	GetWildcard() antlr.Token

	// SetName sets the name token.
	SetName(antlr.Token)

	// SetWildcard sets the wildcard token.
	SetWildcard(antlr.Token)

	// Getter signatures
	STATIC_SEGMENT() antlr.TerminalNode

	// IsParamSegmentContext differentiates from other interfaces.
	IsParamSegmentContext()
}

type ParamSegmentContext struct {
	antlr.BaseParserRuleContext
	parser   antlr.Parser
	name     antlr.Token
	wildcard antlr.Token
}

func NewEmptyParamSegmentContext() *ParamSegmentContext {
	var p = new(ParamSegmentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_paramSegment
	return p
}

func InitEmptyParamSegmentContext(p *ParamSegmentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_paramSegment
}

func (*ParamSegmentContext) IsParamSegmentContext() {}

func NewParamSegmentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ParamSegmentContext {
	var p = new(ParamSegmentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = RestPathParserRULE_paramSegment

	return p
}

func (s *ParamSegmentContext) GetParser() antlr.Parser { return s.parser }

func (s *ParamSegmentContext) GetName() antlr.Token { return s.name }

func (s *ParamSegmentContext) GetWildcard() antlr.Token { return s.wildcard }

func (s *ParamSegmentContext) SetName(v antlr.Token) { s.name = v }

func (s *ParamSegmentContext) SetWildcard(v antlr.Token) { s.wildcard = v }

func (s *ParamSegmentContext) STATIC_SEGMENT() antlr.TerminalNode {
	return s.GetToken(RestPathParserSTATIC_SEGMENT, 0)
}

func (s *ParamSegmentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParamSegmentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ParamSegmentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.EnterParamSegment(s)
	}
}

func (s *ParamSegmentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.ExitParamSegment(s)
	}
}

func (p *RestPathParser) ParamSegment() (localctx IParamSegmentContext) {
	localctx = NewParamSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, RestPathParserRULE_paramSegment)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(22)
		p.Match(RestPathParserT__1)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(23)

		var _m = p.Match(RestPathParserSTATIC_SEGMENT)

		localctx.(*ParamSegmentContext).name = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(25)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == RestPathParserT__2 {
		{
			p.SetState(24)

			var _m = p.Match(RestPathParserT__2)

			localctx.(*ParamSegmentContext).wildcard = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
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

// IBracedParamContext is an interface to support dynamic dispatch.
type IBracedParamContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetName returns the name token.
	GetName() antlr.Token

	// GetWildcard returns the wildcard token.
	GetWildcard() antlr.Token

	// SetName sets the name token.
	SetName(antlr.Token)

	// SetWildcard sets the wildcard token.
	SetWildcard(antlr.Token)

	// Getter signatures
	STATIC_SEGMENT() antlr.TerminalNode

	// IsBracedParamContext differentiates from other interfaces.
	IsBracedParamContext()
}

type BracedParamContext struct {
	antlr.BaseParserRuleContext
	parser   antlr.Parser
	name     antlr.Token
	wildcard antlr.Token
}

func NewEmptyBracedParamContext() *BracedParamContext {
	var p = new(BracedParamContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_bracedParam
	return p
}

func InitEmptyBracedParamContext(p *BracedParamContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = RestPathParserRULE_bracedParam
}

func (*BracedParamContext) IsBracedParamContext() {}

func NewBracedParamContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *BracedParamContext {
	var p = new(BracedParamContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = RestPathParserRULE_bracedParam

	return p
}

func (s *BracedParamContext) GetParser() antlr.Parser { return s.parser }

func (s *BracedParamContext) GetName() antlr.Token { return s.name }

func (s *BracedParamContext) GetWildcard() antlr.Token { return s.wildcard }

func (s *BracedParamContext) SetName(v antlr.Token) { s.name = v }

func (s *BracedParamContext) SetWildcard(v antlr.Token) { s.wildcard = v }

func (s *BracedParamContext) STATIC_SEGMENT() antlr.TerminalNode {
	return s.GetToken(RestPathParserSTATIC_SEGMENT, 0)
}

func (s *BracedParamContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BracedParamContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *BracedParamContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.EnterBracedParam(s)
	}
}

func (s *BracedParamContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RestPathListener); ok {
		listenerT.ExitBracedParam(s)
	}
}

func (p *RestPathParser) BracedParam() (localctx IBracedParamContext) {
	localctx = NewBracedParamContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, RestPathParserRULE_bracedParam)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(27)
		p.Match(RestPathParserT__3)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(28)

		var _m = p.Match(RestPathParserSTATIC_SEGMENT)

		localctx.(*BracedParamContext).name = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(30)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == RestPathParserT__4 {
		{
			p.SetState(29)

			var _m = p.Match(RestPathParserT__4)

			localctx.(*BracedParamContext).wildcard = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(32)
		p.Match(RestPathParserT__5)
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
