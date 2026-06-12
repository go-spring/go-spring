# Testing

Go-Spring is compatible with Go's native `go test` mechanism. It does not require an additional test runner and does not change the project's build process.
You can still use `go test` to run tests and use it together with coverage, race detection, and CI tools from the Go ecosystem.

In addition, Go-Spring provides a set of testing enhancements:
it supports integration tests based on the IoC container and provides type-safe assertion tools and Mock support, reducing boilerplate code
and making tests more concise and maintainable.

## Pure Unit Testing

Pure unit tests usually verify the behavior of a single function, a single method, or a single service, and do not need to start the IoC container.
In this case, it is recommended to use Go's native testing style: manually construct the object under test and explicitly pass in the objects or Mock implementations it depends on.

For example, a user service depends on a user repository:

```go
type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUserName(id int) (string, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return "", err
	}
	return user.Name, nil
}
```

During testing, you can directly create a Mock `UserRepository` implementation and pass it to the service under test:

```go
func TestUserService_GetUserName(t *testing.T) {
    mockRepo := &MockUserRepository{...}
    service := NewUserService(mockRepo)

    name, err := service.GetUserName(1)

    // Make assertions...
}
```

This approach does not depend on framework features, has low startup cost, runs quickly, and makes problems easier to locate.
For tests that only care about business logic, pure unit tests should be preferred.

## IoC Container-Based Testing

When a test needs to verify collaboration among multiple Beans, or needs to cover container behavior such as configuration injection, dependency injection, and conditional assembly,
you can use `gs.RunTest` to start an IoC container-based test.

`gs.RunTest` creates a test object based on the parameter type of the callback function.
This test object is registered in the container as a root Bean, and then the container builds and injects the dependency graph starting from it.
The overall process is as follows:

1. Read the single parameter type of the callback function and create the corresponding test object instance.
2. Register the test object as a root Bean, serving as the entry point for dependency injection.
3. Start the test container, refresh configuration, and parse registered Bean definitions.
4. Recursively inject dependencies starting from the root Bean, while processing tags such as `value` and `autowire`.
5. Call the test callback and pass in the test object whose injection has completed.
6. Close the container after the test callback ends and execute related destroy logic.

Therefore, test code does not need to manually start or close the container; it only needs to write assertion logic in the callback function.
The callback parameter can use Go-Spring tags such as `value` and `autowire`, consistent with how regular Beans are used.

```go
func TestOrderFlow(t *testing.T) {
	gs.RunTest(t, func(s *struct {
		OrderSvc *OrderService `autowire:""`              // Inject order service
		StockSvc *StockService `autowire:""`              // Inject stock service
		Env      string        `value:"${app.env:=test}"` // Inject configuration
	}) {
		err := s.OrderSvc.Create(1001)
		assert.That(t, err).Nil()
		assert.That(t, s.StockSvc.Remaining(1001)).Equal(99)
	})
}
```

### Custom Configuration

Before `gs.RunTest()`, you can call `g.Property()` through `gs.Configure()`
to add configuration items for the current test:

```go
func TestApp(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Property("database.url", "sqlite://:memory:")
		g.Property("app.env", "test")
	}).RunTest(t, func(s *struct {
		DB  *DB    `autowire:""`
		Env string `value:"${app.env}"`
	}) {
		assert.String(t, s.Env).Equal("test")

		// Test logic
	})
}
```

### Replacing Dependencies

Before `gs.RunTest()`, you can call `g.Provide()` through `gs.Configure()`
to add Beans for the current test.

> When running IoC container-based tests, due to the Bean discovery mechanism, Go-Spring does not guarantee that all dependencies are automatically created.
> Therefore, dependency injection in tests is treated as optional (nullable).
> If certain key Beans are not automatically loaded, you can explicitly register the required Beans in this way to ensure the test environment is complete and controllable.

```go
func TestUserService(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Provide(func() UserRepository {
			return &MockUserRepository{ /* Preset test data */ }
		})
	}).RunTest(t, func(s *struct {
		Service *UserService `autowire:""`
	}) {
		user, err := s.Service.GetUser(1)
		assert.That(t, err).Nil()
		assert.That(t, user.ID).Equal(1)
	})
}
```

### Isolation Notes

Each `gs.RunTest()` test copies the Bean definitions registered during the `init` phase.
Beans registered in `gs.Configure()` then only apply to the current test and do not affect other test cases.

Because global `init` registration information is shared, IoC container-based tests currently do not support `t.Parallel()`.

## Assertion Library

Go-Spring provides a fluent assertion library under `go-spring.org/stdlib/testing`,
including the two subpackages `assert` and `require`.

### assert and require

The `assert` package records failure information when an assertion fails, but does not immediately terminate the current test function.
It is suitable for checking multiple independent conditions, allowing one test run to expose as many problems as possible.

The `require` package immediately terminates the current test function when an assertion fails.
It is suitable for checking preconditions for subsequent logic, such as when an object must be non-nil or initialization must succeed.

```go
import (
    "go-spring.org/stdlib/testing/assert"
    "go-spring.org/stdlib/testing/require"
)

func TestExample(t *testing.T) {
	user, err := service.GetUser(1)

	// Terminate immediately when err is not nil, avoiding subsequent code accessing an invalid object
	require.That(t, err).Nil()

	// Subsequent assertions continue to execute other asserts even if one fails
	assert.That(t, user).NotNil()
	assert.That(t, user.ID).Equal(1)
	assert.That(t, user.Name).Equal("Alice")
}
```

### Basic Usage

The assertion library provides type-specific entry points. Different entry points expose different assertion methods, which helps detect type misuse earlier at compile time.

Any type can use `That`:

```go
assert.That(t, 42).Equal(42)     // Equality assertion
assert.That(t, user).NotNil()    // Non-nil assertion
assert.That(t, ok).True()        // Boolean value is true
assert.That(t, available).False() // Boolean value is false
```

Error types can use `Error`:

```go
import "os"

err := someFunc()
assert.Error(t, err).NotNil()           // Expect an error
assert.Error(t, os.ErrNotExist).Is(err) // Use errors.Is to determine the error type
```

Numeric types can use `Number`:

```go
import "math"

assert.Number(t, 42).GreaterThan(40)          // Greater than
assert.Number(t, 100).Between(0, 200)         // Within range
assert.Number(t, 0).Zero()                    // Equal to zero
assert.Number(t, 3.14).InDelta(math.Pi, 0.01) // Floating-point comparison tolerance
```

String types can use `String`:

```go
assert.String(t, "user@example.com").IsEmail()                  // Validate email format
assert.String(t, "hello world").Contains("world")               // Contains substring
assert.String(t, "hello").HasPrefix("he")                       // Prefix check
assert.String(t, `{"name": "bob"}`).JSONEqual(`{"name":"bob"}`) // JSON structures are equal
```

Slice types can use `Slice`:

```go
assert.Slice(t, []int{1, 2, 3}).Contains(2) // Contains element
assert.Slice(t, []int{1, 2, 3}).Length(3)   // Length check
assert.Slice(t, []int{1, 2, 3}).NotEmpty()  // Non-empty check
assert.Slice(t, []int{1, 2, 3}).AllUnique() // All elements are unique
```

map types can use `Map`:

```go
m := map[string]int{"a": 1, "b": 2}
assert.Map(t, m).ContainsKey("a")          // Contains key
assert.Map(t, m).ContainsKeyValue("a", 1)  // Contains key-value pair
assert.Map(t, m).Length(2)                 // Length check
```

To check panic, use the top-level function `Panic`:

```go
assert.Panic(t, func() {
    panic("something wrong happened")
}, "wrong") // Expect a panic, and the message contains "wrong"
```

### Custom Error Messages

Assertion methods support appending custom error messages at the end, so that assertion failure output contains clearer business semantics and is easier to locate quickly.

```go
assert.That(t, result).Equal(expected, "result should match expected")
assert.Number(t, age).GreaterThan(18, "user should be an adult")
```

## Mock Framework

Go-Spring provides `gs-mock` as a Mock framework.
It supports interface Mocks, regular function Mocks, and struct method Mocks, and uses generics to generate type-safe invocation APIs.

### Interface Mock

Interface Mocks can be created by code generation without hand-writing Mock structs. Suppose there is the following interface:

```go
type Service interface {
    Do(n int, s string) (int, error)
    Format(format string, args ...any) string
}
```

Then you need to add a `go:generate` directive at the package level:

```go
//go:generate gs mock -o mock.go
```

This command generates Mock code for interfaces in the current package. If you only need to generate Mocks for some interfaces, you can use the `-i` parameter:

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

After code generation, you can use either the Handle pattern or the When/Return pattern to configure Mocks in tests.

The Handle pattern is suitable for scenarios that require custom logic:

```go
func TestService_Do(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	s.MockDo().Handle(func(n int, s string) (int, error) {
		if n%2 == 0 {
			return n * 2, nil
		}
		return 0, errors.New("odd number")
	})

	res, err := s.Do(2, "abc")
	assert.That(t, err).Nil()
	assert.That(t, res).Equal(4)
}
```

The When/Return pattern is suitable for scenarios where fixed return values are matched according to input parameters:

```go
func TestService_Format(t *testing.T) {
    r := gsmock.NewManager()
    s := NewServiceMockImpl(r)

    // Return "abc" when the first element of args is "abc"
    s.MockFormat().When(func(format string, args []any) bool {
        return args[0] == "abc"
    }).ReturnValue("abc")

    // Return "123" when the first element of args is "123"
    s.MockFormat().When(func(format string, args []any) bool {
        return args[0] == "123"
    }).ReturnValue("123")

    // Matching rules execute in registration order, and the first successfully matched rule returns the result
    assert.That(t, s.Format("", "abc", "123")).Equal("abc")
    assert.That(t, s.Format("", "123", "abc")).Equal("123")
}
```

For variadic methods, variadic arguments are packed as a single slice parameter and passed into the `When` callback, such as `args []any` in the example above.

### Function and Method Mock

In addition to interface Mocks, `gs-mock` can also Mock regular functions and struct methods.
This approach is suitable for adding tests to existing code and avoids abstracting interfaces only for testing.

Regular function Mock requires the function's first parameter to be `context.Context`.
The Mock Manager is passed through Context, isolating Mock configuration across different tests or different call chains.

```go
//go:noinline // Recommended to add, preventing the function from being inlined and causing Mock failure
func GetUser(ctx context.Context, id int) (*User, error) {
    // Real implementation queries the database
}

func Test_GetUser(t *testing.T) {
	r := gsmock.NewManager()
	ctx := gsmock.WithManager(context.TODO(), r)

	// Func22 means 2 parameters and 2 return values
	gsmock.Func22(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
		if id == 1 {
			return &User{ID: 1, Name: "Alice"}, nil
		}
		return nil, errors.New("not found")
	})

	user, err := GetUser(ctx, 1)
	assert.That(t, err).Nil()
	assert.That(t, user.Name).Equal("Alice")
}
```

Mocking struct methods is similar. Note, however, that a method expression passes the receiver as the first parameter:

```go
type Service struct {
    BaseURL string
}

func (s *Service) GetUser(ctx context.Context, id int) (*User, error) {
    // Real implementation
}

func TestService_GetUser(t *testing.T) {
    r := gsmock.NewManager()
    ctx := gsmock.WithManager(context.TODO(), r)

    // Use the method expression (*Service).GetUser, rather than an instance method
    // Receiver *Service is the first parameter, and ctx is the second parameter
    gsmock.Func32((*Service).GetUser, r).Handle(func(s *Service, ctx context.Context, id int) (*User, error) {
        if s.BaseURL == "https://api.example.com" {
            return &User{ID: id, Name: "Mocked"}, nil
        }
        return nil, errors.New("wrong endpoint")
    })

    svc := &Service{BaseURL: "https://api.example.com"}
    user, err := svc.GetUser(ctx, 1)
    assert.That(t, err).Nil()
    assert.That(t, user.Name).Equal("Mocked")
}
```

### Usage Tips

When using function or method Mock, note the following:

- **Disable inlining**: The Go compiler may inline small functions, causing Mock to be unable to intercept calls.
  You can add the parameter `-gcflags="all=-N -l"` when running tests to disable inlining:

```bash
go test -gcflags="all=-N -l" ./...
```

- **Isolate through Context**: `gs-mock` passes the Mock Manager through `context.Context`.
  Each test creates its own Manager, which can avoid interference between different tests or goroutines.

- **Register rules in advance**: All Mock rules should be registered before test logic begins.
  Do not dynamically register rules during concurrent execution, to avoid unpredictable matching order.

- **Matching order**: Multiple When/Return rules are matched in registration order, and the first successfully matched rule immediately returns the result.
  It is recommended to register rules from specific to broad.
