# testing

[English](README.md) | [中文](README_CN.md)

Go-Spring Testing is the zero-dependency assertion library every Go-Spring module uses for its own tests. It provides a fluent, type-specific assertion API (`That`, `Error`, `Number`, `String`, `Slice`, `Map`, `Panic`) as an alternative to `stretchr/testify` and `gomega`, with generics for compile-time type safety and no dependency beyond the Go standard library.

## Usage

Both entry points expose the same functions — pick the failure behaviour by the import:

```go
import (
	"go-spring.org/stdlib/testing/assert"  // fail-continue
	"go-spring.org/stdlib/testing/require" // fail-fast
)
```

### assert vs require

- **`assert`** does not stop the test when an assertion fails: later assertions are still checked, which is useful when you want one run to report every failure at once.
- **`require`** stops the test immediately on the first failure — for critical preconditions whose absence would make later assertions panic or misbehave, e.g. verifying an object is non-nil before touching it.

Use `require` when a failed check invalidates everything after it (the value being unwrapped is nil, the fixture failed to set up); use `assert` when multiple independent assertions in one test are informative on their own. The two are routinely mixed in a single test:

```go
func TestUser(t *testing.T) {
    user := loadUser()
    require.That(t, user).NotNil() // stop here if nil — the next line would panic

    assert.String(t, user.Email).IsEmail()
    assert.Number(t, user.Age).GreaterThan(0)
}
```

### Basic Example

```go
package main

import (
	"testing"
	"os"
	"math"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
)

func TestExample(t *testing.T) {
	// Generic assertions - works with any type
	assert.That(t, "hello").Equal("hello")        // Equality assertion
	assert.That(t, user).NotNil()                 // Non-nil assertion
	assert.That(t, len("hello") > 0).True()       // Boolean expression is true

	// Using require - test stops immediately on failure
	require.That(t, user).NotNil()

	// Error assertions
	err := someFunc()
	assert.Error(t, err).NotNil()                 // Expect an error to occur
	assert.Error(t, err).Is(os.ErrNotExist)        // Check error type using errors.Is

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
	}, "wrong")  // Assert that fn panics and the message matches the pattern "wrong"
}
```

### Assertion Method Reference

Every family below supports a trailing `msg ...string` on every method for custom error messages.

#### Generic Assertions (That)

Usable with any type.

| Method | Description |
|--------|-------------|
| `True(...msg)` | Verify that the boolean value is `true` |
| `False(...msg)` | Verify that the boolean value is `false` |
| `Nil(...msg)` | Verify that the value is `nil` (correctly handles nil in interface types) |
| `NotNil(...msg)` | Verify that the value is not `nil` |
| `Equal(expect, ...msg)` | Deep comparison using `reflect.DeepEqual` |
| `NotEqual(expect, ...msg)` | Verify not deeply equal |
| `Same(expect, ...msg)` | Exact comparison using `==` (identical per Go `==`) |
| `NotSame(expect, ...msg)` | Comparison using `!=` |
| `TypeOf(expect, ...msg)` | Verify that the type is assignable to the target type |
| `Implements(expect, ...msg)` | Verify that the type implements the specified interface |
| `Has(expect, ...msg)` | Call the value's `Has` method, verify it returns `true` |
| `Contains(expect, ...msg)` | Call the value's `Contains` method, verify it returns `true` |

#### Error Assertions (Error)

Dedicated to the `error` type.

| Method | Description |
|--------|-------------|
| `Nil(...msg)` | Verify the error is `nil` |
| `NotNil(...msg)` | Verify the error is not `nil` |
| `Is(target, ...msg)` | Verify error is the target error using `errors.Is` |
| `NotIs(target, ...msg)` | Verify error is not the target error using `errors.Is` |
| `String(expect, ...msg)` | Verify error message string equality |
| `Matches(pattern, ...msg)` | Verify error message matches regular expression |

#### Number Assertions (Number)

Supports all numeric types (`int`/`uint`/`float`, etc.).

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

#### String Assertions (String)

Dedicated to the `string` type.

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

#### Slice Assertions (Slice)

Dedicated to slice type `[]T`.

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

#### Map Assertions (Map)

Dedicated to map type `map[K]V`.

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

#### Panic Assertion

Top-level function used to detect whether a function panics.

| Method | Description |
|--------|-------------|
| `Panic(t, fn, pattern, ...msg)` | Assert that `fn` panics, and the panic message matches the regex `pattern` |

## License

`testing` is distributed under the Apache License 2.0. See
[LICENSE](../../LICENSE) for details.
