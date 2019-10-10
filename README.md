# Go-Spring

### [小喇叭]招募开发者!

由于 Go-Spring 是由我个人发起的项目，还不是团队作战，而且我在公司的主业务也很忙，所以基本上处于我有时间了就会更新的情况，但长此下去对 Go-Spring 的发展极为不利，也满足不了大家对 Go-Spring 的期待，所以欢迎对 Go-Spring 感兴趣的开发者参与到 Go-Spring 的日常开发和运营中来！

### Go-Spring 核心特性

我个人认为目前 Go-Spring 实现了两个非常核心的特性：IoC 容器和开箱即用。

1. IoC 容器可以满足对依赖注入、属性绑定、对象初始化的需求；
2. 开箱即用可以满足对自动配置、依赖管理、第三方类库集成的需求。

有了这两大基本功能，GoLang 开发基本上算是摆脱了茹毛饮血的初级阶段。随着项目的不断完善，后面 GoLang 开发肯定会进入更高级的层次。

### Go-Spring 编程思想

Go-Spring 主推两种编程思想：面向接口编程和面向模块编程。

1. Go-Spring 为常见的业务领域提供了一个抽象层，通过抽象层可以屏蔽底层的实现细节，可以灵活的切换底层方案。
2. Go-Spring 将不同的业务领域封装成模块，在内部注册模块所需的对象，通过 Starter 机制实现开箱即用的能力。

### Go-Spring 项目仓库

Go-Spring 有两个仓库，一个是实现 IoC 容器特性的主项目所在的 didi 仓库，另一个是实现开箱即用特性的 Starters 项目所在的 [go-spring](https://github.com/go-spring)仓库。

### Go-Spring 1.0 目标

SpringCore: 实现完善的 IoC 容器功能，支持数组对象注入，支持更多类型的属性绑定，可能会支持 Bean 设置顺序；

&nbsp;&nbsp;&nbsp;&nbsp; TODO: 属性绑定支持结构嵌套，功能和设计见下面的详述。

SpringWeb: 为 Echo、Gin 等社区流行的 Go Web 服务实现一个抽象层，目前 SpringWeb 和 SpringRPC 的功能重合，未来 SpringWeb 的接口会重构，更贴近现有的 Web 服务的接口；

SpringRPC: 为 Http、Thrift、gRPC、Dubbo 等社区流行的 RPC 服务实现一个抽象层，现在项目中有一个示例，可以证明通过 SpringRpcContext 有机会将上层接口统一；

SpringLog: 为一般日志场景和微服务所需的 Trace 场景提供日志支持，目前规划实现一个 TraceContext 结构，内部包含一个 context.Context 对象，一个 Trace 对象，并且实现 Log 接口，还提供一个 CopyTrace 方法。

脚手架: 创建 Go-Spring 推荐的项目结构，仓库地址 https://github.com/go-spring/create-go-spring。

TODO

### Go-Spring 开发环境

当前请使用 Go1.12 版本进行开发。Go 保持半年一个版本的发布节奏，所以 Go 版本会定期的保持升级，但是一般会低 1~2 个版本。

### Go-Spring 代码风格

代码必须使用 `goimports` 进行格式化，格式化的命令是 `goimports -w -format-only *`。如果你使用的是 IDEA 开发工具，请按照以下步骤进行配置：

1. Editor -> Code Style -> Go，选中 Imports 标签；
2. Sorting type 选择 `goimports`；
3. 选中 `Add parentheses for a single import`；
4. 选中 `Group stdlib imports`；
5. 选中 `Move all stdlib imports in a single group`；
6. 选中 `Move all imports in a single declaration`；

****

### TODO 详述

#### 属性绑定的嵌套功能说明

现在的样子:

```
type StreamServerConfig struct {
	// RTMP 内网配置
	RtmpInnerSecurePort   string `value:"${rtmp.inner.secure.port}"`
	RtmpInnerInsecurePort string `value:"${rtmp.inner.insecure.port}"`

	// RTMP 外网配置
	RtmpPublicSecureHost   string `value:"${rtmp.public.secure.host}"`
	RtmpPublicInsecureHost string `value:"${rtmp.public.insecure.host}"`

	// HTTP 内网配置
	HttpInnerSecurePort   string `value:"${http.inner.secure.port}"`
	HttpInnerInsecurePort string `value:"${http.inner.insecure.port}"`

	// HTTP 外网配置
	HttpPublicSecureHost   string `value:"${http.public.secure.host}"`
	HttpPublicInsecureHost string `value:"${http.public.insecure.host}"`
}
```

改造后的样子：

```
type StreamServer struct {

    SecurePort      string  `value:"${secure.port}"` 
    SecureHost      string  `value:"${secure.host}"` 

    InsecurePort    string  `value:"${insecure.port}"`
    InsecureHost    string  `value:"${insecure.host}"`
}

type StreamServerConfig struct {

    RtmpInner      *StreamServer     `value:"${rtmp.inner}"`
    RtmpPublic     *StreamServer     `value:"${rtmp.public}"`

    HttpInner      *StreamServer     `value:"${http.inner}"`
    HttpPublic     *StreamServer     `value:"${http.public}"`
}

```

****

### Go-Spring 项目简介

Go-Spring 是模仿 Java 的 Spring 全家桶实现的一套 GoLang 的应用程序框架，遵循“习惯优于配置”的原则，提供了依赖注入、自动配置、开箱即用、丰富的第三方类库集成等功能，能够让程序员少写很多的样板代码。

完整的 go-spring 项目一共包含 6 个模块，当前模块仅实现了基础的 IOC 容器的能力，该模块可以独立使用，但是配合其他模块才能使得效率最大化。其他模块发布在 https://github.com/go-spring 仓库下。下面是所有模块的列表：

1、程序启动框架  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;AppRunner  
2、核心功能模块  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;GoSpring  
3、启动器核心组件  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;GoSpringBoot  
4、开源微服务组件  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;GoSpringCloud  
5、多个项目启动器  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;GoSpringBootStarter  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;GoSpringCloudStarter  

### 项目特点

1. 面向接口编程
2. 面向模块化编程
3. 简单的启动器框架
4. 依赖注入、属性注入
5. 项目依赖管理
6. 简化的 http test 框架
7. 支持多种配置文件格式
8. 支持多环境配置文件
9. 统一的代码风格
10. 自动加载配置、模块
11. 丰富的示例，极易上手

### 代码规范

1. 一个单词的包名采用小写格式（Maybe）
2. 多个单词的包名使用首字母大写的格式
3. HTTP 接口强制使用 POST 方法
4. 业务代码允许 panic 中断请求
5. 返回值包含详细的错误信息 …

### 实现原理

1. AppRunner
2. SpringContext
3. Bean 管理
4. Bean 注入，autowire
5. 属性注入，value
6. SpringBootApplication，适配 AppRunner
7. 启动器框架，Starters
8. 常用模块简介，Web、Redis、Mysql 等
9. Spring-Message 框架
10. Spring-Check + RPC框架

### 未来规划

1. 继承 Java Spring 全家桶的设计原则，但不照搬照抄，适应 Go 语言
2. 形成滴滴的 Go 项目和代码规范
3. 完整支持微服务框架，监控、日志跟踪等
4. 和 dubbo 协议、框架打通
5. 创建新项目的工具软件
6. 探索无服务器架构支持
7. 管理端点 endpoint
8. 更丰富的 debug 信息输出
9. 支持用户配置覆盖模块默认配置
10. 支持禁用特定的自动配置
11. 定制 banner
12. 属性支持占位符，松散绑定等高级特性 …

### 1.0 版本目标

TODO

### 示例

https://github.com/go-spring

### 相关文档

TODO

### 项目成员

#### 发起者/负责人

[lvan100 (LiangHuan)](https://github.com/lvan100)

如何成为外部贡献者？ 提交有意义的PR，并被采纳。

### QQ 交流群

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq.png" width="140" height="*" />

### Note

This is not an official Didi product (experimental or otherwise), it is just code that happens to be owned by Didi.