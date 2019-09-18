# Go-Spring

是模仿 Java 的 Spring 全家桶实现的一套 GoLang 的应用程序框架，仍然遵循“习惯优于配置”的原则，提供了依赖注入、自动配置、开箱即用、丰富的第三方类库集成等功能，能够让程序员少写很多的样板代码。

### 模块介绍

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