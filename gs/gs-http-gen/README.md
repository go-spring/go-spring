# gs-http-gen

<div>
   <img src="https://img.shields.io/github/license/go-spring/gs-http-gen" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/gs-http-gen" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/gs-http-gen?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/gs-http-gen" > 
      <img src="https://codecov.io/gh/go-spring/gs-http-gen/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://deepwiki.com/go-spring/gs-http-gen"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> This project is under continuous development, and its features and capabilities are being actively enhanced.

`gs-http-gen` is an **IDL (Interface Definition Language)-based HTTP code generation tool**.
It can generate **Go server-side code** and **client-side code in other languages** based on unified interface
definitions. The server-side code includes:

* Data models
* Validation logic
* HTTP route binding
* Support for both regular and streaming (SSE) interfaces

By using a declarative IDL description, developers can focus on business logic while reducing boilerplate code
and human errors.

Additionally, IDL serves as a **contract and documentation for cross-team and frontend-backend collaboration**,
helping teams reduce communication overhead and ensure interface consistency.

## Features

### 🌟 IDL-Driven

* Define services and data models using a concise interface definition language.
* Supports:

    * Constants, enums, structs, and `oneof` types
    * Generics and type embedding (field reuse)
    * RPC interface definitions
    * Custom annotations (e.g., `json`, `go.type`, `enum_as_string`, etc.)

### ⚙️ Automatic Code Generation

Generate Go server-side code and client code in other languages from IDL files:

* Data model structures
* Parameter and data validation logic
* Automatic HTTP request parameter binding (path, query, body)
* Support for both regular and streaming (SSE) interfaces
* Server interface definitions and route binding
* Client-side call code

### 📦 Rich Data Type Support

* Basic types: `bool`, `int`, `float`, `string`
* Advanced types: `list`, `map`, `oneof`
* Nullable fields: supported via `?` suffix
* Type redefinitions and generics

### 🔎 Efficient Data Validation

* High performance, reflection-free implementation
* Expression-based validation rules
* Auto-generated `OneOfXXX` validation functions for enums
* Custom validation functions supported

### 🌐 HTTP-Friendly

* Automatic binding of HTTP request parameters (path, query, body)
* Supports `form`, `json`, and `multipart-form` formats
* Native support for streaming RPC (SSE) interfaces

### 📝 Comments & Documentation

* Supports single-line and multi-line comments
* Planned support for Markdown comments for richer documentation generation

## Installation

* **Recommended:**

Use the [gs](https://github.com/go-spring/gs) integrated development tool.

* **Standalone installation:**

```bash
go install github.com/go-spring/gs-http-gen@latest
```

## Usage

### Step 1: Define IDL Files

Create `.idl` files to describe your services and data models.

> **Syntax Notes:**
>
> * A document consists of zero or more definitions, separated by newlines or semicolons, and ends with EOF.
> * Identifiers consist of letters, digits, and underscores, but cannot start with a digit.
> * Use `?` to denote nullable fields.

**Example:**

```idl
// Constants
const int MAX_AGE = 150 // years
const int MIN_AGE = 18  // years

// Enums
enum ErrCode {
    ERR_OK = 0
    PARAM_ERROR = 1003
}

enum Department {
    ENGINEERING = 1
    MARKETING = 2
    SALES = 3
}

// Data structures
type Manager {
    string id
    string name (validate="len($) > 0 && len($) <= 64")
    int? age (validate="$ >= MIN_AGE && $ <= MAX_AGE")
    Department dept (enum_as_string)
}

type Response<T> {
    ErrCode errno (validate="OneOfErrCode($)")
    string errmsg
    T data
}

// Request & response types
type ManagerReq {
    string id (path="id")
}

type GetManagerResp Response<Manager?>

// Regular RPC interface
rpc GetManager(ManagerReq) GetManagerResp {
    method="GET"
    path="/managers/{id}"
    summary="Get manager info by ID"
}

// Streaming RPC example
type StreamReq {
    string ID (json="id")
}

type StreamResp {
    string id
    string data
    Payload payload
}

oneof Payload {
    string text_data
    int? numberData (json="number_data")
    bool boolean_data (json="")
}

// Streaming RPC
rpc Stream(StreamReq) stream<StreamResp> {
    method="GET"
    path="/stream/{id}"
    summary="Stream data by ID"
}
```

### Step 2: Generate Code

Run the CLI tool to generate code:

```bash
# Generate server-side code only (default)
gs-http-gen --server --output ./generated --go_package myservice

# Generate both server-side and client-side code
gs-http-gen --server --client --output ./generated --go_package myservice
```

**Command-line options:**

| Option         | Description                                         | Default |
|----------------|-----------------------------------------------------|---------|
| `--server`     | Generate server-side code (HTTP handlers & routing) | false   |
| `--client`     | Generate client-side code (HTTP call wrappers)      | false   |
| `--output`     | Output directory                                    | `.`     |
| `--go_package` | Go package name for generated code                  | `proto` |
| `--language`   | Target language (currently only `go`)               | `go`    |

### Step 3: Use the Generated Code

**Example:**

```go
// Implement the service interface
type MyManagerServer struct{}

func (m *MyManagerServer) GetManager(ctx context.Context, req *proto.ManagerReq) *proto.GetManagerResp {
    // Regular response
    return &proto.GetManagerResp{
        Data: &proto.Manager{
            Id:   "1",
            Name: "Jim",
            Dept: proto.Department_ENGINEERING,
        },
    }
}

func (m *MyManagerServer) Stream(ctx context.Context, req *proto.StreamReq, resp chan<- *proto.StreamResp) {
    // Streaming response
    for i := 0; i < 5; i++ {
        resp <- &proto.StreamResp{
            Id: strconv.Itoa(i),
            Payload: proto.Payload{
                TextData: "data",
            },
        }
    }
}

// Register routes
mux := http.NewServeMux()
proto.InitRouter(mux, &MyManagerServer{})

http.ListenAndServe(":8080", mux)
```

## ⚠️ Notes

* Generated code does **not** enforce required fields; you must handle this in your business logic.
* Validation logic does not automatically invoke `Validate()`; invoke it explicitly as needed for deep validation.
* It’s recommended to manage generated code centrally and keep it in sync with IDL files to avoid divergence.

## License

This project is licensed under the [Apache License 2.0](LICENSE).
