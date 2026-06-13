# http-gen

http-gen is a tool that generates Go HTTP server and client code based on IDL definitions.

## Project Structure and Dependencies

A typical http-gen project structure:
```
project/
├── meta.json        # Metadata (package name, output path, etc.)
├── api/             # IDL file directory
│   ├── user.idl
│   └── order.idl
└── ...
```

- Supports the `import` mechanism for introducing external IDL files
- Multiple IDLs share the same namespace

## IDL Syntax and Semantics

### Comments

```idl
// Single-line comment

/*
   Multi-line comment
*/
```

### Keywords

| Keyword | Description |
|--------|------|
| `extends` | Extend enum |
| `const` | Constant definition |
| `enum` | Enum definition |
| `type` | Type definition |
| `oneof` | Union type |
| `rpc` | RPC method definition |
| `sse` | Streaming response (Server-Sent Events) |
| `true`/`false` | Boolean value |
| `optional` | Optional field |
| `required` | Required field |

### Basic Types

| Type | Description |
|------|------|
| `bool` | Boolean |
| `int` | Signed integer (`uint` is not built in, but can be customized) |
| `float` | Floating point number |
| `string` | String |
| `bytes` | Byte array |
| `list` | List slice |
| `map` | Map dictionary |

Multi-level nesting is supported.

### Constants

```idl
const MAX_USERS = 1000;
```

### Annotations

```idl
@route("/api/users")      // Single-line annotation
@method("POST")
```

Syntax: `@key(=value)?`; omitting value means `true` boolean semantics.

### Enums

```idl
enum ErrorCode {
    OK = 0 @errmsg("success")
    NOT_FOUND = 1 @errmsg("not found")
}
```

- Supports the `errmsg` annotation for storing error messages
- Supports the `enum_as_string` option to generate string enums
- Supports `extends` to extend and merge other enums

### Types (Structs)

```idl
type User {
    required int64 id;
    required string username;
    optional string avatar;
    @pattern("^[a-z]+$") string name;
}
```

- Supports regular structs and generic structs
- Supports nested fields and embedded types (field merging)
- Supports union type `oneof` (enum + struct)
- Supports field validation:
  - Built-in validation functions
  - Custom validation functions
  - Operator expressions, where `$` represents the current field value

## API Definition

```idl
rpc CreateUser(CreateUserRequest) returns (CreateUserResponse) @method("POST") @path("/api/users");
```

- Supports regular `rpc` methods
- Supports `sse` streaming response methods
- API annotations support `method`, `path`, `contentType`, `timeout`, and others
- Restful path parameters support `:name` or `{name}` syntax, and only `int`/`string` types are supported

## Code Generator

- Generates Go server skeleton code
- Generates Go client invocation code
- Streaming parsing optimization: supports `required` validation and streaming JSON/form parsing, with better performance

Usage:
```bash
go run go-spring.org/gs-http-gen/cmd/gs-http-gen --config meta.json
```
