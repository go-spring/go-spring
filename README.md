# Go-Spring: Make Go Service Development as Simple as Spring Boot, and Then Some

<div align="center">
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" alt="logo"/>
</div>

> **If you think this is just another Go framework, keep reading.**
>
> Go-Spring's mission is **building a complete, vendor-neutral application ecosystem for Go** — the assembly layer the Go ecosystem is missing: DI libraries ship parts, framework ecosystems orbit their own transport, component libraries don't know each other. Go-Spring assembles them all.
>
> It takes the battle-tested paradigms from two decades of Java Spring—dependency injection, auto-configuration, the Starter mechanism—and reimagines them in idiomatic Go. Spring rescued Java from EJB hell, turning heavyweight applications into composable, reusable, modular engineering. Go-Spring aims to give Go developers that same superpower.
>
> **But that's not the whole story.** Go-Spring is attempting something bigger: [Process as Code](MANIFESTO.md)—treating the software development process itself as an assemblable, reusable, versionable "application." Application assembly and development workflows share the same IoC philosophy; only the assembly target shifts from runtime components to development actions. It's a bold direction, and a hypothesis worth testing.

## The Ecosystem

Go-Spring is not a single repository—it's a complete R&D ecosystem composed of a **core framework, an ecosystem abstraction library, 70+ Starters, developer tooling, example applications, and project templates**. The layers are strictly ordered — dependencies flow one way, downward — and each layer has a clear role; use what you need.

| Layer | Role | Key Projects |
|---|---|---|
| **Foundation** | Zero-dependency utilities + pure semantic pieces (HTTP client/server, …) + structured logging engine | [`stdlib`](stdlib/), [`log`](log/) |
| **Core** | IoC container, DI, layered config engine, application lifecycle — nothing else | [`spring`](spring/) (`gs` + `conf`) |
| **Ecosystem Library** | Container-free capability abstractions: governance center, discovery, cache, repository, i18n/validation, … — imports **no** spring package | [`cloud`](cloud/) |
| **Integration** | 70+ pluggable Starters: third-party SDKs + gs wiring | [`starter/`](starter/) — Gin, gRPC, Redis, MySQL, Kafka, Dubbo, Kitex… |
| **Tooling** | CLI, code generation, mocking | [`gs`](gs/gs), [`gs-http-gen`](gs/gs-http-gen), [`gs-mock`](gs/gs-mock) |
| **Examples & Templates** | End-to-end example apps + project scaffolds | [`examples/`](examples/), [`contrib/`](contrib/), [`layout/`](layout/) |

The rule of thumb that keeps the stack clean: **abstractions go in `cloud`, third-party SDKs and gs wiring go in a starter, pure semantics with no ecosystem dependency go in `stdlib`** — a starter wires, cloud abstracts, spring runs.

Full module inventory and architecture constraints: [ARCHITECTURE.md](ARCHITECTURE.md).

## Why Go-Spring

### The Mission: Completing the Go Ecosystem

Go's standard library is famously complete for a language runtime — and deliberately minimal at the application level: no dependency-injection model, no layered-configuration convention, no starter-style auto-configuration, no unified application lifecycle or governance layer. Java fills that gap with Spring; **Go has nobody filling it** — DI libraries ship parts, framework ecosystems orbit their own transport, component libraries don't know each other.

Go-Spring exists to fill exactly that gap: **building a complete, vendor-neutral application ecosystem for Go** — not another RPC framework, not another web framework, and not a rewrite of anything you already use. It competes with nothing in your stack; it assembles your stack.

Who else plays this seat? Honest landscape:

| Adjacent players | Examples | Why they're not this |
|---|---|---|
| DI libraries | Wire, fx, dig | Solve injection only — no config engine, no starter model, no lifecycle or governance. Parts, not an ecosystem. |
| Framework-centric ecosystems | Kratos, go-zero, CloudWeGo | Excellent, but the ecosystem orbits **their own** framework: transport, layout, codegen, tooling. Adopting one is a commitment. |
| Component libraries | dubbo-go, Kitex, Hertz, GORM, Gin… | **Peers, not competitors** — Go-Spring integrates them all symmetrically as Starters. |

The neutral Spring-Boot-shaped application platform for Go is an essentially empty niche. That is the seat Go-Spring takes.

### Not Another RPC Framework — an Application Platform

This is the sharpest day-to-day difference from projects like **dubbo-go, Kitex, Kratos, or go-zero**: those are (primarily) **RPC / microservice frameworks** — they own the transport layer, the service model, and often a code-generation pipeline. Your application *is* a dubbo application or a Kitex application; adopting one means adopting its programming model.

Go-Spring owns **no protocol and no transport**. It is the **assembly and runtime layer beneath and beside them**: dependency injection, layered configuration with hot reload, lifecycle management, and centralized service governance. Proof by construction — in this very repository, dubbo-go, Kitex, Kratos, go-zero, GoFrame, gRPC, tRPC, and Thrift each appear as **one Starter among 70**, integrated by the same `gs.Module` mechanism as Redis or MySQL:

| | dubbo-go / Kitex / go-zero / Kratos | Go-Spring |
|---|---|---|
| **Category** | RPC / microservice framework (owns transport, service model, codegen) | Application assembly & runtime platform (IoC + config + lifecycle + governance) |
| **Relationship** | *Integrated by Go-Spring* — `starter-dubbo` wraps dubbo-go; `starter-kitex`, `starter-kratos`, `starter-go-zero`… wrap the rest | Integrates them all symmetrically; picks no winner |
| **Configuration** | Each ships its own config model | One layered engine (CLI → env → files → Nacos/etcd/Consul/Vault/K8s) driving every component, with `gs.Dync[T]` hot reload |
| **Governance** | Scoped to its own RPC calls (dubbo-go: URL-param overrides) | **One centralized governance center** — timeout/retry/breaker/rate-limit + fault injection applied uniformly across Redis, GORM, HTTP, gRPC, gin, dubbo… from one governance rules document, with pluggable rule sources (file/HTTP console/nacos/etcd direct listeners) |
| **Programming model** | Framework-defined interfaces and structure required | Zero intrusion: standard `net/http`, plain structs, your layout |
| **Use alone** | Yes | Yes — the core (`spring`) and ecosystem library (`cloud`) are usable without any RPC framework at all |

In short: **they answer "how do services call each other"; Go-Spring answers "how an application is assembled, configured, governed, and kept maintainable"**. A production service typically needs both — which is why Go-Spring integrates them rather than competing with them. (For the DI-framework axis — Wire/fx/dig — see the comparison in [spring/README.md](spring/README.md#11--comparison-with-other-frameworks).)

### Out-of-the-Box, Zero Intrusion

Every capability ships as a **Starter**. No inheritance, no adapters, no sprawling initialization boilerplate in `main.go`—`import` a starter and it automatically wires your components into the application lifecycle. The framework doesn't hijack `main()`, doesn't impose routing groups, doesn't mandate a directory layout. You write business logic your way; the framework handles assembly and lifecycle.

### Dependency Injection, the Go Way

No reflection magic, no `@Autowired` annotations, no XML configuration. Every dependency is declared explicitly through **constructor parameters** and wired by type automatically:

```go
gs.Provide(func(db *gorm.DB) *UserService {
    return &UserService{db: db}
})
```

Declare what you need, expose what you provide—nothing more.

### Unified Runtime Model: Runners & Servers

Go-Spring distills all service patterns into two abstractions:

- **Runner** — A one-shot execution unit (scheduled tasks, batch processing, startup-only logic). The container collects all Runners and executes them in configured order.
- **Server** — A long-lived service (HTTP, gRPC, Thrift, WebSocket…). The container handles `ListenAndServe` and graceful shutdown; `ReadySignal` notifies when the service is ready.

No manual signal handling, no goroutine lifecycle management—the framework has you covered.

### Built-in Enterprise Infrastructure

| Domain | Capability | Coverage |
|---|---|---|
| **Configuration** | Multi-source layered merging (CLI → env vars → config files → remote config centers), type-safe binding, dynamic refresh | Nacos, Consul, Etcd, K8s ConfigMap, Vault |
| **Service Governance** | Centralized governance center: timeout/retry/breaker/rate-limit/bulkhead + fault injection, one rules document for every client, hot-reloaded | default & sentinel backends; rule sources: file, HTTP console, nacos, etcd |
| **Logging** | Structured logging model, concise config DSL, pluggable Appenders | Console, File, custom |
| **Service Discovery** | Unified `Discovery` abstraction, multiple registry backends | Consul, Etcd, Nacos, Zookeeper, Polaris, K8s |
| **Distributed Coordination** | Distributed locks, messaging, transactions, events, scheduling, batch processing | Lock (4 backends), Kafka, Pulsar, RabbitMQ, NATS, MQTT, Saga, TCC, AT |
| **Observability** | Unified OpenTelemetry integration—one line of config enables full tracing and metrics | starter-otel + per-starter example-otel |
| **Security** | Access control, OAuth2, JWT, Session | Casbin (RBAC/ABAC/ACL), OAuth2 Client/Server, JWT, distributed Session |

### A Rich Starter Ecosystem — 70+ Modules

Each starter is an independent Go module. Pull in only what you need; the dependency graph stays clean:

- **Web Frameworks**: Gin, Echo, Hertz, go-zero, GoFrame, Kratos
- **RPC Frameworks**: gRPC, Kitex, Thrift, tRPC, Dubbo-go, go-zero/zrpc, GoFrame/gRPC, Kratos/gRPC
- **WebSocket**: Gorilla, Coder, GoFrame, Kratos
- **Databases**: MySQL, PostgreSQL, SQL Server, ClickHouse, MongoDB, Neo4j, Elasticsearch
- **Caching**: Redis (go-redis / redigo dual drivers), Memcached, BigCache
- **Message Queues**: Kafka (franz-go / Sarama dual drivers), Pulsar, RabbitMQ, NATS, MQTT
- **Config Centers**: File, Consul, Etcd, Nacos, K8s ConfigMap, Vault, Config Bus
- **Service Registries**: Consul, Etcd, Nacos, Zookeeper, K8s
- **Distributed Primitives**: Locks (Consul/Etcd/K8s/Redis), Transactions (Saga/TCC/AT), Scheduler, Batch, Goroutine Pool
- **Security**: Casbin, OAuth2 Client/Server, JWT, Session-Redis
- **Observability**: OpenTelemetry (Tracing + Metrics), pprof, Actuator, Admin UI
- **More**: Mail, Lua Filters, Swagger, Validation, Repository Pattern, Data Migration, API Gateway, Service Mesh, Resilience

> Full categorized list: [starter/README.md](starter/README.md).

### Powerful Toolchain

| Tool | Purpose |
|---|---|
| `gs` | One-stop CLI: create projects, add components, generate code, run services |
| `gs-http-gen` | Modern IDL syntax → HTTP server + declarative client code generation (nullable types, generics, embedding—OpenFeign-equivalent) |
| `gs-mock` | Type-safe Go mock library with native generics support and concurrency safety |

### Seamless Testing Integration

Deeply integrated with `go test`. Use `gs.RunTest()` to boot a real container in tests, with real dependencies—no need to mock everything. When you do need mocks, `gs-mock` provides type-safe method/function-level mocking that's goroutine-safe via context-based data isolation.

## Get Started in One Minute

```bash
# 1. Install the gs tool
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"

# 2. Create a project
gs init --module github.com/yourname/yourproject

# 3. Run it
go run main.go
```

## Documentation

| Document | Description |
|---|---|
| [Overview](website/en/docs/0.overview/) | Introduction, AI engineering philosophy, Claude Code best practices |
| [Getting Started](website/en/docs/1.getting-started/) | Project creation, development, running |
| [Guides](website/en/docs/2.guides/) | Configuration, IoC, lifecycle, logging, HTTP server, components, testing, http-gen |
| [Examples](website/en/docs/3.examples/) | Complete example index |
| [Integrations](website/en/docs/4.integrations/) | Detailed integration docs for each starter |
| [FAQ](website/en/docs/5.faq.md) | Frequently asked questions |
| [Contributing](website/en/docs/6.contributing.md) | How to contribute |
| [Changelog](website/en/docs/7.changelog.md) | Version history |

If you prefer learning through complete, progressive examples, see [go-spring-first](https://github.com/lvan100/go-spring-first), which provides 10 getting-started examples.

## Contributing

How to become a contributor? Submit meaningful PRs or feature requests, and have them accepted. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.


## Community

<table style="border: none;">
<tr style="border: none;">
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="*" height="180" alt="QQ Group QR"/></td>
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="*" height="180" alt="WeChat Official Account QR"/></td>
</tr>
<tr style="border: none;">
<td style="text-align: center; border:none;">QQ Group: 721077608</td>
<td style="text-align: center; border:none;">WeChat: GoSpring实战</td>
</tr>
</table>

## Donation

<img src="https://raw.githubusercontent.com/go-spring/go-spring/master/sponsor.png" width="140" />

To drive the continuous growth of Go-Spring, we warmly invite your support. Your donation will help us iterate faster, improve the ecosystem, and strengthen the community.

## Star History

<img src="https://api.star-history.com/svg?repos=go-spring/go-spring&type=Date" width="600" alt="Star History"/>

## License

Go-Spring is released under version 2.0 of the [Apache License](LICENSE).