// --------------------
// Lexer Grammar
// --------------------
lexer grammar VLexer;

channels { WS_CHAN }

// --------------------
// Keywords
// --------------------
KW_DOLLAR : '$' ;
KW_NIL    : 'nil' ;

// --------------------
// Comparison operators
// --------------------
EQUAL            : '==' ;
NOT_EQUAL        : '!=' ;
LESS_THAN        : '<'  ;
GREATER_THAN     : '>'  ;
LESS_OR_EQUAL    : '<=' ;
GREATER_OR_EQUAL : '>=' ;

// --------------------
// Logical operators
// --------------------
LOGICAL_AND : '&&' ;
LOGICAL_OR  : '||' ;
LOGICAL_NOT : '!'  ;

// --------------------
// Delimiters
// --------------------
LEFT_PAREN  : '(' ;
RIGHT_PAREN : ')' ;
COMMA       : ',' ;

// --------------------
// String literal
// Single-quoted string
// Supports escape sequences (e.g., \' for quote, \\ for backslash)
// --------------------
STRING
    : '\'' ( '\\' . | ~['\\] )* '\''
    ;

// --------------------
// Integer literal
// Decimal integer with optional sign (+/-) or hexadecimal integer prefixed with 0x.
// --------------------
INTEGER
    : ('+' | '-')? DIGIT+ | '0x' HEX_DIGIT+
    ;

// --------------------
// Floating-point number
// Supports decimals and scientific notation (e.g., 1.23e+10)
// --------------------
FLOAT
    : ('+' | '-')? ( DIGIT+ ('.' DIGIT+)? | '.' DIGIT+ ) (('E' | 'e') ('+'|'-')? DIGIT+ )?
    ;

// --------------------
// Identifier
// - Start with a letter or underscore.
// - May contain letters, digits, or underscores.
// --------------------
IDENTIFIER
    : (LETTER | '_') (LETTER | DIGIT | '_')*
    ;

// --------------------
// Fragments (used internally, not emitted as tokens)
// --------------------
fragment DIGIT     : '0'..'9' ;
fragment LETTER    : 'A'..'Z' | 'a'..'z' ;
fragment HEX_DIGIT : DIGIT | 'A'..'F' | 'a'..'f' ;

// --------------------
// Whitespace
// Skipped by sending to WS_CHAN
// --------------------
WHITESPACE
    : [ \t\r\n]+ -> channel(WS_CHAN)
    ;
