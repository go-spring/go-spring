// Code generated from VLexer.g4 by ANTLR 4.13.2. DO NOT EDIT.

package validate

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

type VLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var VLexerLexerStaticData struct {
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

func vlexerLexerInit() {
	staticData := &VLexerLexerStaticData
	staticData.ChannelNames = []string{
		"DEFAULT_TOKEN_CHANNEL", "HIDDEN", "WS_CHAN",
	}
	staticData.ModeNames = []string{
		"DEFAULT_MODE",
	}
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
		"KW_DOLLAR", "KW_NIL", "EQUAL", "NOT_EQUAL", "LESS_THAN", "GREATER_THAN",
		"LESS_OR_EQUAL", "GREATER_OR_EQUAL", "LOGICAL_AND", "LOGICAL_OR", "LOGICAL_NOT",
		"LEFT_PAREN", "RIGHT_PAREN", "COMMA", "STRING", "INTEGER", "FLOAT",
		"IDENTIFIER", "DIGIT", "LETTER", "HEX_DIGIT", "WHITESPACE",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 0, 19, 172, 6, -1, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2,
		4, 7, 4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2,
		10, 7, 10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15,
		7, 15, 2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7,
		20, 2, 21, 7, 21, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 1, 2, 1, 2,
		1, 3, 1, 3, 1, 3, 1, 4, 1, 4, 1, 5, 1, 5, 1, 6, 1, 6, 1, 6, 1, 7, 1, 7,
		1, 7, 1, 8, 1, 8, 1, 8, 1, 9, 1, 9, 1, 9, 1, 10, 1, 10, 1, 11, 1, 11, 1,
		12, 1, 12, 1, 13, 1, 13, 1, 14, 1, 14, 1, 14, 1, 14, 5, 14, 86, 8, 14,
		10, 14, 12, 14, 89, 9, 14, 1, 14, 1, 14, 1, 15, 3, 15, 94, 8, 15, 1, 15,
		4, 15, 97, 8, 15, 11, 15, 12, 15, 98, 1, 15, 1, 15, 1, 15, 1, 15, 4, 15,
		105, 8, 15, 11, 15, 12, 15, 106, 3, 15, 109, 8, 15, 1, 16, 3, 16, 112,
		8, 16, 1, 16, 4, 16, 115, 8, 16, 11, 16, 12, 16, 116, 1, 16, 1, 16, 4,
		16, 121, 8, 16, 11, 16, 12, 16, 122, 3, 16, 125, 8, 16, 1, 16, 1, 16, 4,
		16, 129, 8, 16, 11, 16, 12, 16, 130, 3, 16, 133, 8, 16, 1, 16, 1, 16, 3,
		16, 137, 8, 16, 1, 16, 4, 16, 140, 8, 16, 11, 16, 12, 16, 141, 3, 16, 144,
		8, 16, 1, 17, 1, 17, 3, 17, 148, 8, 17, 1, 17, 1, 17, 1, 17, 5, 17, 153,
		8, 17, 10, 17, 12, 17, 156, 9, 17, 1, 18, 1, 18, 1, 19, 1, 19, 1, 20, 1,
		20, 3, 20, 164, 8, 20, 1, 21, 4, 21, 167, 8, 21, 11, 21, 12, 21, 168, 1,
		21, 1, 21, 0, 0, 22, 1, 1, 3, 2, 5, 3, 7, 4, 9, 5, 11, 6, 13, 7, 15, 8,
		17, 9, 19, 10, 21, 11, 23, 12, 25, 13, 27, 14, 29, 15, 31, 16, 33, 17,
		35, 18, 37, 0, 39, 0, 41, 0, 43, 19, 1, 0, 6, 2, 0, 39, 39, 92, 92, 2,
		0, 43, 43, 45, 45, 2, 0, 69, 69, 101, 101, 2, 0, 65, 90, 97, 122, 2, 0,
		65, 70, 97, 102, 3, 0, 9, 10, 13, 13, 32, 32, 189, 0, 1, 1, 0, 0, 0, 0,
		3, 1, 0, 0, 0, 0, 5, 1, 0, 0, 0, 0, 7, 1, 0, 0, 0, 0, 9, 1, 0, 0, 0, 0,
		11, 1, 0, 0, 0, 0, 13, 1, 0, 0, 0, 0, 15, 1, 0, 0, 0, 0, 17, 1, 0, 0, 0,
		0, 19, 1, 0, 0, 0, 0, 21, 1, 0, 0, 0, 0, 23, 1, 0, 0, 0, 0, 25, 1, 0, 0,
		0, 0, 27, 1, 0, 0, 0, 0, 29, 1, 0, 0, 0, 0, 31, 1, 0, 0, 0, 0, 33, 1, 0,
		0, 0, 0, 35, 1, 0, 0, 0, 0, 43, 1, 0, 0, 0, 1, 45, 1, 0, 0, 0, 3, 47, 1,
		0, 0, 0, 5, 51, 1, 0, 0, 0, 7, 54, 1, 0, 0, 0, 9, 57, 1, 0, 0, 0, 11, 59,
		1, 0, 0, 0, 13, 61, 1, 0, 0, 0, 15, 64, 1, 0, 0, 0, 17, 67, 1, 0, 0, 0,
		19, 70, 1, 0, 0, 0, 21, 73, 1, 0, 0, 0, 23, 75, 1, 0, 0, 0, 25, 77, 1,
		0, 0, 0, 27, 79, 1, 0, 0, 0, 29, 81, 1, 0, 0, 0, 31, 108, 1, 0, 0, 0, 33,
		111, 1, 0, 0, 0, 35, 147, 1, 0, 0, 0, 37, 157, 1, 0, 0, 0, 39, 159, 1,
		0, 0, 0, 41, 163, 1, 0, 0, 0, 43, 166, 1, 0, 0, 0, 45, 46, 5, 36, 0, 0,
		46, 2, 1, 0, 0, 0, 47, 48, 5, 110, 0, 0, 48, 49, 5, 105, 0, 0, 49, 50,
		5, 108, 0, 0, 50, 4, 1, 0, 0, 0, 51, 52, 5, 61, 0, 0, 52, 53, 5, 61, 0,
		0, 53, 6, 1, 0, 0, 0, 54, 55, 5, 33, 0, 0, 55, 56, 5, 61, 0, 0, 56, 8,
		1, 0, 0, 0, 57, 58, 5, 60, 0, 0, 58, 10, 1, 0, 0, 0, 59, 60, 5, 62, 0,
		0, 60, 12, 1, 0, 0, 0, 61, 62, 5, 60, 0, 0, 62, 63, 5, 61, 0, 0, 63, 14,
		1, 0, 0, 0, 64, 65, 5, 62, 0, 0, 65, 66, 5, 61, 0, 0, 66, 16, 1, 0, 0,
		0, 67, 68, 5, 38, 0, 0, 68, 69, 5, 38, 0, 0, 69, 18, 1, 0, 0, 0, 70, 71,
		5, 124, 0, 0, 71, 72, 5, 124, 0, 0, 72, 20, 1, 0, 0, 0, 73, 74, 5, 33,
		0, 0, 74, 22, 1, 0, 0, 0, 75, 76, 5, 40, 0, 0, 76, 24, 1, 0, 0, 0, 77,
		78, 5, 41, 0, 0, 78, 26, 1, 0, 0, 0, 79, 80, 5, 44, 0, 0, 80, 28, 1, 0,
		0, 0, 81, 87, 5, 39, 0, 0, 82, 83, 5, 92, 0, 0, 83, 86, 9, 0, 0, 0, 84,
		86, 8, 0, 0, 0, 85, 82, 1, 0, 0, 0, 85, 84, 1, 0, 0, 0, 86, 89, 1, 0, 0,
		0, 87, 85, 1, 0, 0, 0, 87, 88, 1, 0, 0, 0, 88, 90, 1, 0, 0, 0, 89, 87,
		1, 0, 0, 0, 90, 91, 5, 39, 0, 0, 91, 30, 1, 0, 0, 0, 92, 94, 7, 1, 0, 0,
		93, 92, 1, 0, 0, 0, 93, 94, 1, 0, 0, 0, 94, 96, 1, 0, 0, 0, 95, 97, 3,
		37, 18, 0, 96, 95, 1, 0, 0, 0, 97, 98, 1, 0, 0, 0, 98, 96, 1, 0, 0, 0,
		98, 99, 1, 0, 0, 0, 99, 109, 1, 0, 0, 0, 100, 101, 5, 48, 0, 0, 101, 102,
		5, 120, 0, 0, 102, 104, 1, 0, 0, 0, 103, 105, 3, 41, 20, 0, 104, 103, 1,
		0, 0, 0, 105, 106, 1, 0, 0, 0, 106, 104, 1, 0, 0, 0, 106, 107, 1, 0, 0,
		0, 107, 109, 1, 0, 0, 0, 108, 93, 1, 0, 0, 0, 108, 100, 1, 0, 0, 0, 109,
		32, 1, 0, 0, 0, 110, 112, 7, 1, 0, 0, 111, 110, 1, 0, 0, 0, 111, 112, 1,
		0, 0, 0, 112, 132, 1, 0, 0, 0, 113, 115, 3, 37, 18, 0, 114, 113, 1, 0,
		0, 0, 115, 116, 1, 0, 0, 0, 116, 114, 1, 0, 0, 0, 116, 117, 1, 0, 0, 0,
		117, 124, 1, 0, 0, 0, 118, 120, 5, 46, 0, 0, 119, 121, 3, 37, 18, 0, 120,
		119, 1, 0, 0, 0, 121, 122, 1, 0, 0, 0, 122, 120, 1, 0, 0, 0, 122, 123,
		1, 0, 0, 0, 123, 125, 1, 0, 0, 0, 124, 118, 1, 0, 0, 0, 124, 125, 1, 0,
		0, 0, 125, 133, 1, 0, 0, 0, 126, 128, 5, 46, 0, 0, 127, 129, 3, 37, 18,
		0, 128, 127, 1, 0, 0, 0, 129, 130, 1, 0, 0, 0, 130, 128, 1, 0, 0, 0, 130,
		131, 1, 0, 0, 0, 131, 133, 1, 0, 0, 0, 132, 114, 1, 0, 0, 0, 132, 126,
		1, 0, 0, 0, 133, 143, 1, 0, 0, 0, 134, 136, 7, 2, 0, 0, 135, 137, 7, 1,
		0, 0, 136, 135, 1, 0, 0, 0, 136, 137, 1, 0, 0, 0, 137, 139, 1, 0, 0, 0,
		138, 140, 3, 37, 18, 0, 139, 138, 1, 0, 0, 0, 140, 141, 1, 0, 0, 0, 141,
		139, 1, 0, 0, 0, 141, 142, 1, 0, 0, 0, 142, 144, 1, 0, 0, 0, 143, 134,
		1, 0, 0, 0, 143, 144, 1, 0, 0, 0, 144, 34, 1, 0, 0, 0, 145, 148, 3, 39,
		19, 0, 146, 148, 5, 95, 0, 0, 147, 145, 1, 0, 0, 0, 147, 146, 1, 0, 0,
		0, 148, 154, 1, 0, 0, 0, 149, 153, 3, 39, 19, 0, 150, 153, 3, 37, 18, 0,
		151, 153, 5, 95, 0, 0, 152, 149, 1, 0, 0, 0, 152, 150, 1, 0, 0, 0, 152,
		151, 1, 0, 0, 0, 153, 156, 1, 0, 0, 0, 154, 152, 1, 0, 0, 0, 154, 155,
		1, 0, 0, 0, 155, 36, 1, 0, 0, 0, 156, 154, 1, 0, 0, 0, 157, 158, 2, 48,
		57, 0, 158, 38, 1, 0, 0, 0, 159, 160, 7, 3, 0, 0, 160, 40, 1, 0, 0, 0,
		161, 164, 3, 37, 18, 0, 162, 164, 7, 4, 0, 0, 163, 161, 1, 0, 0, 0, 163,
		162, 1, 0, 0, 0, 164, 42, 1, 0, 0, 0, 165, 167, 7, 5, 0, 0, 166, 165, 1,
		0, 0, 0, 167, 168, 1, 0, 0, 0, 168, 166, 1, 0, 0, 0, 168, 169, 1, 0, 0,
		0, 169, 170, 1, 0, 0, 0, 170, 171, 6, 21, 0, 0, 171, 44, 1, 0, 0, 0, 21,
		0, 85, 87, 93, 98, 106, 108, 111, 116, 122, 124, 130, 132, 136, 141, 143,
		147, 152, 154, 163, 168, 1, 0, 2, 0,
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

// VLexerInit initializes any static state used to implement VLexer. By default the
// static state used to implement the lexer is lazily initialized during the first call to
// NewVLexer(). You can call this function if you wish to initialize the static state ahead
// of time.
func VLexerInit() {
	staticData := &VLexerLexerStaticData
	staticData.once.Do(vlexerLexerInit)
}

// NewVLexer produces a new lexer instance for the optional input antlr.CharStream.
func NewVLexer(input antlr.CharStream) *VLexer {
	VLexerInit()
	l := new(VLexer)
	l.BaseLexer = antlr.NewBaseLexer(input)
	staticData := &VLexerLexerStaticData
	l.Interpreter = antlr.NewLexerATNSimulator(l, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	l.channelNames = staticData.ChannelNames
	l.modeNames = staticData.ModeNames
	l.RuleNames = staticData.RuleNames
	l.LiteralNames = staticData.LiteralNames
	l.SymbolicNames = staticData.SymbolicNames
	l.GrammarFileName = "VLexer.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// VLexer tokens.
const (
	VLexerKW_DOLLAR        = 1
	VLexerKW_NIL           = 2
	VLexerEQUAL            = 3
	VLexerNOT_EQUAL        = 4
	VLexerLESS_THAN        = 5
	VLexerGREATER_THAN     = 6
	VLexerLESS_OR_EQUAL    = 7
	VLexerGREATER_OR_EQUAL = 8
	VLexerLOGICAL_AND      = 9
	VLexerLOGICAL_OR       = 10
	VLexerLOGICAL_NOT      = 11
	VLexerLEFT_PAREN       = 12
	VLexerRIGHT_PAREN      = 13
	VLexerCOMMA            = 14
	VLexerSTRING           = 15
	VLexerINTEGER          = 16
	VLexerFLOAT            = 17
	VLexerIDENTIFIER       = 18
	VLexerWHITESPACE       = 19
)

// VLexerWS_CHAN is the VLexer channel.
const VLexerWS_CHAN = 2
