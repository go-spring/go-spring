<div>
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" height="*" alt="logo"/>
</div>
<br/>

> Go-Spring 是一个由众多子项目组成的大型生态。建议大家为这个总览仓库点亮 ⭐，这样能够更直观地展示 Go-Spring 的整体价值。

> Go-Spring is a large ecosystem with many sub-projects. We recommend starring this overview repository so that the full
> value of Go-Spring can be more clearly recognized.

Go-Spring 是对传统 Go 项目开发痛点的一次有力回应。它借鉴了 Java 社区 Spring / Spring Boot
的成功经验，以依赖注入和自动配置为核心基础，把“开箱即用”设为首要目标。同时，它坚持 Go
社区简洁高效的哲学，融合了代码生成等优秀理念，在保持轻量与灵活的前提下，带来全新的开发体验。Go-Spring 不仅降低了项目搭建的复杂度，更为
Go 应用开发提供了里程碑式的突破。

Go-Spring is a bold response to the challenges of traditional Go project development. Inspired by the success of Spring
and Spring Boot in the Java community, it builds on dependency injection and auto-configuration as its foundation, with
“out-of-the-box” usability as a top priority. At the same time, it stays true to Go’s philosophy of simplicity and
efficiency, incorporating ideas like code generation to deliver a fresh development experience. Go-Spring not only
reduces project setup complexity but also represents a milestone breakthrough for Go application development.

## 特性

1. **开箱即用 & 无侵入设计** (**Out-of-the-box & Non-intrusive Design**)  
   提供即插即用的能力，不强制框架结构，让开发者专注于业务逻辑。  
   Works immediately without enforcing rigid framework structures, letting developers focus on business logic.


2. **依赖注入与自动装配** (**Dependency Injection & Auto-configuration**)  
   借鉴 Spring 的 Starter 机制，实现灵活的依赖管理与自动配置，支撑开箱即用体验。  
   Inspired by Spring’s Starter mechanism, it enables flexible dependency management and automatic setup to support an
   out-of-the-box experience.


3. **统一的基础设施框架** (**Unified Infrastructure Frameworks**)  
   提供可扩展的配置系统与日志系统，为依赖注入和自动装配打下坚实基础。  
   Provides extensible configuration and logging systems that serve as the foundation for DI and auto-configuration.


4. **模块化项目脚手架** (**Modular Project Scaffolding**)  
   基于 **modulith** 模块化理念，快速生成项目结构，提升工程组织性与可维护性。  
   Generates project structures based on the **modulith** modularization concept, improving organization and
   maintainability.


5. **IDLs-First 设计理念** (**IDLs-First Philosophy**)  
   采用现代化 IDL 语法，支持可空、嵌入、模板等特性，推动契约驱动的开发模式。  
   Adopts modern IDL syntax with support for nullable types, embedding, templates, and more—promoting contract-first
   development.


6. **多协议代码生成** (**Multi-protocol Code Generation**)  
   内置代码生成工具，支持 HTTP、gRPC、Thrift 等多种协议，减少重复工作。  
   Built-in code generators support HTTP, gRPC, Thrift, and other protocols, reducing repetitive work.


7. **抽象化运行模型** (**Abstracted Runtime Models**)  
   通过 **Runner、Job、Server** 三种核心模型，统一抽象多种服务形态，简化扩展与集成。  
   Introduces three unified models—**Runner, Job, Server**—to simplify integration and support multiple service types.


8. **丰富的组件生态** (**Rich Component Ecosystem**)  
   提供 MySQL、Redis 等常用中间件的 Starter，真正做到即插即用。  
   Provides ready-to-use Starters for common middleware like MySQL and Redis.


9. **无缝测试集成** (**Seamless Testing Integration**)  
   与 `go test` 深度集成，提供简洁高效的单元测试支持。  
   Deeply integrates with `go test` to deliver simple yet powerful unit testing capabilities.

## 模块 (Modules)

| 模块名<br>Module Name                                                    | 描述<br>Description                                                        |
|-----------------------------------------------------------------------|--------------------------------------------------------------------------|
| [go-spring :: log](https://github.com/go-spring/log)                  | 前后端统一的日志库<br>Unified front-end and back-end log                          |
| [spring-base :: assert](https://github.com/go-spring/spring-base)     | 用于 Go 单测的断言库<br>An assertion library for Go unit tests                   |
| [spring-core](https://github.com/go-spring/spring-core)               | 核心项目<br>Core project                                                     |
| [gs-mock](https://github.com/go-spring/gs-mock)                       | 现代化的、类型安全的 Go 语言 mocking 库<br>A modern, type-safe mocking library for Go |
| [starter-gorm-mysql](https://github.com/go-spring/starter-gorm-mysql) | gorm mysql 启动器<br>Starter for gorm with mysql                            |
| [starter-redigo](https://github.com/go-spring/starter-redigo)         | redigo 启动器<br>Starter for redigo                                         |
| [starter-go-redis](https://github.com/go-spring/starter-go-redis)     | go-redis 启动器<br>Starter for go-redis                                     |
| [gs-http-gen](https://github.com/go-spring/gs-http-gen)               | 基于 IDL 的 HTTP 代码生成工具<br>HTTP code generation tool based on IDL files     |
| [gs](https://github.com/go-spring/gs)                                 | Go-Spring 工具管理器<br>Go-Spring Tools Manager                               |
| [gs-init](https://github.com/go-spring/gs-init)                       | 创建新项目的工具<br>Create new projects                                          |
| [gs-add](https://github.com/go-spring/gs-add)                         | 为项目添加新组件的工具<br>Add new components                                        |
| [gs-gen](https://github.com/go-spring/gs-gen)                         | 根据 IDL 文件生成 Go 服务端代码<br>Generate go server code based on IDL files       |
| [skeleton](https://github.com/go-spring/skeleton)                     | 实践 modulith 的项目骨架<br>Modulith practice project skeleton                  |

## 开箱 (Getting Started)

1. 安装 [gs](https://github.com/go-spring/gs) 工具。Install the [gs](https://github.com/go-spring/gs) tool.

```shell
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"
```

2. 创建项目。Create a project.

```shell
gs init --module my-git/my-group/my-module
```

3. 运行程序。 Run the program.

```shell
go run main.go
```

> 你可以找到更多的[文档](docs)和[示例](docs/4.examples)。  
> Find more [docs](docs) and [examples](docs/4.examples).

## 贡献 (Contribution)

如何成为贡献者？提交有意义的 PR 或者需求，并被采纳。  
How to become a contributor? Submit meaningful PRs or feature requests, and have them accepted.

## 交流 (Communication)

<table style="border: none;">
<tr style="border: none;">
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="*" height="180"  alt=""/></td>
<td style="text-align: center; border:none;"><img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="*" height="180"  alt=""/></td>
</tr> 
<tr style="border: none;">
<td style="text-align: center; border:none;">QQ群号: 721077608</td>
<td style="text-align: center; border:none;">公众号: GoSpring实战</td>
</tr>
</table>

## 捐赠 (Donation)

<img src="https://raw.githubusercontent.com/go-spring/go-spring/master/sponsor.png" width="140" height="*" />

为了推动 Go-Spring 的持续发展，我们诚挚邀请您支持本项目。您的捐赠将帮助我们更快地迭代功能、完善生态，并壮大社区力量。

To drive the continuous growth of Go-Spring, we warmly invite your support. Your donation will help us iterate faster,
improve the ecosystem, and strengthen the community.

## Star History

<img src="https://api.star-history.com/svg?repos=go-spring/go-spring&type=Date" width="600" alt=""/>

## 鸣谢 (Thanks)

Thanks to JetBrains' IntelliJ IDEA product for providing a convenient and efficient code editing and testing
environment.

## 许可证 (License)

The Go-Spring is released under version 2.0 of the Apache License.
