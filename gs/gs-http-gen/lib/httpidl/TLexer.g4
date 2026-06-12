// --------------------
// Lexer Grammar
// --------------------
lexer grammar TLexer;

// Define additional channels for whitespace and comments.
channels {WS_CHAN, SL_COMMENT_CHAN, ML_COMMENT_CHAN}

// --------------------
// Keywords
// --------------------
KW_EXTENDS  : 'extends';
KW_CONST    : 'const';
KW_ENUM     : 'enum';
KW_TYPE     : 'type';
KW_ONEOF    : 'oneof';
KW_RPC      : 'rpc';
KW_SSE      : 'sse';
KW_TRUE     : 'true';
KW_FALSE    : 'false';
KW_OPTIONAL : 'optional';
KW_REQUIRED : 'required';

// --------------------
// Basic types
// --------------------
TYPE_BOOL   : 'bool';
TYPE_INT    : 'int';
TYPE_FLOAT  : 'float';
TYPE_STRING : 'string';
TYPE_BYTES  : 'bytes';

// --------------------
// Container types
// --------------------
TYPE_MAP  : 'map';
TYPE_LIST : 'list';

// --------------------
// Special symbols
// --------------------
LESS_THAN    : '<';
GREATER_THAN : '>';
LEFT_PAREN   : '(';
RIGHT_PAREN  : ')';
LEFT_BRACE   : '{';
RIGHT_BRACE  : '}';
EQUAL        : '=';
COMMA        : ',';

// --------------------
// String literal
// A double-quoted string supporting escape sequences.
// Examples: "hello", "escaped \" quote".
// --------------------
STRING
    : '"' ( '\\' . | ~["\\] )* '"'
    ;

// --------------------
// Identifier
// Begins with a letter; may contain letters, digits, underscores,
// or dots (supporting namespaced identifiers).
// --------------------
IDENTIFIER
    : LETTER (LETTER | DIGIT | '.' | '_')*
    ;

// --------------------
// Integer literal
// Supports signed decimal and unsigned hexadecimal integers.
// Examples: 42, -17, +8, 0x1A2B.
// --------------------
INTEGER
    : ('+' | '-')? DIGIT+ | '0x' HEX_DIGIT+
    ;

// --------------------
// Floating-point number
// Supports integer+fraction parts and scientific notation.
// Examples: 1.23, .5, -3.14e+10.
// --------------------
FLOAT
    : ('+' | '-')? ( DIGIT+ ('.' DIGIT+)? | '.' DIGIT+ ) (('E' | 'e') ('+'|'-')? DIGIT+ )?
    ;

// --------------------
// Fragments (used internally, not emitted as tokens)
// --------------------
fragment DIGIT     : '0'..'9';
fragment LETTER    : 'A'..'Z' | 'a'..'z';
fragment HEX_DIGIT : DIGIT | 'A'..'F' | 'a'..'f';

// --------------------
// Newline
// --------------------
NEWLINE
    : '\r'? '\n'
    ;

// --------------------
// Whitespace
// --------------------
WHITESPACE
    : [ \t]+ -> channel(WS_CHAN)
    ;

// --------------------
// Single-line comments
// Supports '//' and '#' styles.
// --------------------
SINGLE_LINE_COMMENT
    : ('//' | '#') ~[\r\n]* -> channel(SL_COMMENT_CHAN)
    ;

// --------------------
// Multi-line comments
// Supports C-style block comments using non-greedy matching.
// --------------------
MULTI_LINE_COMMENT
    : '/*' .*? '*/' -> channel(ML_COMMENT_CHAN)
    ;
