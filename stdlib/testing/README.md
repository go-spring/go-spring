# testing

[English](README.md) | [中文](README_CN.md)

Go-Spring Testing is an elegant test assertion library designed for Go, providing a Fluent API style that
makes your test code clearer and more readable.

## Features Overview

- 📝 **Dual-mode support**: Provides both `assert` and `require` modes to meet different scenario requirements
- 💧 **Fluent API**: Method chaining makes code more readable, close to natural language
- 🏷️ **Type safety**: Generics guarantee type safety with compile-time error checking
- 🔧 **Type-specific**: Provides dedicated assertion methods for different data types
- 🧩 **Feature-rich**: Covers the vast majority of assertion needs in daily testing, supporting generic values,
  errors, numbers, strings, slices, maps, panic detection, and more
- ✅ **Zero dependencies**: Only depends on the Go standard library

## assert vs require

Go-Spring Testing provides two packages to meet different testing needs:

### `assert` package

The `assert` package provides assertion functions that **do not terminate test execution when an assertion fails**.

When an assertion fails, the test continues running and subsequent assertions will still be checked.
This is very useful when you want to report multiple failures in a single test run and see all issues at once.

### `require` package

The `require` package provides assertion functions that **immediately stop test execution when an assertion fails**.

When an assertion fails, the test terminates immediately and no further assertions are checked.
This is suitable for scenarios where critical conditions are not met and subsequent assertions may cause panics or other issues.
For example, when you need to verify that an object is non-nil before you can proceed with subsequent operations.

## Basic Example

```go
package main

import (
	"testing"
	"os"
	"math"

	"github.com/go-spring/stdlib/testing/assert"
	"github.com/go-spring/stdlib/testing/require"
)

func TestExample(t *testing.T) {
	// Generic assertions - works with any type
	assert.That(t, "hello").Equal("hello")        // Equality assertion
	assert.That(t, user).NotNil()                 // Non-nil assertion
	assert.That(t, 42).True()                     // Boolean is true

	// Using require - test stops immediately on failure
	require.That(t, user).NotNil()

	// Error assertions
	err := someFunc()
	assert.Error(t, err).NotNil()                 // Expect an error to occur
	assert.Error(t, err).Is(os.IsNotExist)         // Check error type using errors.Is

	// Number assertions
	assert.Number(t, 42).GreaterThan(40)          // Greater than
	assert.Number(t, 100).Between(0, 200)          // Within range
	assert.Number(t, 0).Zero()                     // Equal to zero
	assert.Number(t, 3.14).InDelta(math.Pi, 0.01)  // Floating point comparison with tolerance

	// String assertions
	assert.String(t, "user@example.com").IsEmail()      // Validate email format
	assert.String(t, "hello world").Contains("world")   // Contains substring
	assert.String(t, "hello").HasPrefix("he")            // Prefix check
	assert.String(t, `{"name": "bob"}`).JSONEqual(`{"name":"bob"}`) // JSON structural equality

	// Slice assertions
	assert.Slice(t, []int{1, 2, 3}).Contains(2)         // Contains element
	assert.Slice(t, []int{1, 2, 3}).Length(3)           // Length check
	assert.Slice(t, []int{1, 2, 3}).NotEmpty()           // Not empty check
	assert.Slice(t, []int{1, 2, 3}).AllUnique()         // All elements unique

	// Map assertions
	m := map[string]int{"a": 1, "b": 2}
	assert.Map(t, m).ContainsKey("a")                    // Contains key
	assert.Map(t, m).ContainsKeyValue("a", 1)           // Contains key-value pair
	assert.Map(t, m).Length(2)                           // Length check

	// Panic assertion
	assert.Panic(t, func() {
		panic("something wrong happened")
	}, "wrong")  // Assert that panic occurs and message contains "wrong"
}
```

## Assertion Method Reference

### Generic Assertions (That)

These generic assertion methods can be used with any type.
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `True(...msg)` | Verify that the boolean value is `true` |
| `False(...msg)` | Verify that the boolean value is `false` |
| `Nil(...msg)` | Verify that the value is `nil` (correctly handles nil in interface types) |
| `NotNil(...msg)` | Verify that the value is not `nil` |
| `Equal(expected, ...msg)` | Deep comparison using `reflect.DeepEqual` |
| `NotEqual(expected, ...msg)` | Verify not deeply equal |
| `Same(expected, ...msg)` | Exact comparison using `==` (same pointer address) |
| `NotSame(expected, ...msg)` | Comparison using `!=` |
| `TypeOf(interface, ...msg)` | Verify that the type is assignable to the target type |
| `Implements(interface, ...msg)` | Verify that the type implements the specified interface |
| `Has(expected, ...msg)` | Call the value's `Has` method, verify it returns `true` |
| `Contains(expected, ...msg)` | Call the value's `Contains` method, verify it returns `true` |

### Error Assertions (Error)

Dedicated assertions for `error` type.
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Nil(...msg)` | Verify the error is `nil` |
| `NotNil(...msg)` | Verify the error is not `nil` |
| `Is(target, ...msg)` | Verify error is the target error using `errors.Is` |
| `NotIs(target, ...msg)` | Verify error is not the target error using `errors.Is` |
| `String(expect, ...msg)` | Verify error message string equality |
| `Matches(pattern, ...msg)` | Verify error message matches regular expression |

### Number Assertions (Number)

Supports all numeric types (`int`/`uint`/`float`, etc.).
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Equal(expect, ...msg)` | Equal to |
| `NotEqual(expect, ...msg)` | Not equal to |
| `GreaterThan(expect, ...msg)` | Greater than |
| `GreaterOrEqual(expect, ...msg)` | Greater than or equal to |
| `LessThan(expect, ...msg)` | Less than |
| `LessOrEqual(expect, ...msg)` | Less than or equal to |
| `Zero(...msg)` | Equal to zero |
| `NotZero(...msg)` | Not equal to zero |
| `Positive(...msg)` | Positive number |
| `NotPositive(...msg)` | Non-positive (≤ 0) |
| `Negative(...msg)` | Negative number |
| `NotNegative(...msg)` | Non-negative (≥ 0) |
| `Between(lower, upper, ...msg)` | Within the interval (inclusive) |
| `NotBetween(lower, upper, ...msg)` | Not within the interval |
| `InDelta(expect, delta, ...msg)` | Within the expected error tolerance |
| `IsNaN(...msg)` | Is NaN (only valid for floats) |
| `IsInf(sign, ...msg)` | Is infinity (sign ≥ 0 for +Inf, < 0 for -Inf) |
| `IsFinite(...msg)` | Is a finite number (not NaN and not Inf) |

### String Assertions (String)

Dedicated assertions for `string` type.
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Length(length, ...msg)` | Verify length |
| `Blank(...msg)` | Verify empty or all whitespace |
| `NotBlank(...msg)` | Verify not blank |
| `Equal(expect, ...msg)` | Equal to |
| `NotEqual(expect, ...msg)` | Not equal to |
| `EqualFold(expect, ...msg)` | Case-insensitive equality |
| `JSONEqual(expect, ...msg)` | Deserialize JSON and compare structural equality |
| `Matches(pattern, ...msg)` | Match regular expression |
| `HasPrefix(prefix, ...msg)` | Starts with the specified prefix |
| `HasSuffix(suffix, ...msg)` | Ends with the specified suffix |
| `Contains(substr, ...msg)` | Contains substring |
| `IsLowerCase(...msg)` | All lowercase |
| `IsUpperCase(...msg)` | All uppercase |
| `IsNumeric(...msg)` | All digits |
| `IsAlpha(...msg)` | All letters |
| `IsAlphaNumeric(...msg)` | All letters and digits |
| `IsEmail(...msg)` | Verify is a valid email address |
| `IsURL(...msg)` | Verify is a valid URL |
| `IsIPv4(...msg)` | Verify is a valid IPv4 address |
| `IsHex(...msg)` | Verify is a valid hexadecimal string |
| `IsBase64(...msg)` | Verify is a valid Base64 encoding |

### Slice Assertions (Slice)

Dedicated assertions for slice type `[]T`.
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Length(length, ...msg)` | Verify length |
| `Nil(...msg)` | Verify is nil |
| `NotNil(...msg)` | Verify is not nil |
| `Empty(...msg)` | Verify empty (length is zero) |
| `NotEmpty(...msg)` | Verify not empty |
| `Equal(expect, ...msg)` | Slice is exactly equal (element order and values match) |
| `NotEqual(expect, ...msg)` | Verify not equal |
| `Contains(element, ...msg)` | Contains element |
| `NotContains(element, ...msg)` | Does not contain element |
| `ContainsSlice(sub, ...msg)` | Contains consecutive sub-slice |
| `NotContainsSlice(sub, ...msg)` | Does not contain consecutive sub-slice |
| `HasPrefix(prefix, ...msg)` | Starts with the specified slice as a prefix |
| `HasSuffix(suffix, ...msg)` | Ends with the specified slice as a suffix |
| `AllUnique(...msg)` | All elements are unique |
| `AllMatches(fn, ...msg)` | All elements satisfy the predicate function |
| `AnyMatches(fn, ...msg)` | At least one element satisfies the predicate function |
| `NoneMatches(fn, ...msg)` | No element satisfies the predicate function |

### Map Assertions (Map)

Dedicated assertions for map type `map[K]V`.
**All methods support adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Length(length, ...msg)` | Verify length |
| `Nil(...msg)` | Verify is nil |
| `NotNil(...msg)` | Verify is not nil |
| `Empty(...msg)` | Verify empty |
| `NotEmpty(...msg)` | Verify not empty |
| `Equal(expect, ...msg)` | Exactly equal |
| `NotEqual(expect, ...msg)` | Not equal |
| `ContainsKey(key, ...msg)` | Contains key |
| `NotContainsKey(key, ...msg)` | Does not contain key |
| `ContainsValue(value, ...msg)` | Contains value |
| `NotContainsValue(value, ...msg)` | Does not contain value |
| `ContainsKeyValue(key, value, ...msg)` | Contains the specified key-value pair |
| `ContainsKeys(keys, ...msg)` | Contains all specified keys |
| `NotContainsKeys(keys, ...msg)` | Does not contain any of the specified keys |
| `ContainsValues(values, ...msg)` | Contains all specified values |
| `NotContainsValues(values, ...msg)` | Does not contain any of the specified values |
| `SubsetOf(expect, ...msg)` | Current map is a subset of expect (all key-value pairs exist in expect) |
| `SupersetOf(expect, ...msg)` | Current map is a superset of expect (all key-value pairs in expect exist in current) |
| `HasSameKeys(expect, ...msg)` | Has exactly the same set of keys as expect |
| `HasSameValues(expect, ...msg)` | Has exactly the same multiset of values as expect (order doesn't matter) |

### Panic Assertion

Used to detect whether a function will panic. This is a top-level function.
**Supports adding `msg ...string` at the end for custom error messages**.

| Method | Description |
|--------|-------------|
| `Panic(t, fn, pattern, ...msg)` | Assert that `fn` panics, and the panic message matches the regex `pattern` |

## Custom Error Messages

All assertion methods support adding custom error messages at the end:

```go
assert.That(t, result).Equal(expected, "result should match expected")
assert.Number(t, age).GreaterThan(18, "user should be an adult")
```

## License

Apache License 2.0
