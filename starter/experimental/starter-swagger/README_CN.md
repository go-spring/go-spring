# starter-swagger

[English](README.md) | [中文](README_CN.md)

`starter-swagger` 提供交互式 [Swagger UI](https://github.com/swagger-api/swagger-ui)
页面,并托管由 `gs-http-gen --openapi` 生成的 OpenAPI 文档,让 Go-Spring 服务无需
内置静态资源、也无需额外监听端口即可暴露可浏览的 API 文档。

## 安装

```bash
go get go-spring.org/starter-swagger
```

## 快速开始

### 1. 生成 OpenAPI 文档

用 HTTP 代码生成器从 IDL 生成 `openapi.json`:

```bash
gs-http-gen --openapi -i api.thrift -o .
```

### 2. 导入 `starter-swagger` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-swagger"
```

### 3. 配置 UI

在项目的[配置文件](example/conf/app.properties)中添加 Swagger 配置:

```properties
spring.swagger.specFile=openapi.json
spring.swagger.basePath=/swagger
spring.swagger.title=My API Docs
```

### 4. 访问文档

该 starter 以 `endpoint.Endpoint` 形式贡献 UI。若同时启用了 `starter-actuator`,
页面会自动挂载到管理端口,无需接线;否则注入 `*StarterSwagger.UI` bean 并挂载到你
自己的 HTTP 服务器上:

```go
gs.Provide(func(ui *StarterSwagger.UI) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle(ui.Path(), ui)
    return &gs.HttpServeMux{Handler: mux}
})
```

然后在浏览器打开 `http://<host>/swagger/`。

## 核心功能

[example.go](example/example.go) 程序演示并断言三件事:

* **UI 外壳** —— `GET /swagger/` 返回启动 Swagger UI 的 HTML 页面。
* **index 别名** —— `GET /swagger/index.html` 返回同一外壳。
* **文档透传** —— `GET /swagger/openapi.json` 原样返回生成的 OpenAPI 文档。

## 设计说明

* **不内置静态资源**:CSS/JS 包在运行时从 CDN 加载
  (`spring.swagger.assetBaseURL`,默认 `unpkg.com/swagger-ui-dist@5`),因此
  starter 只携带一小段 HTML 外壳。内网隔离环境可将其指向自建镜像。
* **快速失败**:OpenAPI 文档在启动时读取一次;文件缺失或不可读会立即暴露,而不是等
  应用上线后才 404。
* **不占用自有端口**:UI 以 bean/`endpoint.Endpoint` 形式贡献,挂载到应用已经运行的
  服务器上(actuator 管理端口或应用自身的 HTTP 服务器)。
* **无需改代码即可开关**:设置 `spring.swagger.enabled=false` 可在生产环境禁用文档,
  同时保留 import。
