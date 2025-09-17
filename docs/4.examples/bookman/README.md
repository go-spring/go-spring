# BookMan

[中文](README_CN.md)

## 1. Directory Structure

```text
conf/           Configuration files
log/            Log files
public/         Static files
src/            Source code
  app/          Startup phase files
    bootstrap/  Bootstrap files
    common/     Common modules for startup
      handlers/ Startup component handlers
        log/    Logging component
      httpsvr/  HTTP server module
    controller/ Controller modules
  biz/          Business logic modules
    job/        Background job modules
    service/    Business service modules
  dao/          Data access layer
  idl/          Interface definition files
    http/       HTTP service interfaces
      proto/    Generated protocol code
  sdk/          Wrapped SDK modules
```

**Directory Structure Features**:

- **Modular design** with clear separation of responsibilities.
- **Classic structure** for easy development, management, and scalability.
- **Maintainability** supporting continuous iteration for large-scale applications.

## 2. Functionality Overview

### 2.1 Bootstrap Phase Configuration Management

- Fetch configuration files remotely and save them locally.
- Register configuration refresh beans during the startup phase.
- Related file: `src/app/bootstrap/bootstrap.go`

### 2.2 Logging Component Initialization

- Load and parse local configuration files during the startup phase.
- Create logging components based on the configuration.
- Related file: `src/app/common/handlers/log/log.go`

### 2.3 HTTP Server Initialization

- Create an HTTP server during the startup phase.
- Register HTTP service routes.
- Related file: `src/app/common/httpsvr/httpsvr.go`

### 2.4 Controller Grouping and Management

- Group controller methods based on functionality.
- Independently inject and manage each sub-controller.
- Related files:
    - `src/app/controller/controller.go`
    - `src/app/controller/controller-book.go`

### 2.5 Dynamic Configuration Refresh

- Support dynamic configuration refresh at runtime.
- Related file: `src/biz/service/book_service/book_service.go`

### 2.6 Graceful Shutdown of Background Jobs

- Ensure background tasks shut down gracefully, preserving data integrity and releasing resources properly.
- Related file: `src/biz/job/job.go`

## 3. Summary

This project follows modular, clear, maintainable, and extensible design principles, making it suitable for the
development needs of medium to large-scale systems. It implements a complete and robust architecture with modules for
bootstrapping, logging management, HTTP services, dynamic configuration refreshing, and background job handling.