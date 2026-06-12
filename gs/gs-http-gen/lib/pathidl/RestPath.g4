grammar RestPath;

// ----------------------
// Top-level rule: a path consists of multiple segments
// ----------------------
path
    : '/' segment ( '/' segment )*
    ;

// ----------------------
// Path segment types
// ----------------------
segment
    : STATIC_SEGMENT       // Static segment
    | paramSegment         // Colon-style :param or :param* (wildcard)
    | bracedParam          // Curly-brace style {param} or {param...} (wildcard)
    ;

// ----------------------
// Static path segment, e.g., "users", "books"
// ----------------------
STATIC_SEGMENT
    : [a-zA-Z0-9_-]+
    ;

// ----------------------
// Colon-style parameter :param or :param* (wildcard)
// ----------------------
paramSegment
    : ':' name=STATIC_SEGMENT (wildcard='*')?
    ;

// ----------------------
// Curly-brace style parameter {param} or {param...} (wildcard)
// ----------------------
bracedParam
    : '{' name=STATIC_SEGMENT (wildcard='...')? '}'
    ;
