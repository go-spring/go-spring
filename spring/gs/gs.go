/*
 * Copyright 2024 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package gs provides dependency injection (DI) and application container
// inspired by Java Spring Framework and Spring Boot.
//
// Core Design Principles (aligned with Spring):
//
//  1. IoC (Inversion of Control): The container manages bean lifecycle, instantiation,
//     and dependency injection. Your code doesn't manually new-up dependencies -
//     container creates everything and wires them together.
//
//  2. DI (Dependency Injection): Beans declare dependencies via constructor parameters,
//     container automatically resolves and injects matching beans. This promotes
//     loose coupling and testability.
//
//  3. Conditional Auto-Configuration: Modules can be conditionally activated based on
//     configuration properties and existing beans, enabling a starter-style
//     auto-configuration system just like Spring Boot.
//
//  4. Early Registration: All beans and modules MUST be registered during package
//     initialization (init() phase). This guarantees everything is registered before
//     application startup, making the entire boot process predictable.
//
// Key Concepts mapping from Spring:
//
//	Java/Spring Annotation    | Go-Spring API
//	------------------------- | ------------------------------
//	@Bean / <bean>            | gs.Provide(...)
//	@Value("${prop.key}")     | gs.TagArg("${prop.key}")
//	@Conditional              | gs.Condition
//	@ConditionalOnProperty    | gs.OnProperty(...)
//	@ConditionalOnBean        | gs.OnBean[...]()
//	@ConditionalOnMissingBean | gs.OnMissingBean[...]()
//	Spring Boot AutoConfig    | gs.Module(condition, func)
//
// Go-Spring Extensions:
//   - [Group]: Bulk registers multiple beans from a configuration map.
//     When you have a list/map of configurations in YAML, it automatically
//     creates one bean per entry. This is great for multiple databases,
//     multiple HTTP clients, dynamic plugin loading, etc.
//   - [Dync]: Dynamic configuration that automatically updates when the
//     underlying configuration changes.
//
// Typical Usage:
//
//	// Step 1: Register beans in package-level init() function
//	func init() {
//	    // Register a simple bean
//	    gs.Provide(&MyServiceImpl{})
//
//	    // Register a conditional auto-configuration module (like Spring Boot starter)
//	    gs.Module(gs.OnProperty("redis.enabled"),
//	        func(r gs.BeanProvider, p flatten.Storage) error {
//	            r.Provide(NewRedisClient)
//	            return nil
//	        })
//	}
//
//	// Step 2: Boot the application in main()
//	func main() {
//	    gs.Configure(func(app gs.App) {
//	        app.Property("server.port", "8080")
//	    }).Run()
//	}
package gs

import (
	"reflect"
	"runtime"
	"strings"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_app"
	"go-spring.org/spring/gs/internal/gs_arg"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/spring/gs/internal/gs_dync"
	"go-spring.org/spring/gs/internal/gs_init"
	"go-spring.org/stdlib/flatten"
)

const (
	Version = "go-spring@v1.3.1"
	Website = "https://go-spring.org/"
)

// BeanID represents a selector for a bean.
type BeanID = gs.BeanID

// BeanIDFor returns a BeanID for the given type T.
func BeanIDFor[T any](name ...string) BeanID {
	return gs.BeanIDFor[T](name...)
}

// Dync is a generic alias for a dynamic configuration value.
// Dync values are automatically updated when the underlying configuration changes.
type Dync[T any] = gs_dync.Value[T]

// As returns the [reflect.Type] of the given generic interface type T.
// It is typically used with [BeanDefinition.Export] to register interfaces
// that a bean implements, allowing dependency injection by interface type.
//
// Example:
//
//	type Service interface {
//	    Serve() error
//	}
//
//	type ServiceImpl struct{}
//
//	func (s *ServiceImpl) Serve() error { return nil }
//
//	gs.Provide(&ServiceImpl{}).Export(gs.As[Service]())
func As[T any]() reflect.Type {
	return gs.As[T]()
}

/************************************ arg ***********************************/

// Arg represents an argument used when binding constructor parameters.
// Args are primarily used in:
//   - Constructor parameter injection via gs.Provide(func(...) T, args...)
//   - Method-based bean definition with custom arguments
//   - Conditional bean creation based on runtime conditions
type Arg = gs.Arg

// TagArg creates an argument that injects a property or bean
// identified by the specified struct-tag expression.
//
// Example:
//
//	// Bind a configuration property
//	gs.Provide(func(cfg string) *MyBean { ... }, gs.TagArg("${app.name}"))
//
//	// Wire a dependency
//	gs.Provide(func(svc Service) *MyBean { ... }, gs.TagArg(""))
func TagArg(tag string) Arg {
	return gs_arg.Tag(tag)
}

// ValueArg creates an argument with a fixed value.
// This is useful for injecting constant values or pre-created instances into constructors.
//
// Example:
//
//	gs.Provide(func(timeout time.Duration) *Client { ... }, gs.ValueArg(30*time.Second))
func ValueArg(v any) Arg {
	return gs_arg.Value(v)
}

// IndexArg targets a specific constructor parameter by index
// and provides the given Arg as its value.
//
// Use cases:
//   - Skipping optional parameters
//   - Providing arguments out of order
//   - Injecting values into variadic functions
//
// Example:
//
//	// Inject value into the 3rd parameter (index 2)
//	gs.Provide(func(a, b, c string) *Bean { ... }, gs.IndexArg(2, gs.ValueArg("value")))
func IndexArg(n int, arg Arg) Arg {
	return gs_arg.Index(n, arg)
}

// BindArg creates an argument whose value is computed dynamically by a function.
// It executes the function at bean creation time to get the actual argument value,
// and supports attaching conditions that control whether the argument is accepted.
//
// Use cases:
//   - Conditional bean creation based on runtime state
//   - Complex initialization logic that requires multiple dependencies
//   - Lazy evaluation of expensive resources
//
// Example:
//
//	gs.Provide(
//	    func(logger *Logger) *Service { ... },
//	    gs.BindArg(func() (*Logger, error) {
//	        return createLogger()
//	    }).Condition(gs.OnProperty("logging.enabled"))
//	)
func BindArg(fn any, args ...Arg) *gs_arg.BindArg {
	return gs_arg.Bind(fn, args...)
}

/************************************ cond ***********************************/

type (
	// Condition represents a logical predicate that decides whether
	// a bean or module should be activated.
	Condition = gs.Condition

	// ConditionContext provides the evaluation context for a Condition.
	ConditionContext = gs.ConditionContext

	// PropertyCondition is a convenience wrapper for property-based conditions.
	PropertyCondition = gs_cond.PropertyCondition
)

// OnOnce wraps the given conditions so they are evaluated only once.
// Subsequent calls return the cached result without re-evaluation.
func OnOnce(conditions ...Condition) Condition {
	cond := gs_cond.And(conditions...)
	if cond == nil {
		return nil
	}
	var (
		done      bool
		result    bool
		resultErr error
	)
	return OnFunc(func(ctx ConditionContext) (bool, error) {
		if done {
			return result, resultErr
		}
		done = true
		result, resultErr = cond.Matches(ctx)
		return result, resultErr
	})
}

// OnFunc creates a Condition backed by the given function.
// This is the most flexible way to create custom conditions.
//
// Example:
//
//	gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
//	    val, ok := ctx.Prop("feature.enabled")
//	    return ok && val == "true", nil
//	})
func OnFunc(fn func(ctx ConditionContext) (bool, error)) Condition {
	return gs_cond.OnFunc(fn)
}

// OnProperty creates a property-based condition.
// It checks for the presence and/or value of a configuration property.
//
// Example:
//
//	// Match if property exists
//	gs.OnProperty("app.name")
//
//	// Match if property has specific value
//	gs.OnProperty("app.env").HavingValue("production")
//
//	// Match if property is missing
//	gs.OnProperty("debug.mode").MatchIfMissing()
func OnProperty(name string) PropertyCondition {
	return gs_cond.OnProperty(name)
}

// OnBean requires that a bean of the given type (and optional name) exists.
// The condition is satisfied if at least one matching bean is found.
//
// Example:
//
//	// Require a Logger bean to exist
//	gs.OnBean[*Logger]()
//
//	// Require a specific named DataSource bean
//	gs.OnBean[*DataSource]("primary")
func OnBean[T any](name ...string) Condition {
	return gs_cond.OnBean[T](name...)
}

// OnMissingBean requires that no bean of the given type (and optional name) exists.
// The condition is satisfied if zero matching beans are found.
//
// Example:
//
//	// Provide default logger only if none exists
//	gs.Provide(&DefaultLogger{}).Condition(gs.OnMissingBean[*Logger]())
func OnMissingBean[T any](name ...string) Condition {
	return gs_cond.OnMissingBean[T](name...)
}

// OnSingleBean requires that exactly one instance of the given bean type exists.
// The condition is satisfied if len(beans) == 1.
//
// Example:
//
//	// Ensure only one Database connection exists
//	gs.OnSingleBean[*Database]()
func OnSingleBean[T any](name ...string) Condition {
	return gs_cond.OnSingleBean[T](name...)
}

// RegisterExpressFunc registers a custom expression function
// that can be used inside conditional expressions.
// It should be called during application initialization.
func RegisterExpressFunc(name string, fn any) {
	gs_cond.RegisterExpressFunc(name, fn)
}

// OnExpression creates a condition from an expression.
// The expression is evaluated to determine if the condition is satisfied.
//
// Example (future):
//
//	gs.OnExpression("${app.port} > 8080 && ${app.env} == 'prod'")
func OnExpression(expression string) Condition {
	return gs_cond.OnExpression(expression)
}

// Not returns the logical negation of the given condition.
// It inverts the result: true becomes false, false becomes true.
//
// Example:
//
//	// Activate bean only if debug mode is NOT enabled
//	gs.Not(gs.OnProperty("debug.enabled"))
func Not(c Condition) Condition {
	return gs_cond.Not(c)
}

// Or combines multiple conditions using logical OR.
// The combined condition is satisfied if AT LEAST ONE condition is true.
//
// Example:
//
//	// Activate if either dev OR test profile is active
//	gs.Or(gs.OnProperty("profile.dev"), gs.OnProperty("profile.test"))
func Or(conditions ...Condition) Condition {
	return gs_cond.Or(conditions...)
}

// And combines multiple conditions using logical AND.
// The combined condition is satisfied only if ALL conditions are true.
//
// Example:
//
//	// Activate only if production AND database configured
//	gs.And(
//	    gs.OnProperty("app.env").HavingValue("production"),
//	    gs.OnBean[*Database](),
//	)
func And(conditions ...Condition) Condition {
	return gs_cond.And(conditions...)
}

// None returns a condition that is true if ALL provided conditions are false.
// It is equivalent to NOR (NOT OR) logic.
//
// Example:
//
//	// Activate only if neither debug nor verbose is enabled
//	gs.None(gs.OnProperty("debug"), gs.OnProperty("verbose"))
func None(conditions ...Condition) Condition {
	return gs_cond.None(conditions...)
}

/*********************************** app *************************************/

type (
	Configuration       = gs_bean.Configuration
	BeanProvider        = gs_init.BeanProvider
	Runner              = gs_app.Runner
	Server              = gs_app.Server
	ReadySignal         = gs_app.ReadySignal
	ContextProvider     = gs_app.ContextProvider
	PropertiesRefresher = gs_app.PropertiesRefresher
)

// Provide registers a global bean definition.
// It must be called during package initialization (init phase).
// Calling it after application configuration has started will panic.
// It accepts either an existing instance or a constructor function.
// The optional args are used to bind parameters for the constructor or to
// provide explicit injection values.
//
// Parameters:
//   - objOrCtor: bean instance (struct pointer) or constructor function
//   - args: optional arguments for parameter binding
//
// Limitations:
//   - MUST be called during init phase (before application startup)
//   - Will panic if called after initialization is complete
//   - Bean must be a reference type (pointer), not a value type
//   - Constructor functions must have signature: func(...)bean or func(...)(bean, error)
//
// Example:
//
//	// Register an instance
//	gs.Provide(&MyService{})
//
//	// Register a constructor with arguments
//	gs.Provide(NewMyService, gs.TagArg("${config.key}"))
//
//	// Chain configuration
//	gs.Provide(&Logger{}).Name("default").InitMethod("Start")
func Provide(objOrCtor any, args ...Arg) *gs_bean.BeanDefinition {
	if inited {
		panic("gs.Provide can only be called in init function")
	}
	b := gs_bean.NewBean(objOrCtor, args...)
	gs_init.AddBean(b)
	return b.Caller(2)
}

// ModuleFunc defines the signature of a module function.
//
// Use cases:
//   - Auto-configuration modules (similar to Spring Boot starters)
//   - Grouping related bean registrations
//   - Conditional bean registration based on configuration
type ModuleFunc = gs_init.ModuleFunc

// Module registers a configuration module that is conditionally activated
// based on property values.
//
// Limitations:
//   - MUST be called during init phase (will panic otherwise)
//   - Condition is evaluated at module execution time, not registration time
//   - Module functions cannot be called directly; they are managed by the framework
//
// Use cases:
//   - Starter modules that auto-configure based on classpath/dependencies
//   - Feature modules that activate based on configuration flags
//   - Modular application architecture
//
// Example:
//
//	gs.Module(gs.OnProperty("redis.enabled"),
//	func(r gs.BeanProvider, p flatten.Storage) error {
//	    r.Provide(NewRedisClient)
//	    return nil
//	})
func Module(c PropertyCondition, fn ModuleFunc) {
	if inited {
		panic("gs.Module can only be called in init function")
	}
	if fn == nil {
		panic("gs.Module function cannot be nil")
	}
	_, file, line, _ := runtime.Caller(1)
	gs_init.AddModule(c, fn, file, line)
}

// Group reads a map from the specified configuration property, then iterates
// over each entry and creates one bean per entry using the constructor fn.
// The map key becomes the bean name, and the map value is passed to fn as
// the configuration for constructing the bean. An optional destructor d
// can be provided for cleanup when the application shuts down.
//
// Use cases:
//   - Multiple database connections from configuration
//   - Multiple dynamic service clients configured in YAML
//   - Plugin architectures with configurable components
//
// Example:
//
//	// Register multiple HTTP clients from configuration
//	gs.Group(
//	    "${http.clients}",
//	    func(cfg HTTPClientConfig) (*HTTPClient, error) {
//	        return NewHTTPClient(cfg)
//	    },
//	    func(c *HTTPClient) error {
//	        return c.Close()
//	    },
//	)
//
// Corresponding YAML configuration:
//
//	http:
//	  clients:
//	    serviceA:  # <- "serviceA" becomes the bean name
//	      baseURL: "http://a.example.com"
//	      timeout: 30s
//	    serviceB:  # <- "serviceB" becomes the bean name
//	      baseURL: "http://b.example.com"
//	      timeout: 60s
func Group[T any, R any](tag string, fn func(c T) (R, error), d func(R) error) {
	if inited {
		panic("gs.Group can only be called in init function")
	}
	if !strings.HasPrefix(tag, "${") || !strings.HasSuffix(tag, "}") {
		panic("gs.Group tag must be in ${...} format")
	}
	if fn == nil {
		panic("gs.Group function cannot be nil")
	}
	_, file, line, _ := runtime.Caller(1)
	key := strings.TrimSuffix(strings.TrimPrefix(tag, "${"), "}")
	gs_init.AddModule(OnProperty(key), func(r BeanProvider, p flatten.Storage) error {
		var m map[string]T
		if err := conf.Bind(p, &m, "${"+key+"}"); err != nil {
			return err
		}
		for name, c := range m {
			b := r.Provide(fn, ValueArg(c)).Name(name)
			if d != nil {
				b.Destroy(d)
			}
			b.SetFileLine(file, line)
		}
		return nil
	}, file, line)
}
