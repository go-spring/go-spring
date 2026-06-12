# gs-mock

<div>
   <img src="https://img.shields.io/github/license/go-spring/gs-mock" alt="license"/>
   <a href="https://codecov.io/gh/go-spring/gs-mock" > 
      <img src="https://codecov.io/gh/go-spring/gs-mock/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://deepwiki.com/go-spring/gs-mock"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`gs-mock` is a modern, type-safe Go mock library with **full support for generics**.
It addresses the shortcomings of traditional Go mock tools in terms of **type safety** and **usability**,
and natively supports concurrent testing through `context.Context` propagation.

`gs-mock` supports mocking the following targets:

* Interfaces (via code generation)
* Plain functions
* Struct methods

It is especially suitable for **unit testing** and **component testing** in microservice architectures.

## Features

### Type System and Language Features

* **Type Safety & Generics Support**

    * Native support for generic interfaces and generic functions
    * Full type inference and auto-completion provided by IDEs

* **Multiple Parameters and Multiple Return Values**

    * Supports up to **7 parameters**
    * Supports up to **4 return values**
    * Covers the vast majority of real-world business function signatures

### Mocking Modes

* **Handle Mode**

    * Encapsulates the entire mock logic in a single callback
    * Suitable for scenarios with complex conditional logic

* **When / Return Mode**

    * Returns results based on condition matching
    * Better suited for declarative and highly readable mock configurations

### Mock Target Types

* **Interface Mocking**

    * Automatically generates mock implementations via code generation
    * No reliance on `context.Context` parameter

* **Plain Function & Struct Method Mocking**

    * Propagates mock configuration through `context.Context`
    * No need to introduce additional interfaces for functions or methods

### Concurrency and Context

* **Concurrent Testing Support**

    * Passes the Mock Manager via `context.Context`
    * Ensures isolation and safety of mocks in concurrent test scenarios

## Installation

### Standalone Installation

```
go install github.com/go-spring/gs-mock@latest
```

### Installation via the gs Toolchain

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"
```

## Quick Start

### 1. Interface Mocking

#### 1. Define an Interface

```
type Service interface {
    Do(n int, s string) (int, error)
    Format(s string, args ...any) string
}
```

#### 2. Generate Mock Code

```
//go:generate gs mock -o src_mock.go
```

> `gs mock` indicates using the `mock` subcommand from the `gs` toolchain.
> You may also directly use the `gs-mock` command.

After adding the directive at the top of the interface file, mock code will be generated for **all interfaces in the
current package**. If you only want to generate mocks for specific interfaces, use the `-i` option:

```
//go:generate gs-mock -o src_mock.go -i '!RepositoryV2,Repository'
```

**Examples of the `-i` option:**

* `-i 'Repository'`
  Generate a mock only for the `Repository` interface
* `-i '!Repository'`
  Generate mocks for all interfaces except `Repository`
* `-i 'Repository,Service'`
  Generate mocks only for `Repository` and `Service`
* `-i '!Repository,Service'`
  Generate mocks for all interfaces except `Repository`, but include `Service`

#### 3. Using Mocks (Handle Mode)

```
r := gsmock.NewManager()
s := NewServiceMockImpl(r)

// Handle mode: determine return logic based on input arguments
s.MockDo().Handle(func (n int, s string) (int, error) {
    if n%2 == 0 {
        return n * 2, nil
    }
    return n + 1, errors.New("error")
})

fmt.Println(s.Do(1, "abc")) // 2 error
fmt.Println(s.Do(2, "abc")) // 4 <nil>
```

#### 4. Using Mocks (When / Return Mode)

```
r := gsmock.NewManager()
s := NewServiceMockImpl(r)

// For args[0] == "abc"; variadic arguments are represented as a slice
s.MockFormat().When(func (s string, args []any) bool {
    return args[0] == "abc"
}).ReturnValue("abc")

// For args[0] == "123"; variadic arguments are represented as a slice
s.MockFormat().When(func (s string, args []any) bool {
    return args[0] == "123"
}).ReturnValue("123")

fmt.Println(s.Format("", "abc", "123")) // abc
fmt.Println(s.Format("", "123", "abc")) // 123
fmt.Println(s.Format("", "xyz", "abc")) // panic: no matching mock found
```

> **Notes**
>
> * Do not mix `Handle` mode and `When/Return` mode on the same method
> * When multiple `When/Return` configurations exist, they are matched in registration order; the first successful match
    is executed

### 2. Function Mocking

#### 1. Define a Plain Function

```
//go:noinline // Prevent the function from being inlined
func Do(ctx context.Context, n int) int { return n }
```

#### 2. Mock the Function

```
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

// Return a fixed value using ReturnValue
gsmock.Func21(Do, r).ReturnValue(2)

fmt.Println(Do(ctx, 1)) // 2
```

**Explanation:**

* Requirements for mocking plain functions:

    * The first parameter must be `context.Context`
    * Mock configuration is propagated via the context chain

* `Func21` means:

    * 2 parameters
    * 1 return value

* For variadic functions, use the `VarFuncNN` series, such as `VarFunc21`

### 3. Struct Method Mocking

#### 1. Define a Struct Method

```
type Service struct{ m int }

func (s *Service) Do(ctx context.Context, n int) int {
    return n
}
```

#### 2. Mock the Struct Method

```
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

// The first parameter becomes the receiver type, followed by the original method parameters
gsmock.Func31((*Service).Do, r).Handle(func(s *Service, ctx context.Context, n int) int {
    return n + s.m
})

fmt.Println((&Service{m: 1}).Do(ctx, 1)) // 2
fmt.Println((&Service{m: 2}).Do(ctx, 1)) // 3
```

**Notes:**

* Use a **method expression** (e.g. `(*Service).Do`) instead of a method value (e.g. `s.Do`)
* The receiver becomes the **first parameter** of the mock callback; `ctx` becomes the **second parameter**
* Tests must be run with `-gcflags="all=-N -l"` to prevent method inlining

> More examples and usage can be found in the [example](example) directory.

## FAQ

### 1. Mock Failure Due to Inlining Optimization

* **Problem**:
  In some cases, functions or methods are inlined by the Go compiler, causing mock logic not to be triggered.

* **Solution**:
  Explicitly disable compiler optimizations when running tests:

  ```
  go test -gcflags="all=-N -l" ./...
  ```

* **Explanation**:
  This disables inlining and certain optimizations, ensuring the mock framework can correctly intercept calls.

### 2. Context Parameter Requirement

* **Problem**:
  When mocking plain functions or struct methods, the first parameter must be `context.Context`.

* **Solution**:
  When designing testable functions, include `context.Context` as the first parameter to allow mock manager propagation.

* **Notes**:

    * This restriction **only applies to plain functions and struct method mocking**
    * **Interface mocks do not require** `context.Context` in method signatures

### 3. When / Return Registration Order

* **Problem**:
  When multiple `When/Return` rules are registered for the same method, matching order affects the outcome.

* **Solution**:
  **The first successfully matched rule is executed**, so register rules from **most specific to most general**.

### 4. Manager Scope and Concurrency Safety

* **Problem**:
  The `Mock Manager` itself is not goroutine-safe. Registering mocks dynamically during concurrent execution may cause
  unpredictable behavior.

* **Solution**:
  All mock registrations must be completed **before any concurrent logic starts**, and the manager should be passed to
  goroutines via `context.Context`.

### 5. Mocking Variadic Functions

* **Problem**:
  Variadic functions (e.g. `Printf(format string, args ...any)`) have different parameter structures during mocking and
  require special handling.

* **Solution**:
  Use the `VarFuncNN` series to mock variadic functions.

* **Explanation**:

    * Variadic arguments are wrapped as a single slice parameter in the mock callback
    * All variadic mock types are prefixed with `Var`

## License

This project is licensed under the Apache License Version 2.0.
