# Startup

[English](README.md)

本项目虽小，但展示了 **Go-Spring** 最核心的功能。

## 功能介绍

### 1. 分层配置

支持从多种来源加载配置，包括：

- `sysconf`
- 本地文件
- 远程文件
- 环境变量
- 命令行参数

### 2. 属性绑定

可以将文件、命令行参数或环境变量中的配置，直接绑定到结构体字段。

```go
type Service struct {
    StartTime time.Time `value:"${start-time}"`
}
```

### 3. 依赖注入

Go-Spring 能自动组织 Bean 之间的依赖关系，在运行时完成注入。

```go
gs.Provide(func (s *Service) *http.ServeMux {
    http.HandleFunc("/echo", s.Echo)
    http.HandleFunc("/refresh", s.Refresh)
    return http.DefaultServeMux
})
```

### 4. 配置刷新

支持配置在运行时动态刷新，无需重启应用，即可应用最新配置。

```go
type Service struct {
    RefreshTime gs.Dync[time.Time] `value:"${refresh-time}"`
}
```

### 5. 路由注册

支持 HTTP 服务器，且可以灵活定制所需路由。

```go
gs.Provide(func (s *Service) *http.ServeMux {
    http.HandleFunc("/echo", s.Echo)
    http.HandleFunc("/refresh", s.Refresh)
    return http.DefaultServeMux
})
```
