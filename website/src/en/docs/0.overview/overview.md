# Project Overview

Go-Spring is a **lightweight application development framework** for Go. It draws on excellent design ideas proven over many years in the Java Spring framework,
while staying true to Go's native simplicity and efficiency. It avoids overengineering and unnecessary complexity, aiming to balance engineering capability with developer experience.

## Why Build Go-Spring?

Since its creation, Go has quickly gained a strong foothold in server-side development thanks to its simple syntax, high performance, and excellent concurrency model.
However, the community still lacks a widely recognized, unified approach for elegantly building server-side applications.

### Pain Points of Native Development

When building server-side applications from scratch with native Go, developers often encounter a set of common problems:

- **Scattered configuration management**

  Configuration loading, struct binding, and parameter validation often need to be implemented manually. Projects repeatedly reinvent the wheel and lack unified conventions.

- **Cumbersome dependency management**

  Component dependencies are organized through manual instantiation, which creates tight coupling and makes testing, extension, and replacement harder.

- **No unified lifecycle management**

  Each component controls its own startup and shutdown process. Without coordination, graceful shutdown is difficult to implement correctly.

- **Repeated work integrating basic components**

  Each time components such as MySQL or Redis are introduced, developers need to rewrite boilerplate such as connection initialization and configuration parsing, which is inefficient.

These problems are not defects in Go itself. They arise from the lack of a unified foundation framework that provides standardized solutions.

### Trade-offs in Existing Solutions

The Go community already has several dependency injection frameworks, but each focuses on different concerns and is difficult to use as a complete solution:

- **wire**

  Implements dependency injection through compile-time code generation. It uses no reflection and has excellent performance, but the workflow is relatively cumbersome, and code must be regenerated after every change.

- **dig/fx**

  Provides constructor-based dependency injection and powerful capabilities, but relies on runtime resolution. Its overall design is relatively complex and has a higher learning curve.

Fundamentally, these frameworks **mainly focus on dependency injection itself**. They provide limited support for infrastructure such as configuration management, logging, lifecycle control, and component integration,
so developers still need to assemble those pieces themselves.

Meanwhile, Web frameworks such as gin, echo, and fiber focus on HTTP routing and do not take responsibility for the application infrastructure layer.

### Go-Spring's Vision

Go-Spring's vision is to build a **complete, one-stop application development framework** that solves these core problems in a unified way:

- Configuration management
- Dependency injection
- Lifecycle control
- Basic component integration

By providing a systematic solution, Go-Spring helps developers break free from repetitive infrastructure work, focus more on business logic, and build applications efficiently and elegantly.

### Go-Spring's Positioning

From a core design perspective, Go-Spring is built on four foundational capabilities: **configuration management, logging, dependency injection, and lifecycle management**.
On top of these, it extends into a complete ecosystem that can support HTTP, gRPC, and other application scenarios.

Therefore, Go-Spring's positioning can be understood from two perspectives: "cooperation" and "competition".

#### Cooperation: Coexisting with Domain Frameworks

- **Does not replace mainstream Web frameworks**

  Go-Spring does not replace frameworks such as gin, echo, or fiber that focus on HTTP routing.
  You can still use the tools you are familiar with, while Go-Spring handles configuration, dependencies, and lifecycle management in a unified way.

- **Keeps a simple Go-style design**

  Go-Spring does not copy Java Spring's heavyweight system. Instead, it borrows its core ideas while preserving Go's usual lightweight and clear style.

- **Uses a "startup-time IoC" mechanism**

  All dependencies are injected during application startup. Runtime dynamic injection is not supported. This design significantly reduces complexity while avoiding runtime performance overhead.

#### Competition: Competing with Complete Application Frameworks

- **Direct competition through one-stop capabilities**

  When you need a complete solution covering configuration, dependency injection, logging, and component integration,
  Go-Spring naturally competes with full-stack microservice frameworks such as go-zero, kitex, Kratos, and go-frame.

- **Core capabilities can be split out and reused**

  Even in competitive scenarios, Go-Spring's core modules (configuration, dependency injection, logging) remain highly independent
  and can be integrated into other frameworks to complement and coexist with them.

Go-Spring always maintains an open mindset. In the future, it will actively collaborate with the community, integrate more excellent projects, and gradually build a more complete ecosystem.

## Go-Spring's Design Goals

Go-Spring's design is centered on eight core principles, aiming to balance "ease of use" with "flexibility".

### Convention over Configuration

Go-Spring provides a set of reasonable default conventions, allowing developers to start projects quickly without complex configuration in most scenarios. For example:

- Logs are output to the console by default, with the INFO level, ready to use out of the box
- The HTTP service listens on `:8080` by default, while still supporting customization as needed
- pprof listens on `:6060` by default and can be disabled through configuration

All default behaviors can be flexibly adjusted through configuration. From log output methods to component startup and shutdown controls, everything is highly customizable.

In short:

- **Convention** helps you get started quickly
- **Configuration** helps you handle complex scenarios flexibly

### Startup-Time Dependency Injection

This is one of the most important design decisions in the Go-Spring architecture:
**all dependencies are injected only during application startup, and Beans cannot be dynamically retrieved or injected at runtime**.

This choice is mainly based on the following considerations:

- **Simplicity**

  Dependency injection runs only once at startup. There is no need to maintain complex metadata structures at runtime, reducing system complexity and saving memory.

- **Predictability**

  All dependencies are resolved during startup, avoiding uncertain issues such as "missing dependencies" during runtime and making system behavior more stable and easier to reason about.

- **High performance**

  No reflection or dynamic dependency resolution is needed at runtime. All components are already assembled at startup and can be used directly, resulting in stable and efficient performance.

- **Consistent with Go's engineering philosophy**

  Go encourages discovering problems as early as possible at compile time or startup time and failing fast. This design closely matches Go developers' habits.

Of course, this choice also means giving up advanced features such as "runtime dynamic injection". But in the vast majority of server-side application scenarios, these capabilities are not essential.
Trading lower complexity for higher performance and maintainability is a more cost-effective compromise.

> Unlike Java Spring, Go-Spring **does not require interfaces for dependency injection**.
> In most scenarios, injecting concrete types directly is enough; there is no need to over-abstract purely for "decoupling". Interfaces are recommended only when multiple interchangeable implementations are truly needed.

### Container-Level Singletons

All Beans in the container are singletons and unique for the lifetime of the container.

**Why design it this way?**

Because in most business scenarios, one component only needs one instance.
For example, your `UserService` or `DataSource` only needs one instance for the whole application. There is no need to create a new one every time it is used.

If you truly need multiple different instances of the same type (for example, one primary data source and one replica data source), you can register them as different **named Beans**
and select by name during injection. This covers most scenarios that require multiple instances.

### Explicit over Implicit

Go-Spring insists on explicit declarations and avoids implicit automatic inference:

- Interface exports must be **explicitly declared** and are not inferred automatically
- Dependencies are clearly visible; the code itself is the documentation

**Why design it this way?**

Go itself is an explicit language. Although implicit automatic inference may save a few lines of code, it can easily produce surprises.
For example, if a struct accidentally implements all methods of an interface and is automatically inferred and registered by the container, that is usually not what you want.

Explicit declarations make dependencies clear at a glance. You can see all exported relationships by reading the code, without extra explanations.

### Conditional Mutual Exclusion over Override

Beans with the same name and same type cannot override each other. Which Bean takes effect must be determined by conditional matching.

Java Spring allows multiple Beans with the same name and then selects the primary version through `@Primary`.
Go-Spring considers this implicit selection error-prone, so it insists on conditional mutual exclusion.

**Why design it this way?**

If Bean overriding were allowed, it would be hard to tell which Bean overrides which, and debugging would be difficult when problems occur.
Conditional matching makes it explicit when each Bean takes effect.
If a conflict appears during startup, the framework reports an error directly and asks you to handle it. This is far better than silently overriding for you.

In practice, this design also feels natural:
different environments (dev/prod) need different implementations. Register them with different conditions and use configuration switches to decide which one is enabled. This matches the simplest logical habit.

### Go Semantics First

Go-Spring does not deliberately recreate Java Spring's object-oriented model. Instead, it always adheres to Go's native semantics and development habits:

- Error handling follows Go's `error` return mechanism rather than an exception system
- Complex mechanisms such as AOP or dynamic proxies are avoided, keeping code transparent, readable, and debuggable
- Interfaces are native Go interfaces, with no framework-specific markers or constraints

Ultimately, Go-Spring's philosophy is: **let Go remain Go**.
The framework only manages dependencies and lifecycle; it does not change your coding style or mental model.

### Modularity and Componentization

Go-Spring encapsulates components through the Starter mechanism, achieving true **on-demand introduction**:

- Need MySQL? Introduce the corresponding MySQL Starter
- Need Redis? Introduce the corresponding Redis Starter
- Unused components are not compiled into the final artifact, avoiding unnecessary size growth

Each Starter is an independent module responsible for its own configuration parsing and component registration.
Developers do not need to write verbose initialization code. Integration can be completed with a **blank import**, greatly reducing onboarding cost.

### Unified Lifecycle Management

Go-Spring uniformly schedules and manages the full application lifecycle from startup to shutdown:

- **Clear and orderly startup process**

  Initialization proceeds step by step in the order of "configuration → logging → IoC → Runner → Server", with clear layers.

- **Graceful and reliable shutdown process**

  Services (Server) are stopped first, then dependencies (IoC) are released, and finally logs are flushed to ensure resources are reclaimed correctly and logs are not lost.

- **Unified interface conventions**

  All components follow the unified `Run / Stop` interface and are orchestrated and scheduled by the framework.

Developers no longer need to manually coordinate component startup and shutdown order. The framework handles these details for you, making applications more stable and controllable.

## Go-Spring Features

### Powerful Configuration Management

* Multi-format support: natively compatible with `properties`, `YAML`, `TOML`, and `JSON`
* Multi-source access: supports local files, environment variables, command-line arguments, and extensible integration with configuration centers such as K8s ConfigMap, etcd, Nacos, and ZooKeeper
* Flexible variable mechanism: supports `${key}` and `${key:=default}`, making configuration reuse and default values easy
* Direct struct binding: configuration can be bound directly to Go structs, with native support for nested structs, slices, and maps
* Built-in type conversion: supports common types such as `time.Time` and `time.Duration` out of the box
* Complete validation system: supports required fields, ranges, enums, regular expressions, and custom rules
* **Dynamic hot update**: configuration refreshes automatically at runtime without restarting the service

### Lightweight and Efficient IoC Container

* Completes all dependency injection at startup, with zero extra runtime overhead
* Conditional assembly based on Profiles and Conditions, adapting flexibly to different environments
* Explicit dependency declarations: rejects implicit inference and avoids "magic behavior"
* On-demand instantiation: creates only Beans that are actually depended on, reducing resource consumption
* No override design: forbids Bean overriding and uses conditions for clear mutual exclusion
* Supports constructor injection plus struct field injection, balancing flexibility and readability

### Controllable Application Lifecycle

* Linear startup mechanism: any step failure terminates startup immediately, avoiding a "half-started" state
* Graceful shutdown: captures system signals and safely releases resources in dependency order
* Global Context throughout: unified lifecycle management that prevents abuse of `context.Background`
* Dual startup modes:
    * `Run()`: standard mode, fully managed application lifecycle
    * `RunAsync()`: compatibility mode, easily integrated into existing projects

### Business-Oriented Logging System

* 🌟 **Tag routing (core innovation)**: routes and splits logs based on business tags to precisely control log destinations
* Fully structured logs: represented uniformly as Fields, naturally fitting log analysis and search systems
* Pluggable outputs: supports Console, files, and time-rotated files with automatic cleanup
* High-performance writing: supports synchronous/asynchronous modes; asynchronous writing does not block business logic
* Multi-format support: built-in Text and JSON formats, with support for custom formats
* Automatic trace injection: automatically extracts Trace information from `context.Context` to connect call chains

### Flexibly Integrated HTTP Server

* Fully compatible with Go's standard library `net/http`, seamlessly integrating with any third-party router framework
* Lifecycle is uniformly managed by the framework, automatically handling startup and shutdown
* Native graceful shutdown support ensures requests are processed safely before exit
* Supports configuration of key parameters such as port and timeouts, and can also be completely disabled as needed

### Out-of-the-Box Starter Mechanism

* Modular component encapsulation, automatically registered through `blank import`
* Provides three registration methods:
    * `provide`: basic component registration
    * `module`: modular encapsulation
    * `group`: supports multi-instance scenarios such as multiple data sources or clients
* Configuration-driven enable/disable behavior, achieving true "load on demand"
* Instantiates by dependency relationships; unused components are not created, avoiding resource waste

### Go-Idiomatic Testing Support

* Fully compatible with native `go test`, with zero learning cost
* Supports IoC test mode: `RunTest()` starts the application context with one call and automatically isolates test data
* Built-in fluent assertion mechanism, supporting both `assert` and `require` styles
* Integrated Mock capabilities: supports interface mocking and method-level monkey patching

### Contract-Driven HTTP Code Generation

* Describes HTTP / RPC interfaces based on IDL (Interface Definition Language), providing unified contracts
* **Define once, generate for both sides**: automatically generates server skeleton code and client invocation code
* Complete type system support: covers primitive types, lists/maps, enums, oneof (union types), and generic structs
* Supports `required` / `optional` field modifiers to clearly express data constraints
* Rich syntax capabilities: supports constants, custom annotation extensions, normal RPC, and SSE streaming interfaces
* Automatically completes HTTP parameter binding: path parameters, query parameters, request headers, and request bodies are uniformly mapped
* Built-in high-performance validation mechanism: generates validation logic based on expressions, with no reflection overhead

## Go-Spring vs Other Solutions

### vs Pure Manual Wiring

Using purely manual dependency assembly is simple, direct, framework-free, and gives you full control over all logic.
However, this approach is acceptable only when the project is small. As the business grows, problems gradually emerge:
large amounts of repeated component creation and dependency assembly code, repeated handwritten configuration parsing, and no unified lifecycle management, eventually causing maintenance costs to rise quickly.

**Go-Spring's value lies in**:
encapsulating common foundational capabilities such as configuration loading, dependency management, and lifecycle control, so you can focus on business logic instead of reinventing the wheel.

### vs Wire

[Wire](https://github.com/google/wire) is a compile-time dependency injection tool from Google.
It is based on compile-time code generation, requires no reflection, and has excellent runtime performance. Its shortcomings are: code must be regenerated for every dependency change,
the workflow is relatively cumbersome, and it only solves dependency injection. Configuration, logging, lifecycle, and other concerns still need to be integrated manually, and support for multi-environment scenarios is weak.

**How Go-Spring differs**:

- No code generation is required; changes to dependencies or configuration take effect after restarting, making the development experience smoother
- Provides a one-stop solution for configuration, logging, dependency injection, and lifecycle
- Natively supports conditional injection and multi-environment configuration out of the box

### vs dig/fx

[dig](https://github.com/uber-go/dig)/[fx](https://github.com/uber-go/fx) are dependency injection frameworks from Uber.
They are powerful, support runtime dynamic injection, have a relatively mature ecosystem, and are suitable for complex systems.
However, their design is more complex, introducing many concepts (modules, Providers, Decorators, lifecycles, etc.).
They have a higher learning curve, and because they support runtime injection, they need to retain complete dependency metadata, resulting in some runtime overhead.

**Go-Spring's trade-offs**:

- Uses a "startup-time one-shot injection" strategy. All dependencies are resolved during startup, with no extra metadata needed at runtime, making it lighter overall
- Concepts are streamlined, the learning curve is gentle, and daily usage only requires attention to core injection methods such as `@Autowired`
- The configuration system is deeply integrated and covers complete processes such as configuration binding and validation, which most DI frameworks do not solve systematically

Overall, Go-Spring is not a single-point optimization of one capability such as DI. Instead, it starts from the **overall application infrastructure perspective**
and provides a more complete, unified solution that balances development efficiency, maintainability, and complexity.

### vs Java Spring

Many people see the name Go-Spring and mistakenly think it is a direct port of Java Spring, but the two are fundamentally different:

- **Java Spring** has evolved for more than twenty years into an extremely complete enterprise framework,
  covering many advanced features such as runtime dynamic weaving, AOP, and automatic scanning. Its system is large and complex.

- **Go-Spring** focuses on "taking the essence". It keeps only the core ideas proven effective in Spring's long-term practice
  (such as configuration management, dependency injection, and the Starter pattern), and reimplements them in a native Go way, avoiding complex mechanisms that do not fit Go style.

Therefore, Go-Spring is not a simple copy of Java Spring. It is a **concept-based reconstruction**: inheriting the ideas while adhering to Go's simplicity and clarity.

### vs Full-Stack Microservice Frameworks

go-zero/kitex/Kratos/go-frame are mature full-stack microservice solutions in the Go ecosystem that have been validated in large-scale production.
They have complete microservice ecosystems with built-in service discovery, circuit breaking and degradation, rate limiting, tracing, observability, code generation, and more. They are basically ready to use out of the box,
have abundant community resources, and make it easy to build systems from scratch.

#### Go-Spring's Current State

As a later project, Go-Spring still has gaps in the completeness and richness of its microservice ecosystem, and official components are still being continuously improved.
The current core capabilities (configuration, dependency injection, logging, and lifecycle management) are relatively stable, but the surrounding ecosystem still needs time to mature.

#### Go-Spring's Advantages

- **A simpler core design**

    - The configuration system supports multiple formats and sources, with default values, environment variable overrides, dynamic refresh, and other capabilities while keeping the design clear
    - Dependency injection supports both constructor injection and field injection, with intuitive conditional injection logic and no implicit behavior

- **Higher modularity**

    - Use only the configuration module ✔
    - Use only the DI module ✔
    - Combine with any Web framework ✔
    - Use the complete framework ✔

  Everything is selected on demand, rather than being forcibly bound together.

- **A design philosophy that better fits Go**

  It emphasizes simplicity, explicitness, and low magic, avoiding overengineering and making code easier to understand and maintain.

The most important point is: even if you have already chosen one of these mature microservice frameworks, Go-Spring's core capabilities (configuration, DI, logging)
can still be **extracted independently and integrated into them** to enhance an existing system rather than replace it.

This makes Go-Spring not only an "alternative option", but also a set of **composable foundational capability components** that can truly cooperate and coexist with the existing ecosystem.

## Go-Spring's Design Philosophy

### Simplicity First

Go-Spring always follows the design principle of "**simplicity first**":

- Work that can be completed at startup should not be moved to runtime
- Logic that can be expressed explicitly should not rely on implicit "magic"
- Avoid reflection as much as possible; when it is necessary, strictly limit it to the startup phase
- Reject overengineering and do not introduce unnecessary complex features

In short: **solve problems in the most direct way, not in the flashiest way**.

### Modular Design

Go-Spring uses a highly modular architecture. Functional modules are decoupled from each other and can be used on demand:

- If you do not need HTTP capabilities, you can completely avoid introducing related modules
- If you do not use http-gen, it will have no impact on your project
- The framework does not force any technology choices

This design lets you freely combine capabilities instead of being "bundled" by the framework.

### Progressive Adoption

Go-Spring supports multiple adoption modes, fitting projects at different stages and with different needs:

- **Full adoption**: use the framework's capabilities across configuration, dependencies, and lifecycle
- **Partial usage**: use only the configuration module while keeping the rest of the original implementation
- **Capability splitting**: use only dependency injection (DI), while choosing any Web framework freely

The framework does not force a unified solution. Instead, it provides flexible capability assembly: **use as much as you need**.

## Summary

Go-Spring tries to solve the repeated, tedious, and inefficient foundational problems in server-side development:

- **Unified configuration management**

  No need to reimplement configuration loading logic in every project; the framework has built-in support for configuration binding, validation, and dynamic refresh.

- **Clear dependency injection mechanism**

  Dependency injection decouples components, making code structure clearer and making unit testing and mocking easier.

- **Consistent lifecycle management**

  The framework uniformly schedules component startup and shutdown processes, requiring no manual orchestration and ensuring graceful shutdown without losing requests.

- **Out-of-the-box official components**

  Common components such as MySQL, Redis, and pprof provide official Starters and can be used by importing them and adding a small amount of configuration.

- **Unified development experience**

  Teams share consistent project structure and configuration conventions, making onboarding easier for new members and code easier to maintain.

It should be clear that Go-Spring is still growing:
it is not yet a fully complete "all-in-one" framework, and it does not try to make all decisions for you.
As a relatively young project, its microservice ecosystem still has room to improve, and it still needs time to mature compared with established frameworks.

But Go-Spring always insists on:

- Maintaining Go-style simplicity and clarity
- Providing a set of cooperative and composable core modules
- Helping developers build server-side applications efficiently

We maintain an open attitude and encourage coexistence and collaboration with other frameworks. In the future, we will continue to work with the community, integrate more excellent projects,
and gradually build a more complete and powerful ecosystem.
