grammar Expr;

// ----------------------------------
// Lexer Rules
// ----------------------------------

// Identifier for type names, field names, or symbolic constants
// Examples: MyType, field_name, CONSTANT
IDENT : [a-zA-Z_][a-zA-Z0-9_]* ;

// String literal: double-quoted string with optional escape sequences
// Examples: "hello", "line\nbreak"
STRING
    : '"' ( ~["\\] | '\\' ["\\/bfnrt] )* '"'
    ;

// Integer literal: optional sign, decimal or hexadecimal
// Examples: 42, -17, +0xFF
INTEGER
    : ('+' | '-')? DIGIT+ | '0x' HEX_DIGIT+
    ;

// Floating-point number: optional sign, decimal with optional fraction and exponent
// Examples: 3.14, -0.5, +2E10, .25e-2
FLOAT
    : ('+' | '-')? ( DIGIT+ ('.' DIGIT+)? | '.' DIGIT+ ) (('E' | 'e') ('+'|'-')? DIGIT+ )?
    ;

// Fragments
fragment DIGIT     : '0'..'9';
fragment LETTER    : 'A'..'Z' | 'a'..'z';
fragment HEX_DIGIT : DIGIT | 'A'..'F' | 'a'..'f';

// Whitespace (spaces, tabs, newlines) are skipped
WS : [ \t\r\n]+ -> skip ;

// ----------------------------------
// Parser Rules
// ----------------------------------

// Root node: entry point of the parser
// Ensures that the entire input is a single expression
root: expr EOF ;

// Main expression: a type name with optional key-value pairs enclosed in braces
// Example: TypeName { field1 = "value1", field2 = NestedType { ... }, field3 = rawValue }
expr
    : IDENT '{' innerExprList? '}'
    ;

// List of key-value assignments inside an expression
// Commas between entries are optional; trailing comma is allowed
// Example: field1 = 1, field2 = 2,
innerExprList
    : innerExpr (',' innerExpr)* ','?
    ;

// Key-value assignment: field is assigned a value
// Example: foo = "bar"
innerExpr
    : fieldAccess '=' value
    ;

// Field access supports:
// - Simple fields: foo
// - Nested fields: foo.bar
// - Array indices: foo[0]
// - Combined: foo.bar[1].baz
fieldAccess
    : IDENT ('.' IDENT | '[' INTEGER ']')*
    ;

// Value can be:
// - A string literal
// - An identifier (e.g., a symbolic constant or raw value)
// - An integer or floating-point number
// - A nested expression (type with braces)
// Examples: "hello", true, 42, 3.14, NestedType { ... }
value
    : IDENT | STRING | INTEGER | FLOAT | expr ;
