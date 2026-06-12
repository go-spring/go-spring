# ctxcache

[English](README.md) | [中文](README_CN.md)

## Introduction

`ctxcache` is a strongly-typed, context-scoped cache package designed for request-scoped data.

`ctxcache` attaches a concurrency-safe, write-once key-value store to `context.Context`, allowing values to be
implicitly propagated across call boundaries without polluting function signatures.

## Features

- **Strong Type Safety**: Uses generics combining string names and Go type parameters as key identifiers, ensuring type
  safety and preventing collisions between values of different types—even if they share the same string identifier
- **Concurrency Safe**: Internally uses mutex-protected maps, supporting concurrent access
- **Write-Once Semantics**: Each key can be assigned exactly once, then read multiple times until the cache is cleared
- **Context Lifecycle**: Cache lifecycle is explicitly controlled by the cancel function returned from `Init`
- **Request Scope Isolation**: Cache is attached to context, naturally supporting request-level data isolation

## Main Functions

### Initialize Cache

Use the `Init` function to attach a cache to a context:

```go
ctx, cancel := ctxcache.Init(ctx)
defer cancel() // Clean up cache at request boundary
```

`Init` is idempotent: repeated calls with the same context return the original context and a no-op cancel function.

### Set Values

Use the `Set` function to assign a value to a key:

```go
err := ctxcache.Set(ctx, "user", userInfo)
if err != nil {
    // handle error
}
```

Each key can only be set once. Repeated attempts return an `ErrKeyAlreadySet` error.

### Get Values

Use the `Get` function to retrieve a value:

```go
value, err := ctxcache.Get[UserType](ctx, "user")
if err != nil {
    // handle error
}
```

`Get` is a generic function that requires specifying a type parameter to ensure type safety.

### Clear Cache

Calling the cancel function returned by `Init` clears all cached values:

```go
cancel() // Clear cache and make it permanently unusable
```

Once cleared, subsequent `Get` or `Set` operations will return `ErrCacheAlreadyCleared` error.

## Error Types

The package defines the following error types:

- `ErrCacheNotInitialized`: Cache is not initialized
- `ErrCacheAlreadyCleared`: Cache has already been cleared
- `ErrKeyNotSet`: Key is not set
- `ErrKeyAlreadySet`: Key is already set

## Typical Use Cases

1. **HTTP Middleware**: Initialize cache at request entry point and clean up at exit
2. **Authenticated User Info**: Store authenticated user objects
3. **Permission Data**: Pass user permission lists
4. **Trace Metadata**: Carry trace context information
5. **Computed Intermediates**: Share computed intermediate values across call chains

## Example Usage

```go
package main

import (
	"context"
	"fmt"
	"github.com/go-spring/stdlib/ctxcache"
)

type User struct {
	ID   int
	Name string
}

func main() {
	ctx := context.Background()

	// Initialize cache
	ctx, cancel := ctxcache.Init(ctx)
	defer cancel()

	// Set user info
	user := User{ID: 1, Name: "Alice"}
	if err := ctxcache.Set(ctx, "user", user); err != nil {
		panic(err)
	}

	// Get user info in downstream code
	retrievedUser, err := ctxcache.Get[User](ctx, "user")
	if err != nil {
		panic(err)
	}

	fmt.Printf("User: %+v\n", retrievedUser)
}
```

## Notes

- `ctxcache` is not a general-purpose cache. It is designed for structured, short-lived, in-process context data
- Each key can only be assigned once. This is intentional design to ensure data immutability
- The cancel function should be called at request boundaries to ensure request-scoped data is properly cleaned up
- Once cleared, the cache becomes permanently unusable and should not be used again
