// --------------------
// Parser Grammar
// --------------------
parser grammar TParser;

options { tokenVocab = TLexer; }

// --------------------
// Document root
// A document consists of zero or more definitions separated by terminators
// and ends with EOF.
// --------------------
document
    : ((definition terminator) | terminator)* EOF
    ;

// --------------------
// Top-level definitions: const, enum, type, oneof, rpc
// --------------------
definition
    : const_def | enum_def | type_def | oneof_def | rpc_def
    ;

// --------------------
// Constant definition
// Example: const string a = "1"
// --------------------
const_def
    : KW_CONST base_type IDENTIFIER EQUAL const_value
    ;

// --------------------
// Enum definition
// Example:
// enum A {
//   RED = 1
//   GREEN = 2
// }
// extends is used only for error code inheritance
// --------------------
enum_def
    : KW_ENUM KW_EXTENDS? IDENTIFIER LEFT_BRACE terminator? (enum_field terminator)* terminator? RIGHT_BRACE
    ;

// --------------------
// Enum field
// Example: RED = 1
// --------------------
enum_field
    : IDENTIFIER EQUAL INTEGER field_annotations?
    ;

// --------------------
// Type definition
// Example 1:
// type A<T> {
//   B
//   T data
//   string field = "1" (go.type="string")
// }
// Example 2:
// type A B<int>
// type A B<map<string,int>>
// type UserResp Response<User>
// --------------------
type_def
    // Structured type with optional generic parameter
    : KW_TYPE IDENTIFIER (LESS_THAN IDENTIFIER GREATER_THAN)? LEFT_BRACE terminator? (type_field terminator)* terminator? RIGHT_BRACE
    // Generic type instantiation or aliasing
    | KW_TYPE IDENTIFIER IDENTIFIER LESS_THAN value_type GREATER_THAN
    ;

// --------------------
// Type field
// A field can be either an embedded user-defined type
// or a normal named field with optional modifiers and annotations.
// --------------------
type_field
    : embed_type_field | common_type_field
    ;

// --------------------
// Embedded type field
// --------------------
embed_type_field
    : user_type
    ;

// --------------------
// Common type field
// --------------------
common_type_field
    : (KW_REQUIRED | KW_OPTIONAL)? value_type IDENTIFIER field_annotations?
    ;

// --------------------
// Field annotations
// Parenthesized list of key-value annotations.
// Commas and/or newlines may be used as separators.
// Example 1:
//   (go.type="string", db.index=true)
// Example 2:
//   (
//       go.type="string"
//       db.index=true
//   )
// --------------------
field_annotations
    : LEFT_PAREN terminator? annotation ((COMMA | terminator) annotation)* terminator? RIGHT_PAREN
    ;

// --------------------
// OneOf definition
// Example:
//   oneof Value {
//       A
//       B
//   }
// --------------------
oneof_def
    : KW_ONEOF IDENTIFIER LEFT_BRACE terminator? (user_type terminator)* terminator? RIGHT_BRACE
    ;

// --------------------
// RPC definition
// Example:
//   rpc GetUser (ReqType) RespType { method="GET" } // standard HTTP request
//   sse GetUser (ReqType) RespType { method="GET" } // streaming HTTP request
// --------------------
rpc_def
    : (KW_RPC | KW_SSE) IDENTIFIER LEFT_PAREN rpc_req RIGHT_PAREN rpc_resp rpc_annotations
    ;

// --------------------
// RPC request type
// --------------------
rpc_req
    : user_type
    ;

// --------------------
// RPC response type
// --------------------
rpc_resp
    : value_type
    ;

// --------------------
// RPC annotations
// Contains zero or more annotation entries separated by terminators.
// --------------------
rpc_annotations
    : LEFT_BRACE terminator? (annotation terminator)* terminator? RIGHT_BRACE
    ;

// --------------------
// Annotation, key-value pair
// --------------------
annotation
    : IDENTIFIER (EQUAL const_value)?
    ;

// --------------------
// Base types
// --------------------
base_type
    : TYPE_BOOL | TYPE_INT | TYPE_FLOAT | TYPE_STRING
    ;

// --------------------
// User-defined type
// --------------------
user_type
    : IDENTIFIER
    ;

// --------------------
// Container types
// Supported: map<K,V> and list<T>
// --------------------
container_type
    : map_type | list_type
    ;

// --------------------
// Map type
// Keys are restricted to string and int.
// --------------------
map_type
   : TYPE_MAP LESS_THAN key_type COMMA value_type GREATER_THAN
   ;

// --------------------
// Allowed map key types
// --------------------
key_type
    : TYPE_STRING | TYPE_INT
    ;

// --------------------
// List type
// --------------------
list_type
   : TYPE_LIST LESS_THAN value_type GREATER_THAN
   ;

// --------------------
// Allowed container element/value types
// --------------------
value_type
    : base_type | user_type | container_type | TYPE_BYTES
    ;

// --------------------
// Constant values
// Can be literal primitives or an identifier (e.g., enum member).
// --------------------
const_value
    : KW_TRUE | KW_FALSE | INTEGER | FLOAT | STRING | IDENTIFIER
    ;

// --------------------
// Terminator
// One or more NEWLINE tokens.
// Used to separate statements and fields.
// --------------------
terminator
    : (NEWLINE)+
    ;
