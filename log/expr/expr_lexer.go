// Code generated from Expr.g4 by ANTLR 4.13.2. DO NOT EDIT.

package expr

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	"sync"
	"unicode"
)

// Suppress unused import error
var _ = fmt.Printf
var _ = sync.Once{}
var _ = unicode.IsLetter

type ExprLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var ExprLexerLexerStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	ChannelNames           []string
	ModeNames              []string
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func exprlexerLexerInit() {
	staticData := &ExprLexerLexerStaticData
	staticData.ChannelNames = []string{
		"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
	}
	staticData.ModeNames = []string{
		"DEFAULT_MODE",
	}
	staticData.LiteralNames = []string{
		"", "'{'", "'}'", "','", "'='", "'.'", "'['", "']'",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "", "IDENT", "STRING", "INTEGER", "FLOAT",
		"WS",
	}
	staticData.RuleNames = []string{
		"T__0", "T__1", "T__2", "T__3", "T__4", "T__5", "T__6", "IDENT", "STRING",
		"INTEGER", "FLOAT", "DIGIT", "LETTER", "HEX_DIGIT", "WS",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 0, 12, 131, 6, -1, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2,
		4, 7, 4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2,
		10, 7, 10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 1, 0,
		1, 0, 1, 1, 1, 1, 1, 2, 1, 2, 1, 3, 1, 3, 1, 4, 1, 4, 1, 5, 1, 5, 1, 6,
		1, 6, 1, 7, 1, 7, 5, 7, 48, 8, 7, 10, 7, 12, 7, 51, 9, 7, 1, 8, 1, 8, 1,
		8, 1, 8, 5, 8, 57, 8, 8, 10, 8, 12, 8, 60, 9, 8, 1, 8, 1, 8, 1, 9, 3, 9,
		65, 8, 9, 1, 9, 4, 9, 68, 8, 9, 11, 9, 12, 9, 69, 1, 9, 1, 9, 1, 9, 1,
		9, 4, 9, 76, 8, 9, 11, 9, 12, 9, 77, 3, 9, 80, 8, 9, 1, 10, 3, 10, 83,
		8, 10, 1, 10, 4, 10, 86, 8, 10, 11, 10, 12, 10, 87, 1, 10, 1, 10, 4, 10,
		92, 8, 10, 11, 10, 12, 10, 93, 3, 10, 96, 8, 10, 1, 10, 1, 10, 4, 10, 100,
		8, 10, 11, 10, 12, 10, 101, 3, 10, 104, 8, 10, 1, 10, 1, 10, 3, 10, 108,
		8, 10, 1, 10, 4, 10, 111, 8, 10, 11, 10, 12, 10, 112, 3, 10, 115, 8, 10,
		1, 11, 1, 11, 1, 12, 1, 12, 1, 13, 1, 13, 3, 13, 123, 8, 13, 1, 14, 4,
		14, 126, 8, 14, 11, 14, 12, 14, 127, 1, 14, 1, 14, 0, 0, 15, 1, 1, 3, 2,
		5, 3, 7, 4, 9, 5, 11, 6, 13, 7, 15, 8, 17, 9, 19, 10, 21, 11, 23, 0, 25,
		0, 27, 0, 29, 12, 1, 0, 9, 3, 0, 65, 90, 95, 95, 97, 122, 4, 0, 48, 57,
		65, 90, 95, 95, 97, 122, 2, 0, 34, 34, 92, 92, 8, 0, 34, 34, 47, 47, 92,
		92, 98, 98, 102, 102, 110, 110, 114, 114, 116, 116, 2, 0, 43, 43, 45, 45,
		2, 0, 69, 69, 101, 101, 2, 0, 65, 90, 97, 122, 2, 0, 65, 70, 97, 102, 3,
		0, 9, 10, 13, 13, 32, 32, 145, 0, 1, 1, 0, 0, 0, 0, 3, 1, 0, 0, 0, 0, 5,
		1, 0, 0, 0, 0, 7, 1, 0, 0, 0, 0, 9, 1, 0, 0, 0, 0, 11, 1, 0, 0, 0, 0, 13,
		1, 0, 0, 0, 0, 15, 1, 0, 0, 0, 0, 17, 1, 0, 0, 0, 0, 19, 1, 0, 0, 0, 0,
		21, 1, 0, 0, 0, 0, 29, 1, 0, 0, 0, 1, 31, 1, 0, 0, 0, 3, 33, 1, 0, 0, 0,
		5, 35, 1, 0, 0, 0, 7, 37, 1, 0, 0, 0, 9, 39, 1, 0, 0, 0, 11, 41, 1, 0,
		0, 0, 13, 43, 1, 0, 0, 0, 15, 45, 1, 0, 0, 0, 17, 52, 1, 0, 0, 0, 19, 79,
		1, 0, 0, 0, 21, 82, 1, 0, 0, 0, 23, 116, 1, 0, 0, 0, 25, 118, 1, 0, 0,
		0, 27, 122, 1, 0, 0, 0, 29, 125, 1, 0, 0, 0, 31, 32, 5, 123, 0, 0, 32,
		2, 1, 0, 0, 0, 33, 34, 5, 125, 0, 0, 34, 4, 1, 0, 0, 0, 35, 36, 5, 44,
		0, 0, 36, 6, 1, 0, 0, 0, 37, 38, 5, 61, 0, 0, 38, 8, 1, 0, 0, 0, 39, 40,
		5, 46, 0, 0, 40, 10, 1, 0, 0, 0, 41, 42, 5, 91, 0, 0, 42, 12, 1, 0, 0,
		0, 43, 44, 5, 93, 0, 0, 44, 14, 1, 0, 0, 0, 45, 49, 7, 0, 0, 0, 46, 48,
		7, 1, 0, 0, 47, 46, 1, 0, 0, 0, 48, 51, 1, 0, 0, 0, 49, 47, 1, 0, 0, 0,
		49, 50, 1, 0, 0, 0, 50, 16, 1, 0, 0, 0, 51, 49, 1, 0, 0, 0, 52, 58, 5,
		34, 0, 0, 53, 57, 8, 2, 0, 0, 54, 55, 5, 92, 0, 0, 55, 57, 7, 3, 0, 0,
		56, 53, 1, 0, 0, 0, 56, 54, 1, 0, 0, 0, 57, 60, 1, 0, 0, 0, 58, 56, 1,
		0, 0, 0, 58, 59, 1, 0, 0, 0, 59, 61, 1, 0, 0, 0, 60, 58, 1, 0, 0, 0, 61,
		62, 5, 34, 0, 0, 62, 18, 1, 0, 0, 0, 63, 65, 7, 4, 0, 0, 64, 63, 1, 0,
		0, 0, 64, 65, 1, 0, 0, 0, 65, 67, 1, 0, 0, 0, 66, 68, 3, 23, 11, 0, 67,
		66, 1, 0, 0, 0, 68, 69, 1, 0, 0, 0, 69, 67, 1, 0, 0, 0, 69, 70, 1, 0, 0,
		0, 70, 80, 1, 0, 0, 0, 71, 72, 5, 48, 0, 0, 72, 73, 5, 120, 0, 0, 73, 75,
		1, 0, 0, 0, 74, 76, 3, 27, 13, 0, 75, 74, 1, 0, 0, 0, 76, 77, 1, 0, 0,
		0, 77, 75, 1, 0, 0, 0, 77, 78, 1, 0, 0, 0, 78, 80, 1, 0, 0, 0, 79, 64,
		1, 0, 0, 0, 79, 71, 1, 0, 0, 0, 80, 20, 1, 0, 0, 0, 81, 83, 7, 4, 0, 0,
		82, 81, 1, 0, 0, 0, 82, 83, 1, 0, 0, 0, 83, 103, 1, 0, 0, 0, 84, 86, 3,
		23, 11, 0, 85, 84, 1, 0, 0, 0, 86, 87, 1, 0, 0, 0, 87, 85, 1, 0, 0, 0,
		87, 88, 1, 0, 0, 0, 88, 95, 1, 0, 0, 0, 89, 91, 5, 46, 0, 0, 90, 92, 3,
		23, 11, 0, 91, 90, 1, 0, 0, 0, 92, 93, 1, 0, 0, 0, 93, 91, 1, 0, 0, 0,
		93, 94, 1, 0, 0, 0, 94, 96, 1, 0, 0, 0, 95, 89, 1, 0, 0, 0, 95, 96, 1,
		0, 0, 0, 96, 104, 1, 0, 0, 0, 97, 99, 5, 46, 0, 0, 98, 100, 3, 23, 11,
		0, 99, 98, 1, 0, 0, 0, 100, 101, 1, 0, 0, 0, 101, 99, 1, 0, 0, 0, 101,
		102, 1, 0, 0, 0, 102, 104, 1, 0, 0, 0, 103, 85, 1, 0, 0, 0, 103, 97, 1,
		0, 0, 0, 104, 114, 1, 0, 0, 0, 105, 107, 7, 5, 0, 0, 106, 108, 7, 4, 0,
		0, 107, 106, 1, 0, 0, 0, 107, 108, 1, 0, 0, 0, 108, 110, 1, 0, 0, 0, 109,
		111, 3, 23, 11, 0, 110, 109, 1, 0, 0, 0, 111, 112, 1, 0, 0, 0, 112, 110,
		1, 0, 0, 0, 112, 113, 1, 0, 0, 0, 113, 115, 1, 0, 0, 0, 114, 105, 1, 0,
		0, 0, 114, 115, 1, 0, 0, 0, 115, 22, 1, 0, 0, 0, 116, 117, 2, 48, 57, 0,
		117, 24, 1, 0, 0, 0, 118, 119, 7, 6, 0, 0, 119, 26, 1, 0, 0, 0, 120, 123,
		3, 23, 11, 0, 121, 123, 7, 7, 0, 0, 122, 120, 1, 0, 0, 0, 122, 121, 1,
		0, 0, 0, 123, 28, 1, 0, 0, 0, 124, 126, 7, 8, 0, 0, 125, 124, 1, 0, 0,
		0, 126, 127, 1, 0, 0, 0, 127, 125, 1, 0, 0, 0, 127, 128, 1, 0, 0, 0, 128,
		129, 1, 0, 0, 0, 129, 130, 6, 14, 0, 0, 130, 30, 1, 0, 0, 0, 19, 0, 49,
		56, 58, 64, 69, 77, 79, 82, 87, 93, 95, 101, 103, 107, 112, 114, 122, 127,
		1, 6, 0, 0,
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

// ExprLexerInit initializes any static state used to implement ExprLexer. By default the
// static state used to implement the lexer is lazily initialized during the first call to
// NewExprLexer(). You can call this function if you wish to initialize the static state ahead
// of time.
func ExprLexerInit() {
	staticData := &ExprLexerLexerStaticData
	staticData.once.Do(exprlexerLexerInit)
}

// NewExprLexer produces a new lexer instance for the optional input antlr.CharStream.
func NewExprLexer(input antlr.CharStream) *ExprLexer {
	ExprLexerInit()
	l := new(ExprLexer)
	l.BaseLexer = antlr.NewBaseLexer(input)
	staticData := &ExprLexerLexerStaticData
	l.Interpreter = antlr.NewLexerATNSimulator(l, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	l.channelNames = staticData.ChannelNames
	l.modeNames = staticData.ModeNames
	l.RuleNames = staticData.RuleNames
	l.LiteralNames = staticData.LiteralNames
	l.SymbolicNames = staticData.SymbolicNames
	l.GrammarFileName = "Expr.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// ExprLexer tokens.
const (
	ExprLexerT__0    = 1
	ExprLexerT__1    = 2
	ExprLexerT__2    = 3
	ExprLexerT__3    = 4
	ExprLexerT__4    = 5
	ExprLexerT__5    = 6
	ExprLexerT__6    = 7
	ExprLexerIDENT   = 8
	ExprLexerSTRING  = 9
	ExprLexerINTEGER = 10
	ExprLexerFLOAT   = 11
	ExprLexerWS      = 12
)
