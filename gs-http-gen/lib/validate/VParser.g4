// --------------------
// Parser Grammar
// --------------------
parser grammar VParser;

options { tokenVocab = VLexer; }

// --------------------
// Entry point
// --------------------
validateExpr
    : logicalOrExpr
    ;

// --------------------
// Operator precedence (lowest precedence first)
// --------------------

// Logical OR (lowest precedence)
logicalOrExpr
    : logicalAndExpr (LOGICAL_OR logicalAndExpr)*
    ;

// Logical AND
logicalAndExpr
    : equalityExpr (LOGICAL_AND equalityExpr)*
    ;

// Equality / Inequality
equalityExpr
    : relationalExpr ((EQUAL | NOT_EQUAL) relationalExpr)?
    ;

// Relational operators (<, <=, >, >=)
relationalExpr
    : unaryExpr ((LESS_THAN | LESS_OR_EQUAL | GREATER_THAN | GREATER_OR_EQUAL) unaryExpr)?
    ;

// --------------------
// Unary expressions
// --------------------
unaryExpr
    : LOGICAL_NOT unaryExpr
    | primaryExpr
    ;

// --------------------
// Primary expressions
// --------------------
primaryExpr
    : IDENTIFIER
    | KW_DOLLAR
    | KW_NIL
    | INTEGER
    | FLOAT
    | STRING
    | functionCall
    | LEFT_PAREN validateExpr RIGHT_PAREN
    ;

// --------------------
// Function call
// --------------------
functionCall
    : IDENTIFIER LEFT_PAREN (validateExpr (COMMA validateExpr)*)? RIGHT_PAREN
    ;
