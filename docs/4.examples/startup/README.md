# Startup

[中文](README_CN.md)

This project, though small in size, showcases the essential capabilities of **Go-Spring**.

## Features

### 1. Layered Configuration

Configurations can be loaded from various sources including:

- `sysconf`
- Local files
- Remote files
- Environment variables
- Command-line arguments

### 2. Property Binding

Configurations from files, command-line arguments, or environment variables can be bound directly to struct fields.

```go
type Service struct {
    StartTime time.Time `value:"${start-time}"`
}
```

### 3. Dependency Injection

Go-Spring automatically organizes dependencies between beans and injects them at runtime.

```go
gs.Provide(func (s *Service) *http.ServeMux {
    http.HandleFunc("/echo", s.Echo)
    http.HandleFunc("/refresh", s.Refresh)
    return http.DefaultServeMux
})
```

### 4. Dynamic Configuration Refresh

Configurations can be refreshed at runtime without restarting the application, enabling real-time updates.

```go
type Service struct {
    RefreshTime gs.Dync[time.Time] `value:"${refresh-time}"`
}
```

### 5. Route Registration

Supports HTTP server setup with customizable routing.

```go
gs.Provide(func (s *Service) *http.ServeMux {
    http.HandleFunc("/echo", s.Echo)
    http.HandleFunc("/refresh", s.Refresh)
    return http.DefaultServeMux
})
```
