// Code generated from RestPath.g4 by ANTLR 4.13.2. DO NOT EDIT.

package pathidl

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

type RestPathLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var RestPathLexerLexerStaticData struct {
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

func restpathlexerLexerInit() {
	staticData := &RestPathLexerLexerStaticData
	staticData.ChannelNames = []string{
		"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
	}
	staticData.ModeNames = []string{
		"DEFAULT_MODE",
	}
	staticData.LiteralNames = []string{
		"", "'/'", "':'", "'*'", "'{'", "'...'", "'}'",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "STATIC_SEGMENT",
	}
	staticData.RuleNames = []string{
		"T__0", "T__1", "T__2", "T__3", "T__4", "T__5", "STATIC_SEGMENT",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 0, 7, 34, 6, -1, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2,
		4, 7, 4, 2, 5, 7, 5, 2, 6, 7, 6, 1, 0, 1, 0, 1, 1, 1, 1, 1, 2, 1, 2, 1,
		3, 1, 3, 1, 4, 1, 4, 1, 4, 1, 4, 1, 5, 1, 5, 1, 6, 4, 6, 31, 8, 6, 11,
		6, 12, 6, 32, 0, 0, 7, 1, 1, 3, 2, 5, 3, 7, 4, 9, 5, 11, 6, 13, 7, 1, 0,
		1, 5, 0, 45, 45, 48, 57, 65, 90, 95, 95, 97, 122, 34, 0, 1, 1, 0, 0, 0,
		0, 3, 1, 0, 0, 0, 0, 5, 1, 0, 0, 0, 0, 7, 1, 0, 0, 0, 0, 9, 1, 0, 0, 0,
		0, 11, 1, 0, 0, 0, 0, 13, 1, 0, 0, 0, 1, 15, 1, 0, 0, 0, 3, 17, 1, 0, 0,
		0, 5, 19, 1, 0, 0, 0, 7, 21, 1, 0, 0, 0, 9, 23, 1, 0, 0, 0, 11, 27, 1,
		0, 0, 0, 13, 30, 1, 0, 0, 0, 15, 16, 5, 47, 0, 0, 16, 2, 1, 0, 0, 0, 17,
		18, 5, 58, 0, 0, 18, 4, 1, 0, 0, 0, 19, 20, 5, 42, 0, 0, 20, 6, 1, 0, 0,
		0, 21, 22, 5, 123, 0, 0, 22, 8, 1, 0, 0, 0, 23, 24, 5, 46, 0, 0, 24, 25,
		5, 46, 0, 0, 25, 26, 5, 46, 0, 0, 26, 10, 1, 0, 0, 0, 27, 28, 5, 125, 0,
		0, 28, 12, 1, 0, 0, 0, 29, 31, 7, 0, 0, 0, 30, 29, 1, 0, 0, 0, 31, 32,
		1, 0, 0, 0, 32, 30, 1, 0, 0, 0, 32, 33, 1, 0, 0, 0, 33, 14, 1, 0, 0, 0,
		2, 0, 32, 0,
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

// RestPathLexerInit initializes any static state used to implement RestPathLexer. By default the
// static state used to implement the lexer is lazily initialized during the first call to
// NewRestPathLexer(). You can call this function if you wish to initialize the static state ahead
// of time.
func RestPathLexerInit() {
	staticData := &RestPathLexerLexerStaticData
	staticData.once.Do(restpathlexerLexerInit)
}

// NewRestPathLexer produces a new lexer instance for the optional input antlr.CharStream.
func NewRestPathLexer(input antlr.CharStream) *RestPathLexer {
	RestPathLexerInit()
	l := new(RestPathLexer)
	l.BaseLexer = antlr.NewBaseLexer(input)
	staticData := &RestPathLexerLexerStaticData
	l.Interpreter = antlr.NewLexerATNSimulator(l, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	l.channelNames = staticData.ChannelNames
	l.modeNames = staticData.ModeNames
	l.RuleNames = staticData.RuleNames
	l.LiteralNames = staticData.LiteralNames
	l.SymbolicNames = staticData.SymbolicNames
	l.GrammarFileName = "RestPath.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// RestPathLexer tokens.
const (
	RestPathLexerT__0           = 1
	RestPathLexerT__1           = 2
	RestPathLexerT__2           = 3
	RestPathLexerT__3           = 4
	RestPathLexerT__4           = 5
	RestPathLexerT__5           = 6
	RestPathLexerSTATIC_SEGMENT = 7
)
