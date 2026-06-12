// Code generated from TParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package httpidl // TParser
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

type TParser struct {
	*antlr.BaseParser
}

var TParserParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func tparserParserInit() {
	staticData := &TParserParserStaticData
	staticData.LiteralNames = []string{
		"", "'extends'", "'const'", "'enum'", "'type'", "'oneof'", "'rpc'",
		"'sse'", "'true'", "'false'", "'optional'", "'required'", "'bool'",
		"'int'", "'float'", "'string'", "'bytes'", "'map'", "'list'", "'<'",
		"'>'", "'('", "')'", "'{'", "'}'", "'='", "','",
	}
	staticData.SymbolicNames = []string{
		"", "KW_EXTENDS", "KW_CONST", "KW_ENUM", "KW_TYPE", "KW_ONEOF", "KW_RPC",
		"KW_SSE", "KW_TRUE", "KW_FALSE", "KW_OPTIONAL", "KW_REQUIRED", "TYPE_BOOL",
		"TYPE_INT", "TYPE_FLOAT", "TYPE_STRING", "TYPE_BYTES", "TYPE_MAP", "TYPE_LIST",
		"LESS_THAN", "GREATER_THAN", "LEFT_PAREN", "RIGHT_PAREN", "LEFT_BRACE",
		"RIGHT_BRACE", "EQUAL", "COMMA", "STRING", "IDENTIFIER", "INTEGER",
		"FLOAT", "NEWLINE", "WHITESPACE", "SINGLE_LINE_COMMENT", "MULTI_LINE_COMMENT",
	}
	staticData.RuleNames = []string{
		"document", "definition", "const_def", "enum_def", "enum_field", "type_def",
		"type_field", "embed_type_field", "common_type_field", "field_annotations",
		"oneof_def", "rpc_def", "rpc_req", "rpc_resp", "rpc_annotations", "annotation",
		"base_type", "user_type", "container_type", "map_type", "key_type",
		"list_type", "value_type", "const_value", "terminator",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 34, 257, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7, 20, 2,
		21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 1, 0, 1, 0, 1, 0,
		1, 0, 5, 0, 55, 8, 0, 10, 0, 12, 0, 58, 9, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 3, 1, 67, 8, 1, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 3,
		1, 3, 3, 3, 77, 8, 3, 1, 3, 1, 3, 1, 3, 3, 3, 82, 8, 3, 1, 3, 1, 3, 1,
		3, 5, 3, 87, 8, 3, 10, 3, 12, 3, 90, 9, 3, 1, 3, 3, 3, 93, 8, 3, 1, 3,
		1, 3, 1, 4, 1, 4, 1, 4, 1, 4, 3, 4, 101, 8, 4, 1, 5, 1, 5, 1, 5, 1, 5,
		1, 5, 3, 5, 108, 8, 5, 1, 5, 1, 5, 3, 5, 112, 8, 5, 1, 5, 1, 5, 1, 5, 5,
		5, 117, 8, 5, 10, 5, 12, 5, 120, 9, 5, 1, 5, 3, 5, 123, 8, 5, 1, 5, 1,
		5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 3, 5, 133, 8, 5, 1, 6, 1, 6, 3,
		6, 137, 8, 6, 1, 7, 1, 7, 1, 8, 3, 8, 142, 8, 8, 1, 8, 1, 8, 1, 8, 3, 8,
		147, 8, 8, 1, 9, 1, 9, 3, 9, 151, 8, 9, 1, 9, 1, 9, 1, 9, 3, 9, 156, 8,
		9, 1, 9, 5, 9, 159, 8, 9, 10, 9, 12, 9, 162, 9, 9, 1, 9, 3, 9, 165, 8,
		9, 1, 9, 1, 9, 1, 10, 1, 10, 1, 10, 1, 10, 3, 10, 173, 8, 10, 1, 10, 1,
		10, 1, 10, 5, 10, 178, 8, 10, 10, 10, 12, 10, 181, 9, 10, 1, 10, 3, 10,
		184, 8, 10, 1, 10, 1, 10, 1, 11, 1, 11, 1, 11, 1, 11, 1, 11, 1, 11, 1,
		11, 1, 11, 1, 12, 1, 12, 1, 13, 1, 13, 1, 14, 1, 14, 3, 14, 202, 8, 14,
		1, 14, 1, 14, 1, 14, 5, 14, 207, 8, 14, 10, 14, 12, 14, 210, 9, 14, 1,
		14, 3, 14, 213, 8, 14, 1, 14, 1, 14, 1, 15, 1, 15, 1, 15, 3, 15, 220, 8,
		15, 1, 16, 1, 16, 1, 17, 1, 17, 1, 18, 1, 18, 3, 18, 228, 8, 18, 1, 19,
		1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 20, 1, 20, 1, 21, 1, 21, 1,
		21, 1, 21, 1, 21, 1, 22, 1, 22, 1, 22, 1, 22, 3, 22, 248, 8, 22, 1, 23,
		1, 23, 1, 24, 4, 24, 253, 8, 24, 11, 24, 12, 24, 254, 1, 24, 0, 0, 25,
		0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36,
		38, 40, 42, 44, 46, 48, 0, 5, 1, 0, 10, 11, 1, 0, 6, 7, 1, 0, 12, 15, 2,
		0, 13, 13, 15, 15, 2, 0, 8, 9, 27, 30, 266, 0, 56, 1, 0, 0, 0, 2, 66, 1,
		0, 0, 0, 4, 68, 1, 0, 0, 0, 6, 74, 1, 0, 0, 0, 8, 96, 1, 0, 0, 0, 10, 132,
		1, 0, 0, 0, 12, 136, 1, 0, 0, 0, 14, 138, 1, 0, 0, 0, 16, 141, 1, 0, 0,
		0, 18, 148, 1, 0, 0, 0, 20, 168, 1, 0, 0, 0, 22, 187, 1, 0, 0, 0, 24, 195,
		1, 0, 0, 0, 26, 197, 1, 0, 0, 0, 28, 199, 1, 0, 0, 0, 30, 216, 1, 0, 0,
		0, 32, 221, 1, 0, 0, 0, 34, 223, 1, 0, 0, 0, 36, 227, 1, 0, 0, 0, 38, 229,
		1, 0, 0, 0, 40, 236, 1, 0, 0, 0, 42, 238, 1, 0, 0, 0, 44, 247, 1, 0, 0,
		0, 46, 249, 1, 0, 0, 0, 48, 252, 1, 0, 0, 0, 50, 51, 3, 2, 1, 0, 51, 52,
		3, 48, 24, 0, 52, 55, 1, 0, 0, 0, 53, 55, 3, 48, 24, 0, 54, 50, 1, 0, 0,
		0, 54, 53, 1, 0, 0, 0, 55, 58, 1, 0, 0, 0, 56, 54, 1, 0, 0, 0, 56, 57,
		1, 0, 0, 0, 57, 59, 1, 0, 0, 0, 58, 56, 1, 0, 0, 0, 59, 60, 5, 0, 0, 1,
		60, 1, 1, 0, 0, 0, 61, 67, 3, 4, 2, 0, 62, 67, 3, 6, 3, 0, 63, 67, 3, 10,
		5, 0, 64, 67, 3, 20, 10, 0, 65, 67, 3, 22, 11, 0, 66, 61, 1, 0, 0, 0, 66,
		62, 1, 0, 0, 0, 66, 63, 1, 0, 0, 0, 66, 64, 1, 0, 0, 0, 66, 65, 1, 0, 0,
		0, 67, 3, 1, 0, 0, 0, 68, 69, 5, 2, 0, 0, 69, 70, 3, 32, 16, 0, 70, 71,
		5, 28, 0, 0, 71, 72, 5, 25, 0, 0, 72, 73, 3, 46, 23, 0, 73, 5, 1, 0, 0,
		0, 74, 76, 5, 3, 0, 0, 75, 77, 5, 1, 0, 0, 76, 75, 1, 0, 0, 0, 76, 77,
		1, 0, 0, 0, 77, 78, 1, 0, 0, 0, 78, 79, 5, 28, 0, 0, 79, 81, 5, 23, 0,
		0, 80, 82, 3, 48, 24, 0, 81, 80, 1, 0, 0, 0, 81, 82, 1, 0, 0, 0, 82, 88,
		1, 0, 0, 0, 83, 84, 3, 8, 4, 0, 84, 85, 3, 48, 24, 0, 85, 87, 1, 0, 0,
		0, 86, 83, 1, 0, 0, 0, 87, 90, 1, 0, 0, 0, 88, 86, 1, 0, 0, 0, 88, 89,
		1, 0, 0, 0, 89, 92, 1, 0, 0, 0, 90, 88, 1, 0, 0, 0, 91, 93, 3, 48, 24,
		0, 92, 91, 1, 0, 0, 0, 92, 93, 1, 0, 0, 0, 93, 94, 1, 0, 0, 0, 94, 95,
		5, 24, 0, 0, 95, 7, 1, 0, 0, 0, 96, 97, 5, 28, 0, 0, 97, 98, 5, 25, 0,
		0, 98, 100, 5, 29, 0, 0, 99, 101, 3, 18, 9, 0, 100, 99, 1, 0, 0, 0, 100,
		101, 1, 0, 0, 0, 101, 9, 1, 0, 0, 0, 102, 103, 5, 4, 0, 0, 103, 107, 5,
		28, 0, 0, 104, 105, 5, 19, 0, 0, 105, 106, 5, 28, 0, 0, 106, 108, 5, 20,
		0, 0, 107, 104, 1, 0, 0, 0, 107, 108, 1, 0, 0, 0, 108, 109, 1, 0, 0, 0,
		109, 111, 5, 23, 0, 0, 110, 112, 3, 48, 24, 0, 111, 110, 1, 0, 0, 0, 111,
		112, 1, 0, 0, 0, 112, 118, 1, 0, 0, 0, 113, 114, 3, 12, 6, 0, 114, 115,
		3, 48, 24, 0, 115, 117, 1, 0, 0, 0, 116, 113, 1, 0, 0, 0, 117, 120, 1,
		0, 0, 0, 118, 116, 1, 0, 0, 0, 118, 119, 1, 0, 0, 0, 119, 122, 1, 0, 0,
		0, 120, 118, 1, 0, 0, 0, 121, 123, 3, 48, 24, 0, 122, 121, 1, 0, 0, 0,
		122, 123, 1, 0, 0, 0, 123, 124, 1, 0, 0, 0, 124, 133, 5, 24, 0, 0, 125,
		126, 5, 4, 0, 0, 126, 127, 5, 28, 0, 0, 127, 128, 5, 28, 0, 0, 128, 129,
		5, 19, 0, 0, 129, 130, 3, 44, 22, 0, 130, 131, 5, 20, 0, 0, 131, 133, 1,
		0, 0, 0, 132, 102, 1, 0, 0, 0, 132, 125, 1, 0, 0, 0, 133, 11, 1, 0, 0,
		0, 134, 137, 3, 14, 7, 0, 135, 137, 3, 16, 8, 0, 136, 134, 1, 0, 0, 0,
		136, 135, 1, 0, 0, 0, 137, 13, 1, 0, 0, 0, 138, 139, 3, 34, 17, 0, 139,
		15, 1, 0, 0, 0, 140, 142, 7, 0, 0, 0, 141, 140, 1, 0, 0, 0, 141, 142, 1,
		0, 0, 0, 142, 143, 1, 0, 0, 0, 143, 144, 3, 44, 22, 0, 144, 146, 5, 28,
		0, 0, 145, 147, 3, 18, 9, 0, 146, 145, 1, 0, 0, 0, 146, 147, 1, 0, 0, 0,
		147, 17, 1, 0, 0, 0, 148, 150, 5, 21, 0, 0, 149, 151, 3, 48, 24, 0, 150,
		149, 1, 0, 0, 0, 150, 151, 1, 0, 0, 0, 151, 152, 1, 0, 0, 0, 152, 160,
		3, 30, 15, 0, 153, 156, 5, 26, 0, 0, 154, 156, 3, 48, 24, 0, 155, 153,
		1, 0, 0, 0, 155, 154, 1, 0, 0, 0, 156, 157, 1, 0, 0, 0, 157, 159, 3, 30,
		15, 0, 158, 155, 1, 0, 0, 0, 159, 162, 1, 0, 0, 0, 160, 158, 1, 0, 0, 0,
		160, 161, 1, 0, 0, 0, 161, 164, 1, 0, 0, 0, 162, 160, 1, 0, 0, 0, 163,
		165, 3, 48, 24, 0, 164, 163, 1, 0, 0, 0, 164, 165, 1, 0, 0, 0, 165, 166,
		1, 0, 0, 0, 166, 167, 5, 22, 0, 0, 167, 19, 1, 0, 0, 0, 168, 169, 5, 5,
		0, 0, 169, 170, 5, 28, 0, 0, 170, 172, 5, 23, 0, 0, 171, 173, 3, 48, 24,
		0, 172, 171, 1, 0, 0, 0, 172, 173, 1, 0, 0, 0, 173, 179, 1, 0, 0, 0, 174,
		175, 3, 34, 17, 0, 175, 176, 3, 48, 24, 0, 176, 178, 1, 0, 0, 0, 177, 174,
		1, 0, 0, 0, 178, 181, 1, 0, 0, 0, 179, 177, 1, 0, 0, 0, 179, 180, 1, 0,
		0, 0, 180, 183, 1, 0, 0, 0, 181, 179, 1, 0, 0, 0, 182, 184, 3, 48, 24,
		0, 183, 182, 1, 0, 0, 0, 183, 184, 1, 0, 0, 0, 184, 185, 1, 0, 0, 0, 185,
		186, 5, 24, 0, 0, 186, 21, 1, 0, 0, 0, 187, 188, 7, 1, 0, 0, 188, 189,
		5, 28, 0, 0, 189, 190, 5, 21, 0, 0, 190, 191, 3, 24, 12, 0, 191, 192, 5,
		22, 0, 0, 192, 193, 3, 26, 13, 0, 193, 194, 3, 28, 14, 0, 194, 23, 1, 0,
		0, 0, 195, 196, 3, 34, 17, 0, 196, 25, 1, 0, 0, 0, 197, 198, 3, 44, 22,
		0, 198, 27, 1, 0, 0, 0, 199, 201, 5, 23, 0, 0, 200, 202, 3, 48, 24, 0,
		201, 200, 1, 0, 0, 0, 201, 202, 1, 0, 0, 0, 202, 208, 1, 0, 0, 0, 203,
		204, 3, 30, 15, 0, 204, 205, 3, 48, 24, 0, 205, 207, 1, 0, 0, 0, 206, 203,
		1, 0, 0, 0, 207, 210, 1, 0, 0, 0, 208, 206, 1, 0, 0, 0, 208, 209, 1, 0,
		0, 0, 209, 212, 1, 0, 0, 0, 210, 208, 1, 0, 0, 0, 211, 213, 3, 48, 24,
		0, 212, 211, 1, 0, 0, 0, 212, 213, 1, 0, 0, 0, 213, 214, 1, 0, 0, 0, 214,
		215, 5, 24, 0, 0, 215, 29, 1, 0, 0, 0, 216, 219, 5, 28, 0, 0, 217, 218,
		5, 25, 0, 0, 218, 220, 3, 46, 23, 0, 219, 217, 1, 0, 0, 0, 219, 220, 1,
		0, 0, 0, 220, 31, 1, 0, 0, 0, 221, 222, 7, 2, 0, 0, 222, 33, 1, 0, 0, 0,
		223, 224, 5, 28, 0, 0, 224, 35, 1, 0, 0, 0, 225, 228, 3, 38, 19, 0, 226,
		228, 3, 42, 21, 0, 227, 225, 1, 0, 0, 0, 227, 226, 1, 0, 0, 0, 228, 37,
		1, 0, 0, 0, 229, 230, 5, 17, 0, 0, 230, 231, 5, 19, 0, 0, 231, 232, 3,
		40, 20, 0, 232, 233, 5, 26, 0, 0, 233, 234, 3, 44, 22, 0, 234, 235, 5,
		20, 0, 0, 235, 39, 1, 0, 0, 0, 236, 237, 7, 3, 0, 0, 237, 41, 1, 0, 0,
		0, 238, 239, 5, 18, 0, 0, 239, 240, 5, 19, 0, 0, 240, 241, 3, 44, 22, 0,
		241, 242, 5, 20, 0, 0, 242, 43, 1, 0, 0, 0, 243, 248, 3, 32, 16, 0, 244,
		248, 3, 34, 17, 0, 245, 248, 3, 36, 18, 0, 246, 248, 5, 16, 0, 0, 247,
		243, 1, 0, 0, 0, 247, 244, 1, 0, 0, 0, 247, 245, 1, 0, 0, 0, 247, 246,
		1, 0, 0, 0, 248, 45, 1, 0, 0, 0, 249, 250, 7, 4, 0, 0, 250, 47, 1, 0, 0,
		0, 251, 253, 5, 31, 0, 0, 252, 251, 1, 0, 0, 0, 253, 254, 1, 0, 0, 0, 254,
		252, 1, 0, 0, 0, 254, 255, 1, 0, 0, 0, 255, 49, 1, 0, 0, 0, 30, 54, 56,
		66, 76, 81, 88, 92, 100, 107, 111, 118, 122, 132, 136, 141, 146, 150, 155,
		160, 164, 172, 179, 183, 201, 208, 212, 219, 227, 247, 254,
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

// TParserInit initializes any static state used to implement TParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewTParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func TParserInit() {
	staticData := &TParserParserStaticData
	staticData.once.Do(tparserParserInit)
}

// NewTParser produces a new parser instance for the optional input antlr.TokenStream.
func NewTParser(input antlr.TokenStream) *TParser {
	TParserInit()
	this := new(TParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &TParserParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "TParser.g4"

	return this
}

// TParser tokens.
const (
	TParserEOF                 = antlr.TokenEOF
	TParserKW_EXTENDS          = 1
	TParserKW_CONST            = 2
	TParserKW_ENUM             = 3
	TParserKW_TYPE             = 4
	TParserKW_ONEOF            = 5
	TParserKW_RPC              = 6
	TParserKW_SSE              = 7
	TParserKW_TRUE             = 8
	TParserKW_FALSE            = 9
	TParserKW_OPTIONAL         = 10
	TParserKW_REQUIRED         = 11
	TParserTYPE_BOOL           = 12
	TParserTYPE_INT            = 13
	TParserTYPE_FLOAT          = 14
	TParserTYPE_STRING         = 15
	TParserTYPE_BYTES          = 16
	TParserTYPE_MAP            = 17
	TParserTYPE_LIST           = 18
	TParserLESS_THAN           = 19
	TParserGREATER_THAN        = 20
	TParserLEFT_PAREN          = 21
	TParserRIGHT_PAREN         = 22
	TParserLEFT_BRACE          = 23
	TParserRIGHT_BRACE         = 24
	TParserEQUAL               = 25
	TParserCOMMA               = 26
	TParserSTRING              = 27
	TParserIDENTIFIER          = 28
	TParserINTEGER             = 29
	TParserFLOAT               = 30
	TParserNEWLINE             = 31
	TParserWHITESPACE          = 32
	TParserSINGLE_LINE_COMMENT = 33
	TParserMULTI_LINE_COMMENT  = 34
)

// TParser rules.
const (
	TParserRULE_document          = 0
	TParserRULE_definition        = 1
	TParserRULE_const_def         = 2
	TParserRULE_enum_def          = 3
	TParserRULE_enum_field        = 4
	TParserRULE_type_def          = 5
	TParserRULE_type_field        = 6
	TParserRULE_embed_type_field  = 7
	TParserRULE_common_type_field = 8
	TParserRULE_field_annotations = 9
	TParserRULE_oneof_def         = 10
	TParserRULE_rpc_def           = 11
	TParserRULE_rpc_req           = 12
	TParserRULE_rpc_resp          = 13
	TParserRULE_rpc_annotations   = 14
	TParserRULE_annotation        = 15
	TParserRULE_base_type         = 16
	TParserRULE_user_type         = 17
	TParserRULE_container_type    = 18
	TParserRULE_map_type          = 19
	TParserRULE_key_type          = 20
	TParserRULE_list_type         = 21
	TParserRULE_value_type        = 22
	TParserRULE_const_value       = 23
	TParserRULE_terminator        = 24
)

// IDocumentContext is an interface to support dynamic dispatch.
type IDocumentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	EOF() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllDefinition() []IDefinitionContext
	Definition(i int) IDefinitionContext

	// IsDocumentContext differentiates from other interfaces.
	IsDocumentContext()
}

type DocumentContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDocumentContext() *DocumentContext {
	var p = new(DocumentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_document
	return p
}

func InitEmptyDocumentContext(p *DocumentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_document
}

func (*DocumentContext) IsDocumentContext() {}

func NewDocumentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DocumentContext {
	var p = new(DocumentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_document

	return p
}

func (s *DocumentContext) GetParser() antlr.Parser { return s.parser }

func (s *DocumentContext) EOF() antlr.TerminalNode {
	return s.GetToken(TParserEOF, 0)
}

func (s *DocumentContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *DocumentContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *DocumentContext) AllDefinition() []IDefinitionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IDefinitionContext); ok {
			len++
		}
	}

	tst := make([]IDefinitionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IDefinitionContext); ok {
			tst[i] = t.(IDefinitionContext)
			i++
		}
	}

	return tst
}

func (s *DocumentContext) Definition(i int) IDefinitionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IDefinitionContext); ok {
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

	return t.(IDefinitionContext)
}

func (s *DocumentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DocumentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DocumentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterDocument(s)
	}
}

func (s *DocumentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitDocument(s)
	}
}

func (p *TParser) Document() (localctx IDocumentContext) {
	localctx = NewDocumentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, TParserRULE_document)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(56)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2147483900) != 0 {
		p.SetState(54)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}

		switch p.GetTokenStream().LA(1) {
		case TParserKW_CONST, TParserKW_ENUM, TParserKW_TYPE, TParserKW_ONEOF, TParserKW_RPC, TParserKW_SSE:
			{
				p.SetState(50)
				p.Definition()
			}
			{
				p.SetState(51)
				p.Terminator()
			}

		case TParserNEWLINE:
			{
				p.SetState(53)
				p.Terminator()
			}

		default:
			p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			goto errorExit
		}

		p.SetState(58)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(59)
		p.Match(TParserEOF)
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

// IDefinitionContext is an interface to support dynamic dispatch.
type IDefinitionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Const_def() IConst_defContext
	Enum_def() IEnum_defContext
	Type_def() IType_defContext
	Oneof_def() IOneof_defContext
	Rpc_def() IRpc_defContext

	// IsDefinitionContext differentiates from other interfaces.
	IsDefinitionContext()
}

type DefinitionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDefinitionContext() *DefinitionContext {
	var p = new(DefinitionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_definition
	return p
}

func InitEmptyDefinitionContext(p *DefinitionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_definition
}

func (*DefinitionContext) IsDefinitionContext() {}

func NewDefinitionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DefinitionContext {
	var p = new(DefinitionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_definition

	return p
}

func (s *DefinitionContext) GetParser() antlr.Parser { return s.parser }

func (s *DefinitionContext) Const_def() IConst_defContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConst_defContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConst_defContext)
}

func (s *DefinitionContext) Enum_def() IEnum_defContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEnum_defContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEnum_defContext)
}

func (s *DefinitionContext) Type_def() IType_defContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IType_defContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IType_defContext)
}

func (s *DefinitionContext) Oneof_def() IOneof_defContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOneof_defContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOneof_defContext)
}

func (s *DefinitionContext) Rpc_def() IRpc_defContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRpc_defContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRpc_defContext)
}

func (s *DefinitionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DefinitionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DefinitionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterDefinition(s)
	}
}

func (s *DefinitionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitDefinition(s)
	}
}

func (p *TParser) Definition() (localctx IDefinitionContext) {
	localctx = NewDefinitionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, TParserRULE_definition)
	p.SetState(66)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case TParserKW_CONST:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(61)
			p.Const_def()
		}

	case TParserKW_ENUM:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(62)
			p.Enum_def()
		}

	case TParserKW_TYPE:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(63)
			p.Type_def()
		}

	case TParserKW_ONEOF:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(64)
			p.Oneof_def()
		}

	case TParserKW_RPC, TParserKW_SSE:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(65)
			p.Rpc_def()
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

// IConst_defContext is an interface to support dynamic dispatch.
type IConst_defContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	KW_CONST() antlr.TerminalNode
	Base_type() IBase_typeContext
	IDENTIFIER() antlr.TerminalNode
	EQUAL() antlr.TerminalNode
	Const_value() IConst_valueContext

	// IsConst_defContext differentiates from other interfaces.
	IsConst_defContext()
}

type Const_defContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyConst_defContext() *Const_defContext {
	var p = new(Const_defContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_const_def
	return p
}

func InitEmptyConst_defContext(p *Const_defContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_const_def
}

func (*Const_defContext) IsConst_defContext() {}

func NewConst_defContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Const_defContext {
	var p = new(Const_defContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_const_def

	return p
}

func (s *Const_defContext) GetParser() antlr.Parser { return s.parser }

func (s *Const_defContext) KW_CONST() antlr.TerminalNode {
	return s.GetToken(TParserKW_CONST, 0)
}

func (s *Const_defContext) Base_type() IBase_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBase_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBase_typeContext)
}

func (s *Const_defContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Const_defContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(TParserEQUAL, 0)
}

func (s *Const_defContext) Const_value() IConst_valueContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConst_valueContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConst_valueContext)
}

func (s *Const_defContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Const_defContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Const_defContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterConst_def(s)
	}
}

func (s *Const_defContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitConst_def(s)
	}
}

func (p *TParser) Const_def() (localctx IConst_defContext) {
	localctx = NewConst_defContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, TParserRULE_const_def)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(68)
		p.Match(TParserKW_CONST)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(69)
		p.Base_type()
	}
	{
		p.SetState(70)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(71)
		p.Match(TParserEQUAL)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(72)
		p.Const_value()
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

// IEnum_defContext is an interface to support dynamic dispatch.
type IEnum_defContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	KW_ENUM() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode
	LEFT_BRACE() antlr.TerminalNode
	RIGHT_BRACE() antlr.TerminalNode
	KW_EXTENDS() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllEnum_field() []IEnum_fieldContext
	Enum_field(i int) IEnum_fieldContext

	// IsEnum_defContext differentiates from other interfaces.
	IsEnum_defContext()
}

type Enum_defContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEnum_defContext() *Enum_defContext {
	var p = new(Enum_defContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_enum_def
	return p
}

func InitEmptyEnum_defContext(p *Enum_defContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_enum_def
}

func (*Enum_defContext) IsEnum_defContext() {}

func NewEnum_defContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Enum_defContext {
	var p = new(Enum_defContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_enum_def

	return p
}

func (s *Enum_defContext) GetParser() antlr.Parser { return s.parser }

func (s *Enum_defContext) KW_ENUM() antlr.TerminalNode {
	return s.GetToken(TParserKW_ENUM, 0)
}

func (s *Enum_defContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Enum_defContext) LEFT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_BRACE, 0)
}

func (s *Enum_defContext) RIGHT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_BRACE, 0)
}

func (s *Enum_defContext) KW_EXTENDS() antlr.TerminalNode {
	return s.GetToken(TParserKW_EXTENDS, 0)
}

func (s *Enum_defContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *Enum_defContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *Enum_defContext) AllEnum_field() []IEnum_fieldContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IEnum_fieldContext); ok {
			len++
		}
	}

	tst := make([]IEnum_fieldContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IEnum_fieldContext); ok {
			tst[i] = t.(IEnum_fieldContext)
			i++
		}
	}

	return tst
}

func (s *Enum_defContext) Enum_field(i int) IEnum_fieldContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEnum_fieldContext); ok {
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

	return t.(IEnum_fieldContext)
}

func (s *Enum_defContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Enum_defContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Enum_defContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterEnum_def(s)
	}
}

func (s *Enum_defContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitEnum_def(s)
	}
}

func (p *TParser) Enum_def() (localctx IEnum_defContext) {
	localctx = NewEnum_defContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, TParserRULE_enum_def)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(74)
		p.Match(TParserKW_ENUM)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(76)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserKW_EXTENDS {
		{
			p.SetState(75)
			p.Match(TParserKW_EXTENDS)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(78)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(79)
		p.Match(TParserLEFT_BRACE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(81)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 4, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(80)
			p.Terminator()
		}

	} else if p.HasError() { // JIM
		goto errorExit
	}
	p.SetState(88)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == TParserIDENTIFIER {
		{
			p.SetState(83)
			p.Enum_field()
		}
		{
			p.SetState(84)
			p.Terminator()
		}

		p.SetState(90)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	p.SetState(92)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserNEWLINE {
		{
			p.SetState(91)
			p.Terminator()
		}

	}
	{
		p.SetState(94)
		p.Match(TParserRIGHT_BRACE)
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

// IEnum_fieldContext is an interface to support dynamic dispatch.
type IEnum_fieldContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	EQUAL() antlr.TerminalNode
	INTEGER() antlr.TerminalNode
	Field_annotations() IField_annotationsContext

	// IsEnum_fieldContext differentiates from other interfaces.
	IsEnum_fieldContext()
}

type Enum_fieldContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEnum_fieldContext() *Enum_fieldContext {
	var p = new(Enum_fieldContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_enum_field
	return p
}

func InitEmptyEnum_fieldContext(p *Enum_fieldContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_enum_field
}

func (*Enum_fieldContext) IsEnum_fieldContext() {}

func NewEnum_fieldContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Enum_fieldContext {
	var p = new(Enum_fieldContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_enum_field

	return p
}

func (s *Enum_fieldContext) GetParser() antlr.Parser { return s.parser }

func (s *Enum_fieldContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Enum_fieldContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(TParserEQUAL, 0)
}

func (s *Enum_fieldContext) INTEGER() antlr.TerminalNode {
	return s.GetToken(TParserINTEGER, 0)
}

func (s *Enum_fieldContext) Field_annotations() IField_annotationsContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IField_annotationsContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IField_annotationsContext)
}

func (s *Enum_fieldContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Enum_fieldContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Enum_fieldContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterEnum_field(s)
	}
}

func (s *Enum_fieldContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitEnum_field(s)
	}
}

func (p *TParser) Enum_field() (localctx IEnum_fieldContext) {
	localctx = NewEnum_fieldContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, TParserRULE_enum_field)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(96)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(97)
		p.Match(TParserEQUAL)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(98)
		p.Match(TParserINTEGER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(100)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserLEFT_PAREN {
		{
			p.SetState(99)
			p.Field_annotations()
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

// IType_defContext is an interface to support dynamic dispatch.
type IType_defContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	KW_TYPE() antlr.TerminalNode
	AllIDENTIFIER() []antlr.TerminalNode
	IDENTIFIER(i int) antlr.TerminalNode
	LEFT_BRACE() antlr.TerminalNode
	RIGHT_BRACE() antlr.TerminalNode
	LESS_THAN() antlr.TerminalNode
	GREATER_THAN() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllType_field() []IType_fieldContext
	Type_field(i int) IType_fieldContext
	Value_type() IValue_typeContext

	// IsType_defContext differentiates from other interfaces.
	IsType_defContext()
}

type Type_defContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyType_defContext() *Type_defContext {
	var p = new(Type_defContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_type_def
	return p
}

func InitEmptyType_defContext(p *Type_defContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_type_def
}

func (*Type_defContext) IsType_defContext() {}

func NewType_defContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Type_defContext {
	var p = new(Type_defContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_type_def

	return p
}

func (s *Type_defContext) GetParser() antlr.Parser { return s.parser }

func (s *Type_defContext) KW_TYPE() antlr.TerminalNode {
	return s.GetToken(TParserKW_TYPE, 0)
}

func (s *Type_defContext) AllIDENTIFIER() []antlr.TerminalNode {
	return s.GetTokens(TParserIDENTIFIER)
}

func (s *Type_defContext) IDENTIFIER(i int) antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, i)
}

func (s *Type_defContext) LEFT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_BRACE, 0)
}

func (s *Type_defContext) RIGHT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_BRACE, 0)
}

func (s *Type_defContext) LESS_THAN() antlr.TerminalNode {
	return s.GetToken(TParserLESS_THAN, 0)
}

func (s *Type_defContext) GREATER_THAN() antlr.TerminalNode {
	return s.GetToken(TParserGREATER_THAN, 0)
}

func (s *Type_defContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *Type_defContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *Type_defContext) AllType_field() []IType_fieldContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IType_fieldContext); ok {
			len++
		}
	}

	tst := make([]IType_fieldContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IType_fieldContext); ok {
			tst[i] = t.(IType_fieldContext)
			i++
		}
	}

	return tst
}

func (s *Type_defContext) Type_field(i int) IType_fieldContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IType_fieldContext); ok {
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

	return t.(IType_fieldContext)
}

func (s *Type_defContext) Value_type() IValue_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValue_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValue_typeContext)
}

func (s *Type_defContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Type_defContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Type_defContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterType_def(s)
	}
}

func (s *Type_defContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitType_def(s)
	}
}

func (p *TParser) Type_def() (localctx IType_defContext) {
	localctx = NewType_defContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, TParserRULE_type_def)
	var _la int

	p.SetState(132)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 12, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(102)
			p.Match(TParserKW_TYPE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(103)
			p.Match(TParserIDENTIFIER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(107)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == TParserLESS_THAN {
			{
				p.SetState(104)
				p.Match(TParserLESS_THAN)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(105)
				p.Match(TParserIDENTIFIER)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(106)
				p.Match(TParserGREATER_THAN)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(109)
			p.Match(TParserLEFT_BRACE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(111)
		p.GetErrorHandler().Sync(p)

		if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 9, p.GetParserRuleContext()) == 1 {
			{
				p.SetState(110)
				p.Terminator()
			}

		} else if p.HasError() { // JIM
			goto errorExit
		}
		p.SetState(118)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&268958720) != 0 {
			{
				p.SetState(113)
				p.Type_field()
			}
			{
				p.SetState(114)
				p.Terminator()
			}

			p.SetState(120)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}
		p.SetState(122)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == TParserNEWLINE {
			{
				p.SetState(121)
				p.Terminator()
			}

		}
		{
			p.SetState(124)
			p.Match(TParserRIGHT_BRACE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(125)
			p.Match(TParserKW_TYPE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(126)
			p.Match(TParserIDENTIFIER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(127)
			p.Match(TParserIDENTIFIER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(128)
			p.Match(TParserLESS_THAN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(129)
			p.Value_type()
		}
		{
			p.SetState(130)
			p.Match(TParserGREATER_THAN)
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

// IType_fieldContext is an interface to support dynamic dispatch.
type IType_fieldContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Embed_type_field() IEmbed_type_fieldContext
	Common_type_field() ICommon_type_fieldContext

	// IsType_fieldContext differentiates from other interfaces.
	IsType_fieldContext()
}

type Type_fieldContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyType_fieldContext() *Type_fieldContext {
	var p = new(Type_fieldContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_type_field
	return p
}

func InitEmptyType_fieldContext(p *Type_fieldContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_type_field
}

func (*Type_fieldContext) IsType_fieldContext() {}

func NewType_fieldContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Type_fieldContext {
	var p = new(Type_fieldContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_type_field

	return p
}

func (s *Type_fieldContext) GetParser() antlr.Parser { return s.parser }

func (s *Type_fieldContext) Embed_type_field() IEmbed_type_fieldContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEmbed_type_fieldContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEmbed_type_fieldContext)
}

func (s *Type_fieldContext) Common_type_field() ICommon_type_fieldContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICommon_type_fieldContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICommon_type_fieldContext)
}

func (s *Type_fieldContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Type_fieldContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Type_fieldContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterType_field(s)
	}
}

func (s *Type_fieldContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitType_field(s)
	}
}

func (p *TParser) Type_field() (localctx IType_fieldContext) {
	localctx = NewType_fieldContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, TParserRULE_type_field)
	p.SetState(136)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 13, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(134)
			p.Embed_type_field()
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(135)
			p.Common_type_field()
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

// IEmbed_type_fieldContext is an interface to support dynamic dispatch.
type IEmbed_type_fieldContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	User_type() IUser_typeContext

	// IsEmbed_type_fieldContext differentiates from other interfaces.
	IsEmbed_type_fieldContext()
}

type Embed_type_fieldContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEmbed_type_fieldContext() *Embed_type_fieldContext {
	var p = new(Embed_type_fieldContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_embed_type_field
	return p
}

func InitEmptyEmbed_type_fieldContext(p *Embed_type_fieldContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_embed_type_field
}

func (*Embed_type_fieldContext) IsEmbed_type_fieldContext() {}

func NewEmbed_type_fieldContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Embed_type_fieldContext {
	var p = new(Embed_type_fieldContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_embed_type_field

	return p
}

func (s *Embed_type_fieldContext) GetParser() antlr.Parser { return s.parser }

func (s *Embed_type_fieldContext) User_type() IUser_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUser_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUser_typeContext)
}

func (s *Embed_type_fieldContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Embed_type_fieldContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Embed_type_fieldContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterEmbed_type_field(s)
	}
}

func (s *Embed_type_fieldContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitEmbed_type_field(s)
	}
}

func (p *TParser) Embed_type_field() (localctx IEmbed_type_fieldContext) {
	localctx = NewEmbed_type_fieldContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, TParserRULE_embed_type_field)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(138)
		p.User_type()
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

// ICommon_type_fieldContext is an interface to support dynamic dispatch.
type ICommon_type_fieldContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Value_type() IValue_typeContext
	IDENTIFIER() antlr.TerminalNode
	Field_annotations() IField_annotationsContext
	KW_REQUIRED() antlr.TerminalNode
	KW_OPTIONAL() antlr.TerminalNode

	// IsCommon_type_fieldContext differentiates from other interfaces.
	IsCommon_type_fieldContext()
}

type Common_type_fieldContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCommon_type_fieldContext() *Common_type_fieldContext {
	var p = new(Common_type_fieldContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_common_type_field
	return p
}

func InitEmptyCommon_type_fieldContext(p *Common_type_fieldContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_common_type_field
}

func (*Common_type_fieldContext) IsCommon_type_fieldContext() {}

func NewCommon_type_fieldContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Common_type_fieldContext {
	var p = new(Common_type_fieldContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_common_type_field

	return p
}

func (s *Common_type_fieldContext) GetParser() antlr.Parser { return s.parser }

func (s *Common_type_fieldContext) Value_type() IValue_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValue_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValue_typeContext)
}

func (s *Common_type_fieldContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Common_type_fieldContext) Field_annotations() IField_annotationsContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IField_annotationsContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IField_annotationsContext)
}

func (s *Common_type_fieldContext) KW_REQUIRED() antlr.TerminalNode {
	return s.GetToken(TParserKW_REQUIRED, 0)
}

func (s *Common_type_fieldContext) KW_OPTIONAL() antlr.TerminalNode {
	return s.GetToken(TParserKW_OPTIONAL, 0)
}

func (s *Common_type_fieldContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Common_type_fieldContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Common_type_fieldContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterCommon_type_field(s)
	}
}

func (s *Common_type_fieldContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitCommon_type_field(s)
	}
}

func (p *TParser) Common_type_field() (localctx ICommon_type_fieldContext) {
	localctx = NewCommon_type_fieldContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, TParserRULE_common_type_field)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(141)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserKW_OPTIONAL || _la == TParserKW_REQUIRED {
		{
			p.SetState(140)
			_la = p.GetTokenStream().LA(1)

			if !(_la == TParserKW_OPTIONAL || _la == TParserKW_REQUIRED) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

	}
	{
		p.SetState(143)
		p.Value_type()
	}
	{
		p.SetState(144)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(146)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserLEFT_PAREN {
		{
			p.SetState(145)
			p.Field_annotations()
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

// IField_annotationsContext is an interface to support dynamic dispatch.
type IField_annotationsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LEFT_PAREN() antlr.TerminalNode
	AllAnnotation() []IAnnotationContext
	Annotation(i int) IAnnotationContext
	RIGHT_PAREN() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsField_annotationsContext differentiates from other interfaces.
	IsField_annotationsContext()
}

type Field_annotationsContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyField_annotationsContext() *Field_annotationsContext {
	var p = new(Field_annotationsContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_field_annotations
	return p
}

func InitEmptyField_annotationsContext(p *Field_annotationsContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_field_annotations
}

func (*Field_annotationsContext) IsField_annotationsContext() {}

func NewField_annotationsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Field_annotationsContext {
	var p = new(Field_annotationsContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_field_annotations

	return p
}

func (s *Field_annotationsContext) GetParser() antlr.Parser { return s.parser }

func (s *Field_annotationsContext) LEFT_PAREN() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_PAREN, 0)
}

func (s *Field_annotationsContext) AllAnnotation() []IAnnotationContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IAnnotationContext); ok {
			len++
		}
	}

	tst := make([]IAnnotationContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IAnnotationContext); ok {
			tst[i] = t.(IAnnotationContext)
			i++
		}
	}

	return tst
}

func (s *Field_annotationsContext) Annotation(i int) IAnnotationContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IAnnotationContext); ok {
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

	return t.(IAnnotationContext)
}

func (s *Field_annotationsContext) RIGHT_PAREN() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_PAREN, 0)
}

func (s *Field_annotationsContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *Field_annotationsContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *Field_annotationsContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(TParserCOMMA)
}

func (s *Field_annotationsContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(TParserCOMMA, i)
}

func (s *Field_annotationsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Field_annotationsContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Field_annotationsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterField_annotations(s)
	}
}

func (s *Field_annotationsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitField_annotations(s)
	}
}

func (p *TParser) Field_annotations() (localctx IField_annotationsContext) {
	localctx = NewField_annotationsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, TParserRULE_field_annotations)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(148)
		p.Match(TParserLEFT_PAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(150)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserNEWLINE {
		{
			p.SetState(149)
			p.Terminator()
		}

	}
	{
		p.SetState(152)
		p.Annotation()
	}
	p.SetState(160)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 18, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			p.SetState(155)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetTokenStream().LA(1) {
			case TParserCOMMA:
				{
					p.SetState(153)
					p.Match(TParserCOMMA)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

			case TParserNEWLINE:
				{
					p.SetState(154)
					p.Terminator()
				}

			default:
				p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				goto errorExit
			}
			{
				p.SetState(157)
				p.Annotation()
			}

		}
		p.SetState(162)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 18, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}
	p.SetState(164)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserNEWLINE {
		{
			p.SetState(163)
			p.Terminator()
		}

	}
	{
		p.SetState(166)
		p.Match(TParserRIGHT_PAREN)
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

// IOneof_defContext is an interface to support dynamic dispatch.
type IOneof_defContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	KW_ONEOF() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode
	LEFT_BRACE() antlr.TerminalNode
	RIGHT_BRACE() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllUser_type() []IUser_typeContext
	User_type(i int) IUser_typeContext

	// IsOneof_defContext differentiates from other interfaces.
	IsOneof_defContext()
}

type Oneof_defContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyOneof_defContext() *Oneof_defContext {
	var p = new(Oneof_defContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_oneof_def
	return p
}

func InitEmptyOneof_defContext(p *Oneof_defContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_oneof_def
}

func (*Oneof_defContext) IsOneof_defContext() {}

func NewOneof_defContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Oneof_defContext {
	var p = new(Oneof_defContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_oneof_def

	return p
}

func (s *Oneof_defContext) GetParser() antlr.Parser { return s.parser }

func (s *Oneof_defContext) KW_ONEOF() antlr.TerminalNode {
	return s.GetToken(TParserKW_ONEOF, 0)
}

func (s *Oneof_defContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Oneof_defContext) LEFT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_BRACE, 0)
}

func (s *Oneof_defContext) RIGHT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_BRACE, 0)
}

func (s *Oneof_defContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *Oneof_defContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *Oneof_defContext) AllUser_type() []IUser_typeContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IUser_typeContext); ok {
			len++
		}
	}

	tst := make([]IUser_typeContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IUser_typeContext); ok {
			tst[i] = t.(IUser_typeContext)
			i++
		}
	}

	return tst
}

func (s *Oneof_defContext) User_type(i int) IUser_typeContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUser_typeContext); ok {
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

	return t.(IUser_typeContext)
}

func (s *Oneof_defContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Oneof_defContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Oneof_defContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterOneof_def(s)
	}
}

func (s *Oneof_defContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitOneof_def(s)
	}
}

func (p *TParser) Oneof_def() (localctx IOneof_defContext) {
	localctx = NewOneof_defContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, TParserRULE_oneof_def)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(168)
		p.Match(TParserKW_ONEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(169)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(170)
		p.Match(TParserLEFT_BRACE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(172)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 20, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(171)
			p.Terminator()
		}

	} else if p.HasError() { // JIM
		goto errorExit
	}
	p.SetState(179)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == TParserIDENTIFIER {
		{
			p.SetState(174)
			p.User_type()
		}
		{
			p.SetState(175)
			p.Terminator()
		}

		p.SetState(181)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	p.SetState(183)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserNEWLINE {
		{
			p.SetState(182)
			p.Terminator()
		}

	}
	{
		p.SetState(185)
		p.Match(TParserRIGHT_BRACE)
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

// IRpc_defContext is an interface to support dynamic dispatch.
type IRpc_defContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	LEFT_PAREN() antlr.TerminalNode
	Rpc_req() IRpc_reqContext
	RIGHT_PAREN() antlr.TerminalNode
	Rpc_resp() IRpc_respContext
	Rpc_annotations() IRpc_annotationsContext
	KW_RPC() antlr.TerminalNode
	KW_SSE() antlr.TerminalNode

	// IsRpc_defContext differentiates from other interfaces.
	IsRpc_defContext()
}

type Rpc_defContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRpc_defContext() *Rpc_defContext {
	var p = new(Rpc_defContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_def
	return p
}

func InitEmptyRpc_defContext(p *Rpc_defContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_def
}

func (*Rpc_defContext) IsRpc_defContext() {}

func NewRpc_defContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Rpc_defContext {
	var p = new(Rpc_defContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_rpc_def

	return p
}

func (s *Rpc_defContext) GetParser() antlr.Parser { return s.parser }

func (s *Rpc_defContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Rpc_defContext) LEFT_PAREN() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_PAREN, 0)
}

func (s *Rpc_defContext) Rpc_req() IRpc_reqContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRpc_reqContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRpc_reqContext)
}

func (s *Rpc_defContext) RIGHT_PAREN() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_PAREN, 0)
}

func (s *Rpc_defContext) Rpc_resp() IRpc_respContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRpc_respContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRpc_respContext)
}

func (s *Rpc_defContext) Rpc_annotations() IRpc_annotationsContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRpc_annotationsContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRpc_annotationsContext)
}

func (s *Rpc_defContext) KW_RPC() antlr.TerminalNode {
	return s.GetToken(TParserKW_RPC, 0)
}

func (s *Rpc_defContext) KW_SSE() antlr.TerminalNode {
	return s.GetToken(TParserKW_SSE, 0)
}

func (s *Rpc_defContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Rpc_defContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Rpc_defContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterRpc_def(s)
	}
}

func (s *Rpc_defContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitRpc_def(s)
	}
}

func (p *TParser) Rpc_def() (localctx IRpc_defContext) {
	localctx = NewRpc_defContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, TParserRULE_rpc_def)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(187)
		_la = p.GetTokenStream().LA(1)

		if !(_la == TParserKW_RPC || _la == TParserKW_SSE) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	{
		p.SetState(188)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(189)
		p.Match(TParserLEFT_PAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(190)
		p.Rpc_req()
	}
	{
		p.SetState(191)
		p.Match(TParserRIGHT_PAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(192)
		p.Rpc_resp()
	}
	{
		p.SetState(193)
		p.Rpc_annotations()
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

// IRpc_reqContext is an interface to support dynamic dispatch.
type IRpc_reqContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	User_type() IUser_typeContext

	// IsRpc_reqContext differentiates from other interfaces.
	IsRpc_reqContext()
}

type Rpc_reqContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRpc_reqContext() *Rpc_reqContext {
	var p = new(Rpc_reqContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_req
	return p
}

func InitEmptyRpc_reqContext(p *Rpc_reqContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_req
}

func (*Rpc_reqContext) IsRpc_reqContext() {}

func NewRpc_reqContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Rpc_reqContext {
	var p = new(Rpc_reqContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_rpc_req

	return p
}

func (s *Rpc_reqContext) GetParser() antlr.Parser { return s.parser }

func (s *Rpc_reqContext) User_type() IUser_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUser_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUser_typeContext)
}

func (s *Rpc_reqContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Rpc_reqContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Rpc_reqContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterRpc_req(s)
	}
}

func (s *Rpc_reqContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitRpc_req(s)
	}
}

func (p *TParser) Rpc_req() (localctx IRpc_reqContext) {
	localctx = NewRpc_reqContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, TParserRULE_rpc_req)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(195)
		p.User_type()
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

// IRpc_respContext is an interface to support dynamic dispatch.
type IRpc_respContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Value_type() IValue_typeContext

	// IsRpc_respContext differentiates from other interfaces.
	IsRpc_respContext()
}

type Rpc_respContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRpc_respContext() *Rpc_respContext {
	var p = new(Rpc_respContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_resp
	return p
}

func InitEmptyRpc_respContext(p *Rpc_respContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_resp
}

func (*Rpc_respContext) IsRpc_respContext() {}

func NewRpc_respContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Rpc_respContext {
	var p = new(Rpc_respContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_rpc_resp

	return p
}

func (s *Rpc_respContext) GetParser() antlr.Parser { return s.parser }

func (s *Rpc_respContext) Value_type() IValue_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValue_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValue_typeContext)
}

func (s *Rpc_respContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Rpc_respContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Rpc_respContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterRpc_resp(s)
	}
}

func (s *Rpc_respContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitRpc_resp(s)
	}
}

func (p *TParser) Rpc_resp() (localctx IRpc_respContext) {
	localctx = NewRpc_respContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, TParserRULE_rpc_resp)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(197)
		p.Value_type()
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

// IRpc_annotationsContext is an interface to support dynamic dispatch.
type IRpc_annotationsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LEFT_BRACE() antlr.TerminalNode
	RIGHT_BRACE() antlr.TerminalNode
	AllTerminator() []ITerminatorContext
	Terminator(i int) ITerminatorContext
	AllAnnotation() []IAnnotationContext
	Annotation(i int) IAnnotationContext

	// IsRpc_annotationsContext differentiates from other interfaces.
	IsRpc_annotationsContext()
}

type Rpc_annotationsContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRpc_annotationsContext() *Rpc_annotationsContext {
	var p = new(Rpc_annotationsContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_annotations
	return p
}

func InitEmptyRpc_annotationsContext(p *Rpc_annotationsContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_rpc_annotations
}

func (*Rpc_annotationsContext) IsRpc_annotationsContext() {}

func NewRpc_annotationsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Rpc_annotationsContext {
	var p = new(Rpc_annotationsContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_rpc_annotations

	return p
}

func (s *Rpc_annotationsContext) GetParser() antlr.Parser { return s.parser }

func (s *Rpc_annotationsContext) LEFT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserLEFT_BRACE, 0)
}

func (s *Rpc_annotationsContext) RIGHT_BRACE() antlr.TerminalNode {
	return s.GetToken(TParserRIGHT_BRACE, 0)
}

func (s *Rpc_annotationsContext) AllTerminator() []ITerminatorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITerminatorContext); ok {
			len++
		}
	}

	tst := make([]ITerminatorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITerminatorContext); ok {
			tst[i] = t.(ITerminatorContext)
			i++
		}
	}

	return tst
}

func (s *Rpc_annotationsContext) Terminator(i int) ITerminatorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITerminatorContext); ok {
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

	return t.(ITerminatorContext)
}

func (s *Rpc_annotationsContext) AllAnnotation() []IAnnotationContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IAnnotationContext); ok {
			len++
		}
	}

	tst := make([]IAnnotationContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IAnnotationContext); ok {
			tst[i] = t.(IAnnotationContext)
			i++
		}
	}

	return tst
}

func (s *Rpc_annotationsContext) Annotation(i int) IAnnotationContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IAnnotationContext); ok {
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

	return t.(IAnnotationContext)
}

func (s *Rpc_annotationsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Rpc_annotationsContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Rpc_annotationsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterRpc_annotations(s)
	}
}

func (s *Rpc_annotationsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitRpc_annotations(s)
	}
}

func (p *TParser) Rpc_annotations() (localctx IRpc_annotationsContext) {
	localctx = NewRpc_annotationsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, TParserRULE_rpc_annotations)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(199)
		p.Match(TParserLEFT_BRACE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(201)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 23, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(200)
			p.Terminator()
		}

	} else if p.HasError() { // JIM
		goto errorExit
	}
	p.SetState(208)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == TParserIDENTIFIER {
		{
			p.SetState(203)
			p.Annotation()
		}
		{
			p.SetState(204)
			p.Terminator()
		}

		p.SetState(210)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	p.SetState(212)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserNEWLINE {
		{
			p.SetState(211)
			p.Terminator()
		}

	}
	{
		p.SetState(214)
		p.Match(TParserRIGHT_BRACE)
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

// IAnnotationContext is an interface to support dynamic dispatch.
type IAnnotationContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	EQUAL() antlr.TerminalNode
	Const_value() IConst_valueContext

	// IsAnnotationContext differentiates from other interfaces.
	IsAnnotationContext()
}

type AnnotationContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAnnotationContext() *AnnotationContext {
	var p = new(AnnotationContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_annotation
	return p
}

func InitEmptyAnnotationContext(p *AnnotationContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_annotation
}

func (*AnnotationContext) IsAnnotationContext() {}

func NewAnnotationContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AnnotationContext {
	var p = new(AnnotationContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_annotation

	return p
}

func (s *AnnotationContext) GetParser() antlr.Parser { return s.parser }

func (s *AnnotationContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *AnnotationContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(TParserEQUAL, 0)
}

func (s *AnnotationContext) Const_value() IConst_valueContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConst_valueContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConst_valueContext)
}

func (s *AnnotationContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AnnotationContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *AnnotationContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterAnnotation(s)
	}
}

func (s *AnnotationContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitAnnotation(s)
	}
}

func (p *TParser) Annotation() (localctx IAnnotationContext) {
	localctx = NewAnnotationContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, TParserRULE_annotation)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(216)
		p.Match(TParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(219)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == TParserEQUAL {
		{
			p.SetState(217)
			p.Match(TParserEQUAL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(218)
			p.Const_value()
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

// IBase_typeContext is an interface to support dynamic dispatch.
type IBase_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	TYPE_BOOL() antlr.TerminalNode
	TYPE_INT() antlr.TerminalNode
	TYPE_FLOAT() antlr.TerminalNode
	TYPE_STRING() antlr.TerminalNode

	// IsBase_typeContext differentiates from other interfaces.
	IsBase_typeContext()
}

type Base_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyBase_typeContext() *Base_typeContext {
	var p = new(Base_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_base_type
	return p
}

func InitEmptyBase_typeContext(p *Base_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_base_type
}

func (*Base_typeContext) IsBase_typeContext() {}

func NewBase_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Base_typeContext {
	var p = new(Base_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_base_type

	return p
}

func (s *Base_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *Base_typeContext) TYPE_BOOL() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_BOOL, 0)
}

func (s *Base_typeContext) TYPE_INT() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_INT, 0)
}

func (s *Base_typeContext) TYPE_FLOAT() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_FLOAT, 0)
}

func (s *Base_typeContext) TYPE_STRING() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_STRING, 0)
}

func (s *Base_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Base_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Base_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterBase_type(s)
	}
}

func (s *Base_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitBase_type(s)
	}
}

func (p *TParser) Base_type() (localctx IBase_typeContext) {
	localctx = NewBase_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 32, TParserRULE_base_type)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(221)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&61440) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
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

// IUser_typeContext is an interface to support dynamic dispatch.
type IUser_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode

	// IsUser_typeContext differentiates from other interfaces.
	IsUser_typeContext()
}

type User_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUser_typeContext() *User_typeContext {
	var p = new(User_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_user_type
	return p
}

func InitEmptyUser_typeContext(p *User_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_user_type
}

func (*User_typeContext) IsUser_typeContext() {}

func NewUser_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *User_typeContext {
	var p = new(User_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_user_type

	return p
}

func (s *User_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *User_typeContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *User_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *User_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *User_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterUser_type(s)
	}
}

func (s *User_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitUser_type(s)
	}
}

func (p *TParser) User_type() (localctx IUser_typeContext) {
	localctx = NewUser_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, TParserRULE_user_type)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(223)
		p.Match(TParserIDENTIFIER)
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

// IContainer_typeContext is an interface to support dynamic dispatch.
type IContainer_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Map_type() IMap_typeContext
	List_type() IList_typeContext

	// IsContainer_typeContext differentiates from other interfaces.
	IsContainer_typeContext()
}

type Container_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyContainer_typeContext() *Container_typeContext {
	var p = new(Container_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_container_type
	return p
}

func InitEmptyContainer_typeContext(p *Container_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_container_type
}

func (*Container_typeContext) IsContainer_typeContext() {}

func NewContainer_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Container_typeContext {
	var p = new(Container_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_container_type

	return p
}

func (s *Container_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *Container_typeContext) Map_type() IMap_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMap_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMap_typeContext)
}

func (s *Container_typeContext) List_type() IList_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IList_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IList_typeContext)
}

func (s *Container_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Container_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Container_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterContainer_type(s)
	}
}

func (s *Container_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitContainer_type(s)
	}
}

func (p *TParser) Container_type() (localctx IContainer_typeContext) {
	localctx = NewContainer_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 36, TParserRULE_container_type)
	p.SetState(227)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case TParserTYPE_MAP:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(225)
			p.Map_type()
		}

	case TParserTYPE_LIST:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(226)
			p.List_type()
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

// IMap_typeContext is an interface to support dynamic dispatch.
type IMap_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	TYPE_MAP() antlr.TerminalNode
	LESS_THAN() antlr.TerminalNode
	Key_type() IKey_typeContext
	COMMA() antlr.TerminalNode
	Value_type() IValue_typeContext
	GREATER_THAN() antlr.TerminalNode

	// IsMap_typeContext differentiates from other interfaces.
	IsMap_typeContext()
}

type Map_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMap_typeContext() *Map_typeContext {
	var p = new(Map_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_map_type
	return p
}

func InitEmptyMap_typeContext(p *Map_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_map_type
}

func (*Map_typeContext) IsMap_typeContext() {}

func NewMap_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Map_typeContext {
	var p = new(Map_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_map_type

	return p
}

func (s *Map_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *Map_typeContext) TYPE_MAP() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_MAP, 0)
}

func (s *Map_typeContext) LESS_THAN() antlr.TerminalNode {
	return s.GetToken(TParserLESS_THAN, 0)
}

func (s *Map_typeContext) Key_type() IKey_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IKey_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IKey_typeContext)
}

func (s *Map_typeContext) COMMA() antlr.TerminalNode {
	return s.GetToken(TParserCOMMA, 0)
}

func (s *Map_typeContext) Value_type() IValue_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValue_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValue_typeContext)
}

func (s *Map_typeContext) GREATER_THAN() antlr.TerminalNode {
	return s.GetToken(TParserGREATER_THAN, 0)
}

func (s *Map_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Map_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Map_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterMap_type(s)
	}
}

func (s *Map_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitMap_type(s)
	}
}

func (p *TParser) Map_type() (localctx IMap_typeContext) {
	localctx = NewMap_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 38, TParserRULE_map_type)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(229)
		p.Match(TParserTYPE_MAP)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(230)
		p.Match(TParserLESS_THAN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(231)
		p.Key_type()
	}
	{
		p.SetState(232)
		p.Match(TParserCOMMA)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(233)
		p.Value_type()
	}
	{
		p.SetState(234)
		p.Match(TParserGREATER_THAN)
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

// IKey_typeContext is an interface to support dynamic dispatch.
type IKey_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	TYPE_STRING() antlr.TerminalNode
	TYPE_INT() antlr.TerminalNode

	// IsKey_typeContext differentiates from other interfaces.
	IsKey_typeContext()
}

type Key_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyKey_typeContext() *Key_typeContext {
	var p = new(Key_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_key_type
	return p
}

func InitEmptyKey_typeContext(p *Key_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_key_type
}

func (*Key_typeContext) IsKey_typeContext() {}

func NewKey_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Key_typeContext {
	var p = new(Key_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_key_type

	return p
}

func (s *Key_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *Key_typeContext) TYPE_STRING() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_STRING, 0)
}

func (s *Key_typeContext) TYPE_INT() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_INT, 0)
}

func (s *Key_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Key_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Key_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterKey_type(s)
	}
}

func (s *Key_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitKey_type(s)
	}
}

func (p *TParser) Key_type() (localctx IKey_typeContext) {
	localctx = NewKey_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 40, TParserRULE_key_type)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(236)
		_la = p.GetTokenStream().LA(1)

		if !(_la == TParserTYPE_INT || _la == TParserTYPE_STRING) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
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

// IList_typeContext is an interface to support dynamic dispatch.
type IList_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	TYPE_LIST() antlr.TerminalNode
	LESS_THAN() antlr.TerminalNode
	Value_type() IValue_typeContext
	GREATER_THAN() antlr.TerminalNode

	// IsList_typeContext differentiates from other interfaces.
	IsList_typeContext()
}

type List_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyList_typeContext() *List_typeContext {
	var p = new(List_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_list_type
	return p
}

func InitEmptyList_typeContext(p *List_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_list_type
}

func (*List_typeContext) IsList_typeContext() {}

func NewList_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *List_typeContext {
	var p = new(List_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_list_type

	return p
}

func (s *List_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *List_typeContext) TYPE_LIST() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_LIST, 0)
}

func (s *List_typeContext) LESS_THAN() antlr.TerminalNode {
	return s.GetToken(TParserLESS_THAN, 0)
}

func (s *List_typeContext) Value_type() IValue_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValue_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValue_typeContext)
}

func (s *List_typeContext) GREATER_THAN() antlr.TerminalNode {
	return s.GetToken(TParserGREATER_THAN, 0)
}

func (s *List_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *List_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *List_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterList_type(s)
	}
}

func (s *List_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitList_type(s)
	}
}

func (p *TParser) List_type() (localctx IList_typeContext) {
	localctx = NewList_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 42, TParserRULE_list_type)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(238)
		p.Match(TParserTYPE_LIST)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(239)
		p.Match(TParserLESS_THAN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(240)
		p.Value_type()
	}
	{
		p.SetState(241)
		p.Match(TParserGREATER_THAN)
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

// IValue_typeContext is an interface to support dynamic dispatch.
type IValue_typeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Base_type() IBase_typeContext
	User_type() IUser_typeContext
	Container_type() IContainer_typeContext
	TYPE_BYTES() antlr.TerminalNode

	// IsValue_typeContext differentiates from other interfaces.
	IsValue_typeContext()
}

type Value_typeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyValue_typeContext() *Value_typeContext {
	var p = new(Value_typeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_value_type
	return p
}

func InitEmptyValue_typeContext(p *Value_typeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_value_type
}

func (*Value_typeContext) IsValue_typeContext() {}

func NewValue_typeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Value_typeContext {
	var p = new(Value_typeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_value_type

	return p
}

func (s *Value_typeContext) GetParser() antlr.Parser { return s.parser }

func (s *Value_typeContext) Base_type() IBase_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBase_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBase_typeContext)
}

func (s *Value_typeContext) User_type() IUser_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUser_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUser_typeContext)
}

func (s *Value_typeContext) Container_type() IContainer_typeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IContainer_typeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IContainer_typeContext)
}

func (s *Value_typeContext) TYPE_BYTES() antlr.TerminalNode {
	return s.GetToken(TParserTYPE_BYTES, 0)
}

func (s *Value_typeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Value_typeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Value_typeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterValue_type(s)
	}
}

func (s *Value_typeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitValue_type(s)
	}
}

func (p *TParser) Value_type() (localctx IValue_typeContext) {
	localctx = NewValue_typeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 44, TParserRULE_value_type)
	p.SetState(247)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case TParserTYPE_BOOL, TParserTYPE_INT, TParserTYPE_FLOAT, TParserTYPE_STRING:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(243)
			p.Base_type()
		}

	case TParserIDENTIFIER:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(244)
			p.User_type()
		}

	case TParserTYPE_MAP, TParserTYPE_LIST:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(245)
			p.Container_type()
		}

	case TParserTYPE_BYTES:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(246)
			p.Match(TParserTYPE_BYTES)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
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

// IConst_valueContext is an interface to support dynamic dispatch.
type IConst_valueContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	KW_TRUE() antlr.TerminalNode
	KW_FALSE() antlr.TerminalNode
	INTEGER() antlr.TerminalNode
	FLOAT() antlr.TerminalNode
	STRING() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode

	// IsConst_valueContext differentiates from other interfaces.
	IsConst_valueContext()
}

type Const_valueContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyConst_valueContext() *Const_valueContext {
	var p = new(Const_valueContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_const_value
	return p
}

func InitEmptyConst_valueContext(p *Const_valueContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_const_value
}

func (*Const_valueContext) IsConst_valueContext() {}

func NewConst_valueContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *Const_valueContext {
	var p = new(Const_valueContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_const_value

	return p
}

func (s *Const_valueContext) GetParser() antlr.Parser { return s.parser }

func (s *Const_valueContext) KW_TRUE() antlr.TerminalNode {
	return s.GetToken(TParserKW_TRUE, 0)
}

func (s *Const_valueContext) KW_FALSE() antlr.TerminalNode {
	return s.GetToken(TParserKW_FALSE, 0)
}

func (s *Const_valueContext) INTEGER() antlr.TerminalNode {
	return s.GetToken(TParserINTEGER, 0)
}

func (s *Const_valueContext) FLOAT() antlr.TerminalNode {
	return s.GetToken(TParserFLOAT, 0)
}

func (s *Const_valueContext) STRING() antlr.TerminalNode {
	return s.GetToken(TParserSTRING, 0)
}

func (s *Const_valueContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(TParserIDENTIFIER, 0)
}

func (s *Const_valueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *Const_valueContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *Const_valueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterConst_value(s)
	}
}

func (s *Const_valueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitConst_value(s)
	}
}

func (p *TParser) Const_value() (localctx IConst_valueContext) {
	localctx = NewConst_valueContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 46, TParserRULE_const_value)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(249)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2013266688) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
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

// ITerminatorContext is an interface to support dynamic dispatch.
type ITerminatorContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllNEWLINE() []antlr.TerminalNode
	NEWLINE(i int) antlr.TerminalNode

	// IsTerminatorContext differentiates from other interfaces.
	IsTerminatorContext()
}

type TerminatorContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTerminatorContext() *TerminatorContext {
	var p = new(TerminatorContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_terminator
	return p
}

func InitEmptyTerminatorContext(p *TerminatorContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = TParserRULE_terminator
}

func (*TerminatorContext) IsTerminatorContext() {}

func NewTerminatorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TerminatorContext {
	var p = new(TerminatorContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = TParserRULE_terminator

	return p
}

func (s *TerminatorContext) GetParser() antlr.Parser { return s.parser }

func (s *TerminatorContext) AllNEWLINE() []antlr.TerminalNode {
	return s.GetTokens(TParserNEWLINE)
}

func (s *TerminatorContext) NEWLINE(i int) antlr.TerminalNode {
	return s.GetToken(TParserNEWLINE, i)
}

func (s *TerminatorContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TerminatorContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TerminatorContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.EnterTerminator(s)
	}
}

func (s *TerminatorContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(TParserListener); ok {
		listenerT.ExitTerminator(s)
	}
}

func (p *TParser) Terminator() (localctx ITerminatorContext) {
	localctx = NewTerminatorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 48, TParserRULE_terminator)
	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(252)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = 1
	for ok := true; ok; ok = _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		switch _alt {
		case 1:
			{
				p.SetState(251)
				p.Match(TParserNEWLINE)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		default:
			p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			goto errorExit
		}

		p.SetState(254)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 29, p.GetParserRuleContext())
		if p.HasError() {
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
