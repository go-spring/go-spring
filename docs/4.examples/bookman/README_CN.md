# BookMan

[English](README.md)

## 一、目录结构

```text
conf/            配置文件目录
log/             日志文件目录
public/          静态文件目录
src/             源文件目录
  app/           启动阶段文件
    bootstrap/   引导阶段文件
    common/      启动阶段公共模块
      handlers/  启动阶段组件
        log/     日志组件
      httpsvr/   HTTP 服务模块
    controller/  控制器模块
  biz/           业务逻辑模块
    job/         后台任务模块
    service/     业务服务模块
  dao/           数据访问层
  idl/           接口描述文件
    http/        HTTP 服务接口
      proto/     生成的协议代码
  sdk/           封装的 SDK 文件
```

**目录结构特点**：

- **模块化设计**，清晰划分各层职责。
- **经典结构**，便于开发、管理和扩展。
- **易于维护**，支持大规模应用持续迭代。

## 二、功能描述

### 2.1 引导阶段配置管理

- 从远程拉取配置文件并保存至本地。
- 向启动阶段注册配置刷新的 Bean。
- 相关文件：`src/app/bootstrap/bootstrap.go`

### 2.2 日志组件初始化

- 启动阶段读取并解析本地配置。
- 根据配置创建日志组件。
- 相关文件：`src/app/common/handlers/log/log.go`

### 2.3 HTTP 服务器启动

- 启动阶段创建 HTTP 服务器。
- 注册 HTTP 服务路由。
- 相关文件：`src/app/common/httpsvr/httpsvr.go`

### 2.4 控制器功能分组管理

- 根据功能对 Controller 方法进行分组。
- 每个子 Controller 独立注入和管理。
- 相关文件：
    - `src/app/controller/controller.go`
    - `src/app/controller/controller-book.go`

### 2.5 动态配置刷新

- 支持运行时动态刷新配置。
- 相关文件：`src/biz/service/book_service/book_service.go`

### 2.6 后台任务优雅退出

- 后台任务支持优雅停止，保证数据安全和资源释放。
- 相关文件：`src/biz/job/job.go`

## 三、总结

本项目遵循模块化、清晰、可维护、可扩展的设计原则，适合中大型系统的开发需求。通过引导配置、日志管理、HTTP
服务、动态刷新、后台任务管理等功能模块，实现了完整、健壮的应用架构。