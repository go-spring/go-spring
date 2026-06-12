# Configuration Management

Go-Spring provides a unified and powerful configuration management system that helps you handle everything from development to production with confidence.
If you have used Java Spring Boot, you will find the design very similar. Many concepts map directly, so the learning curve is low.

## Properties

The Go-Spring configuration system is designed to be very simple: **no matter which format you use to write configuration, it is ultimately converted into a flattened key-value structure**,
which we call `Properties`. This design is the same as Spring Boot's `Environment` abstraction.

The biggest benefit of this approach is that it **unifies the configuration access interface**: upper-level logic such as binding, validation, and priority merging
never needs to care about the original configuration format. No matter which format you write, it can ultimately be accessed in the same way.

Note that **key matching is case-sensitive**. The framework does not provide automatic case conversion and does not support relaxed matching (such as camelCase-to-underscore conversion,
automatically matching with omitted separators, and so on). A key is exactly the string you wrote, and it must match exactly.

### Path Syntax

Go-Spring uses the industry-standard path syntax to locate configuration items. The syntax is very intuitive:

- Use dots `.` to separate nested levels: for example, `a.b.c` means `a` -> `b` -> `c`
- Use square brackets `[index]` to represent array indexes: for example, `a.b[0].c` means the `c` property of the first element in the `a.b` array

For example, here is a typical YAML configuration:

```yaml
app:
  port: 8080
  database:
    - host: localhost
      port: 5432
    - host: repli.ca
      port: 5433
```

After it is expanded into flattened properties, it looks like this:

```
app.port = 8080
app.database[0].host = localhost
app.database[0].port = 5432
app.database[1].host = repli.ca
app.database[1].port = 5433
```

This is easy to understand: every configuration item has a unique path, which is clear and unambiguous.

## Configuration Binding

After configuration is loaded, the next step is to **bind** configuration values to variables in Go code so you can use them in your application.

Go-Spring provides two binding methods for different scenarios:

- **Struct tag binding** (recommended): suitable for most day-to-day development; declarative usage is the most concise
- **Manual Bind function binding**: usually used when creating multiple beans in a module

### Struct Tag Binding

This is the simplest approach. You only need to define a struct and add a `value` tag to its fields.
This is equivalent to `@Value` or `@ConfigurationProperties` in Spring Boot.

```go
type ServerConfig struct {
	Port        int           `value:"${port:=8080}"`
	Timeout     time.Duration `value:"${timeout:=30s}"`
	EnableSSL   bool          `value:"${enable-ssl:=true}"`
	Endpoints   []string      `value:"${endpoints}"`
}

// For the App.Config field:
// - Use `${server}` as the prefix for configuration binding
// - That is, each field in ServerConfig reads from configuration items that start with `server.`
//   For example: server.port, server.timeout, and so on
type App struct {
	Config ServerConfig `value:"${server}"`
}
```

Let's break down the tag syntax `value:"${key:=defaultValue}"`:

- `key` is the configuration item path mentioned above, corresponding to configuration in the configuration file
- `:=defaultValue` is **optional**. If this key cannot be found in the configuration, the default value you provide is used
- If the default value is omitted and the key does not exist in the configuration, it is a **required field**, and binding fails immediately

> **Tip:** If you write an empty key such as `${:=default}`, the default value is used directly and the configuration is not searched.
> This is useful when you want to hard-code a value while still preserving the possibility of configuration.

### Supported Data Types

Go-Spring supports a rich set of data types out of the box, from basic types to nested structs and custom types.

#### Basic Types

Go basic types can be bound directly without any extra configuration:

- **Boolean types** (`bool`): supports formats such as `true`/`false`, `1`/`0`, `t`/`f`, and more
- **Integer types** (`int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`): supports decimal and hexadecimal formats
- **Floating-point types** (`float32`, `float64`): supports scientific notation
- **String** (`string`): preserved as-is

For structs without a converter, Go-Spring recursively binds each field and supports nesting to any depth.
This lets you organize related configuration into clear struct hierarchies.

#### Built-in Special Converters

In addition to basic types, Go-Spring also includes converters for several commonly used types out of the box:

| Type | Description | Example |
|------|-------------|---------|
| `time.Duration` | Duration, automatically parses string formats | `30s`, `5m`, and `1h30m` are all supported |
| `time.Time` | Point in time, supports common date formats | Supports `2006-01-02` and `2006-01-02 15:04:05` |

For example, if you want to configure a timeout, you can write it directly like this:

```go
type Config struct {
	Timeout time.Duration `value:"${timeout:=30s}"`
}
```

If the configuration file contains `timeout=5m`, after binding it is directly `5 * time.Minute`. This is very convenient and does not require manual parsing.

#### Custom Type Converters

If you have your own custom type, you can also register a type converter to tell the framework how to convert a string into your type. Usage is very simple.

**When do you need a custom converter?**
- You define an enum type and need to parse it from friendly string names
- You need special format conversion logic, such as parsing encrypted data from a string
- Types from third-party libraries require custom parsing rules, and so on

**Complete example**:

```go
import (
	"strconv"
	"go-spring.org/spring/conf"
)

// Custom status enum type
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// String implements the Stringer interface for easier log output
func (s Status) String() string {
	switch s {
	case StatusDisabled:
		return "disabled"
	case StatusEnabled:
		return "enabled"
	default:
		return strconv.Itoa(int(s))
	}
}

// Register the converter in init() (registration must be completed before the program starts)
func init() {
	conf.RegisterConverter(func(s string) (Status, error) {
		switch s {
		case "disabled", "off":
			return StatusDisabled, nil
		case "enabled", "on":
			return StatusEnabled, nil
		default:
			// Also support direct numeric input
			v, err := strconv.Atoi(s)
			if err != nil {
				return 0, err
			}
			return Status(v), nil
		}
	})
}
```

After registration, you can use it directly in struct fields:

```go
type AppConfig struct {
	Status Status `value:"${app.status:=enabled}"`
}
```

Then you can write the configuration file like this:

```yaml
app:
  status: paused
```

> **Remember**: converters are registered globally. They must be registered in `init()`. Register once, and the entire application can use them.

#### Slice (Array) Binding

Slice types support two input methods, making them convenient and flexible for different scenarios:

**Method 1: multi-line expanded format (recommended for complex elements)**

This is common in YAML/TOML, with each element on its own line:

```yaml
apps:
  - a
  - b
  - c
```

After expansion:

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

This approach is suitable when elements are relatively complex, such as when each element is an object.

**Method 2: comma-separated string (suitable for simple lists)**

If it is just a simple string list, writing it on one line is more concise:

```properties
apps=a,b,c
```

Both forms ultimately bind to `[]string{"a", "b", "c"}` with the same result. By default, items are separated by an English comma `,`.

#### Map Binding

Map binding is also convenient. All child nodes with the path as a prefix are automatically bound:

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

If you bind this to `map[string]DatabaseConfig`, then `connections["master"]` and
`connections["slave"]` contain the corresponding configuration respectively. You do not need to traverse anything yourself; the framework handles it for you.

### Manual Bind Function Binding

Generally, manual binding is only used when creating multiple beans in a module, as shown below.

```go
package main

import (
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"github.com/go-spring/stdlib/flatten"
)

func init() {
	// Register a module
	gs.Module(nil, func(r gs.BeanProvider, p flatten.Storage) error {
		var config ServerConfig
		// Bind the configuration under the `${server}` prefix to the ServerConfig struct
		if err := conf.Bind(p, &config, "${server}"); err != nil {
			return err
		}
		// Use config to register related beans
		return nil
	})
}
```

The function signature of `Bind`:

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

Parameter descriptions:
- `storage` - configuration storage object containing all loaded configuration
- `target` - binding target; **must be a pointer**, otherwise the target variable cannot be modified
- `tag` - optional configuration item path to bind; supports the full tag syntax. If omitted, the entire configuration is bound

## Configuration Validation

Successful configuration binding does not mean the configuration is necessarily correct. For example, if you set a port number to `99999`, which exceeds the range 1-65535,
binding can still succeed, but the application will definitely run into problems.

Go-Spring supports validating configuration values so that **errors can be discovered during application startup**, avoiding deployment with incorrect configuration and eliminating problems early.

### Expression Validation

Go-Spring provides very flexible expression validation based on the excellent [`expr-lang/expr`](https://github.com/expr-lang/expr) library.

Usage is simple: add an `expr:"..."` tag to the struct field, and use `$` in the expression to represent the current field value.

Here are some of the most common expressions, available out of the box:

| Expression | Meaning |
|------------|---------|
| `$ > 0` | The current value must be greater than 0 |
| `$ < 65536` | The current value must be less than 65536 |
| `$ in ['debug', 'info', 'warn', 'error']` | Must be one of these enum values |
| `$ matches '^[a-z][a-z0-9_]{3,31}$'` | The string must match the regular expression |
| `$ contains 'prefix-'` | The string must contain this substring |
| `$ > 0 && $ < 65536` | Multiple conditions with an "and" relationship |
| `$ < 10 || $ > 100` | Multiple conditions with an "or" relationship |

Here are a few examples commonly used in day-to-day development:

```go
type ServerConfig struct {
	// The port number is valid only if it is between 1 and 65535
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`

	// The log level must be one of these four values
	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`

	// The username must follow the naming rules
	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{3,31}$'"`

	// Timeout must be at least 1 second
	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`

	// The retry count must not be too high or too low
	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

As you can see, a single expression handles the validation. You do not need to write a pile of `if-else` checks, so the code stays clean.
Putting configuration and validation together also makes maintenance easier.

The expr library supports a very rich syntax. Only the most common forms are listed here.
If you need more complex validation, see the official [expr-lang/expr](https://github.com/expr-lang/expr) documentation.

### Misconceptions About Required Validation

Here is a common question: **when do I need to write required validation myself?**

In fact, you do not need to worry about it: if your field has no default value and the key does not exist in the configuration,
then **the binding process has already failed**, so you do not need to write an extra expression.

There are only two cases where you need to write a validation expression yourself:

1. **You provided a default value, but the default value must also satisfy a condition** (for example, `port` defaults to 8080 but must be greater than 0)
2. **The field exists, but it must satisfy some business rule** (for example, `retry` must be between 0 and 10)

So remember this principle: **the framework already performs existence checks for you; you only need to additionally validate business rules**.

### Custom Validation Functions

If the built-in expression operations cannot meet your needs, you can also register global custom validation functions and use them directly in expressions.
A function accepts parameters of any type and returns `bool` to indicate whether validation passes.

**Complete example**:

```go
import "go-spring.org/spring/conf"

// Register custom functions in init()
func init() {
	// Register a function that determines whether a number is prime, requiring the port number to be prime
	conf.RegisterValidateFunc[int]("isPrime", func(n int) bool {
		for i := 2; i*i <= n; i++ {
			if n%i == 0 {
				return false
			}
		}
		return n > 1
	})

	// Register another function that checks the port range
	conf.RegisterValidateFunc[int]("validPort", func(port int) bool {
		return port > 0 && port < 65536
	})

	// Register a function that checks the minimum string length
	conf.RegisterValidateFunc[string]("minLength", func(s string) bool {
		return len(s) >= 3
	})
}
```

After registration, you can use them directly in tags:

```go
type ServerConfig struct {
	// The port number must be prime and also satisfy the port range
	Port      int    `value:"${port}" expr:"isPrime($) && validPort($)"`
	// The username must be at least 3 characters long
	Username  string `value:"${auth.username}" expr:"minLength($)"`
	// The API Key must satisfy multiple conditions
	APIKey    string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

Custom validation functions can be mixed with native expression operations, making it easy to build complex validation rules.
Your functions only need to return `true` or `false`; the framework automatically handles error messages.

## Configuration Loading: Sources and Formats

Now that we understand the configuration model, binding, types, and validation, let's look at where configuration comes from.
Go-Spring supports many configuration sources and formats, covering most use cases.

### Supported Configuration Formats

Go-Spring supports the four most common configuration formats out of the box:

| Format | File extension | Use case |
|:-------|:---------------|:---------|
| Properties | `.properties` | Simple key-value pairs, traditional format in the Java ecosystem |
| YAML | `.yaml`/`.yml` | Readable, supports comments, currently the most popular |
| TOML | `.toml`/`.tml` | Semantically clear, suitable for complex configuration |
| JSON | `.json` | Machine-friendly, commonly used for program-generated configuration |

The framework automatically selects the corresponding parser based on the file extension, so you do not need to care about parsing details.
If you have requirements for a special format, you can also register a custom parser.

### Custom Configuration Format Parser

If you need to support a special configuration file format, you only need to implement the `reader.Reader` function type
and then call `conf.RegisterReader` to register it.

**Complete example - custom INI format parser**:

```go
import (
	"go-spring.org/spring/conf"
)

// Implement INI format parsing
func parseINI(b []byte) (map[string]any, error) {
	// Call your preferred INI parsing library
	parsed, err := ini.Load(b)
	if err != nil {
		return nil, err
	}

	// Convert to a map[string]any tree structure and return it
	result := make(map[string]any)
	...
	return result, nil
}

// Register in init, binding the .ini extension
func init() {
	conf.RegisterReader(parseINI, ".ini")
}
```

After this registration, your application can directly load configuration files in `.ini` format, just like the built-in formats.

### Supported Configuration Sources

In addition to local files, Go-Spring also supports loading configuration from various remote configuration centers:

| Source | Description |
|:-------|:------------|
| Local file system | Most commonly used; loads configuration files from the local disk |
| Kubernetes ConfigMap (not yet supported) | Loads directly from ConfigMap when running on K8s |
| etcd (not yet supported) | Loads configuration from an etcd cluster |
| Nacos (not yet supported) | Loads from Alibaba Nacos configuration center |
| ZooKeeper (not yet supported) | Loads from ZooKeeper |

This list is still growing. Of course, you can also implement your own configuration provider (Provider) to connect to a custom configuration center.

### Custom Configuration Provider

A configuration provider is responsible for loading configuration data from a specific source (local files, remote services, databases, and so on).
If you need to load configuration from a special place (such as an internal company configuration center, etcd, a database, and so on),
you only need to implement the `provider.Provider` function type and then call `conf.RegisterProvider` to register it.

**Complete example - reading JSON configuration from environment variables**:

```go
import (
	"encoding/json"
	"fmt"
	"os"

	"go-spring.org/spring/conf"
	"github.com/go-spring/stdlib/flatten"
)

// Define a Provider that reads JSON from environment variables
func envJSONProvider(optional bool, source string) (map[string]string, error) {
	// The source parameter is the environment variable name
	envVal := os.Getenv(source)
	if envVal == "" {
		if optional {
			// Optional configuration does not exist; return nil without error
			return nil, nil
		}
		return nil, fmt.Errorf("environment variable %s not found", source)
	}

	// Parse JSON into a tree structure
	var tree map[string]any
	if err := json.Unmarshal([]byte(envVal), &tree); err != nil {
		return nil, err
	}

	// Flatten into a map and return it; keys are paths and values are strings
	return flatten.Flatten(tree), nil
}

// Register the Provider in init
func init() {
	conf.RegisterProvider("envjson", envJSONProvider)
}
```

After registration, you can use this custom Provider in configuration imports:

```properties
# Use a custom provider in spring.app.imports
# Format: <provider>:<source>
spring.app.imports=envjson:APP_CONFIG

# It can also be marked as optional so no error is reported if it does not exist
# Format: optional:<provider>:<source>
spring.app.imports=optional:envjson:LOCAL_OVERRIDES
```

Here, `APP_CONFIG` is the name of the environment variable. Set it before use:

```bash
# Export JSON configuration to an environment variable first
export APP_CONFIG='{"server":{"port":9000},"database":{"host":"localhost"}}'
```

After the application starts, the framework reads and parses JSON configuration from this environment variable.

### Environment Variables

Go-Spring automatically reads configuration from environment variables, which is especially useful for **container deployments**. The conversion rules are:

1. Environment variables starting with the `GS_` prefix are treated as configuration (this avoids loading all system environment variables, keeping things clean)
2. Then the `GS_` prefix is removed -> underscores `_` are replaced with dots `.` -> finally converted to lowercase

Here is an example that makes it clear:

```bash
# Environment variables you set
export GS_SERVER_PORT=8080
export GS_DATABASE_DEFAULT_HOST=localhost

# The framework automatically converts them to
server.port=8080
database.default.host=localhost
```

This perfectly matches the path syntax described earlier.

In addition, you can also **directly bind any environment variable** without following the `GS_` prefix rule:
you only need to use the environment variable name directly in the configuration (environment variables are usually named with uppercase letters,
so the key here also needs to exactly match the environment variable name).

Example:

```go
type ServerConfig struct {
	Port int `value:"${PORT}"`
}
```

In this case, the framework directly reads the value of the system environment variable `PORT` for binding.

### Command-Line Arguments

When starting an application, you can also **temporarily override configuration** through command-line arguments, which is very convenient during development and debugging. The rules are:

- Arguments starting with the `-D` prefix are treated as configuration items
- Both `-Dkey=value` and `-D key=value` forms are supported
- If only `-Dkey` is written without a value, the default value is `true`

Here is a complete example:

```bash
./myapp -Dserver.port=9000 -Denv=prod -Ddebug
```

It is parsed as:

```
server.port=9000
env=prod
debug=true
```

If you do not like the `-D` prefix, you can modify it through the environment variable `GS_ARGS_PREFIX`:

```bash
export GS_ARGS_PREFIX="--config."
./myapp --config.server.port=9000
```

## Hierarchical Configuration and Priority

Go-Spring supports loading configuration from **multiple sources** (base configuration files, Profile configuration files, environment variables, command line, and so on).
These configurations may overlap, so the final effective value must be determined.

The framework merges configuration in priority order. The complete priority order, from high to low, is:

| Priority | Source |
|:--------:|--------|
| 1 (highest) | **Command-line arguments** |
| 2 | **Environment variables** |
| 3 | **Profile-specific configuration** |
| 4 | **Base configuration files** |
| 5 | **Application built-in default configuration** |
| 6 (lowest) | **Struct tag default values** |

This priority order is consistent with Spring Boot and matches common usage habits:
**temporary command-line overrides have the highest priority, followed by environment variables, then configuration files, and finally default values**.

You only need to remember one core principle: **the more specific and closer to the runtime environment a configuration is, the higher its priority**.

### Merge Semantics: Different Rules for Three Types

When multiple configuration sources are merged, Go-Spring uses different merge semantics for different types of configuration.
Understanding this is central to designing configuration. Once you understand it, the merge result will not be confusing:

| Configuration type | Merge semantics | Description |
|:-------------------|:----------------|:------------|
| **Leaf value** | **Override semantics** | A leaf value with the same key in a higher-priority configuration directly overrides the lower-priority value, and the search stops once found |
| **Map object** | **Merge semantics** | Keys from all layers are merged together; different layers can complement each other |
| **Slice array** | **Override semantics** | Once a higher-priority layer defines this array, the entire lower-priority array is ignored and completely replaced |

Let's look at three examples. After reading them, you will understand.

**Example 1: leaf value override**

This is the most common scenario. Suppose the base configuration contains `app.port = 8080`, and you want to change it to 9000 in the development environment. You only need to write:

```properties
# Low priority (base configuration)
app.port = 8080

# High priority (environment override)
app.port = 9000
```

The merge result is `app.port = 9000`. The higher priority directly overrides it, which is exactly what you would expect.

**Example 2: Map merge**

Map objects use **merge semantics**, so keys from different layers are merged together:

```properties
# Low-priority configuration (base configuration)
server.port=8080

# High-priority configuration (environment override)
server.host=localhost
```

The merged Map contains two keys: `port` and `host`. The value of `port` comes from the lower priority, and `host` comes from the higher priority.
The result is:

```properties
server.port=8080  (kept from lower priority)
server.host=localhost (from higher priority)
```

If a key is duplicated, the value still follows override semantics: higher priority overrides lower priority.

This merge method is very convenient: you only need to write the keys that need to change in the higher-priority configuration, without rewriting the entire Map.

**Example 3: Slice array override**

Slice arrays differ from Maps and use **whole replacement** semantics:
once a higher-priority layer defines this array, the entire lower-priority array is ignored.

```
# source1 (low priority):
my.list[0]=a
my.list[1]=b

# source2 (high priority):
my.list[0]=c
```

The final result is `[c]`, not `[c, b]`. Because the higher priority has already defined `my.list`, the entire lower-priority array is ignored.

This is a design decision: for arrays, you usually either **redefine them completely** or do not define them at all.
Partial array modification is uncommon in practice and can easily cause confusion.
Therefore, Go-Spring chooses simple and clear semantics: **replace the entire array**.

## Profile Multi-Environment Configuration

Configuration often differs between development, testing, and production environments: development uses a local database, production uses a production database, port numbers may also differ, and so on.

Go-Spring supports multi-environment configuration through the **Profile mechanism**, which is exactly the same concept as Spring.
With Profiles, you can store configuration for different environments separately and load it as needed.

### Activating Profiles

You can activate Profiles in two ways:

```bash
# Command-line argument
./app -Dspring.profiles.active=prod

# Environment variable
export GS_SPRING_PROFILES_ACTIVE=prod
```

Activating multiple Profiles at the same time is supported. Separate them with commas:

```bash
-Dspring.profiles.active=prod,metrics
```

### Configuration File Naming Convention

Go-Spring follows the same naming convention as Spring Boot:

- `app.yaml` - **base configuration**, effective in all environments
- `app-{profile}.yaml` - **Profile-specific configuration**, with higher priority than base configuration

So your project structure usually looks like this:

```
conf/
  app.yaml          # Common base configuration shared by all environments
  app-dev.yaml      # Special configuration for the development environment
  app-test.yaml     # Special configuration for the testing environment
  app-prod.yaml     # Special configuration for the production environment
```

When the `prod` Profile is active, the load order is:

1. First load the `app.yaml` base configuration
2. Then load the `app-prod.yaml` Profile configuration
3. Profile configuration overrides keys with the same name in base configuration

This implements the best practice of **"share base configuration and put only differences in environment configuration"**, avoiding duplicate code.

### Custom Configuration Directory

By default, Go-Spring loads configuration files from the `./conf` directory.
If you want to use another directory, you can modify it through `spring.app.config.dir`:

```bash
# Specify the configuration directory through an environment variable
export GS_SPRING_APP_CONFIG_DIR=./config

# Or specify it through a command-line argument
./myapp -Dspring.app.config.dir=./config
```

Then `spring.app.config.dir` itself is parsed according to the normal priority rules,
so you can override it through environment variables, command-line arguments, and other methods.

### Priority of Multiple Profiles

When multiple Profiles are active at the same time, **later Profiles have higher priority than earlier ones**. For example:

```
spring.profiles.active=dev,metrics
```

`metrics` has higher priority than `dev`. If they contain the same configuration item, the value from `metrics` overrides the value from `dev`.
This is intuitive: later entries have higher priority.

### Design Recommendation: Keep Profiles Orthogonal

When designing Profiles, it is recommended to keep the Profiles **orthogonal** to each other:

- Each Profile should be responsible for configuration changes in **one dimension** only
- Avoid dependencies or overlapping configuration between multiple Profiles
- Configuration from different dimensions should be **independently composable**

For example:
- `dev`/`test`/`prod` are the **environment dimension**, representing different runtime environments
- `metrics`/`trace` are the **feature dimension**, representing whether specific features are enabled

These two dimensions are orthogonal. You can freely combine `dev,metrics` or `prod,metrics` without preparing a separate configuration file for every combination.

## Configuration Imports

Sometimes you want to **split one large configuration file into several smaller files** for easier maintenance, or you want to load some configuration from a remote configuration center.
This is when you can use configuration imports:

```properties
# Import other configuration files in the main configuration, separated by commas
spring.app.imports=./dev.properties,http://config-server/app.properties

# The optional: prefix means this configuration file is optional, and no error is reported if it does not exist
spring.app.imports=optional:./local.overrides
```

Imports can be used in both base configuration and Profile configuration.

- If other configuration is imported in `app.yaml` (base configuration), the imported configuration has higher priority than `app.yaml` itself
- If other configuration is imported in `app-prod.yaml` (Profile configuration), the imported configuration also has higher priority than `app-prod.yaml` itself

The core principle here is: **later-loaded configuration has higher priority**.
The configuration system loads configuration in discovery order; configuration discovered later and loaded later has higher priority.

By using imports this way, you can extract common configuration into separate files and reuse it across different environments conveniently.

## Variable References

Configuration supports full variable reference syntax. You can reference values of other configuration items in any configuration value. This is very useful in many scenarios, such as:
- Extracting common prefixes for reuse in multiple places
- Combining multiple configuration items into a new value
- Referencing environment variables
- Providing flexible default values, and so on

### Common Usage Examples

```properties
# 1. Directly reference another configuration item
server.port=${port}

# 2. With a default value; if port cannot be found in configuration, use 8080
server.port=${port:=8080}

# 3. Combine multiple configuration items into a new value; mixing with normal text is supported
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api

# 4. Directly reference an environment variable
redis.password=${REDIS_PASSWORD:=}
```

### Nested References

The framework supports **nested references**, meaning another reference can be used inside a reference, and arbitrary depth is supported:

```properties
env=prod
config.file=config/${env}.properties
```

The framework automatically and recursively resolves all dependencies and ensures correct expansion.

In short, variable references make configuration more flexible. You can write very concise configuration through composition and reuse.

## Dynamic Configuration

In many cases, you may need to **refresh configuration without restarting the application**. Go-Spring natively supports dynamic configuration,
and the syntax is exactly the same as static configuration, so there is no learning curve.

You only need to declare your field type as the `gs.Dync[T]` generic:

```go
import "go-spring.org/spring/gs"

type AppConfig struct {
	// Static configuration; it does not change after startup
	Port int `value:"${server.port}"`

	// Dynamic configuration; it can be automatically refreshed at runtime
	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

When using it, call the `Value()` method to get the latest current value:

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Each read gets the latest value
	timeout := a.Config.Timeout.Value()
	// ...
}
```

`Dync[T]` is concurrency-safe, so multiple goroutines can read it at the same time without any problem.

Dynamic refresh guarantees **atomic commit**: either all configuration updates succeed, or none are updated. There will be no intermediate state with partial updates.
To guarantee this property, the framework **pre-validates all configuration** before refreshing. If validation fails, the entire refresh is canceled
and the configuration currently in use is not affected.

It is worth noting that for resources such as connection pools that need dynamic refresh, we do not need a special mechanism to satisfy their requirements.
Resources generally have a lifetime and do not instantly switch to new resources;
instead, you need to control a smooth transition at the business layer (for example, gradually reclaiming old connections).
Therefore, the framework only needs to provide the core semantics of **dynamically refreshed values**. When new connections are created, the latest configuration is naturally applied.

In addition, we recommend using a **version number mechanism** at the business layer to avoid unnecessary refreshes.
This way, real resource reloading is triggered only when the version changes.
