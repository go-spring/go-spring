# starter-nacos

[English](README.md) | [中文](README_CN.md)

`starter-nacos` provides a Nacos client wrapper based on github.com/nacos-group/nacos-sdk-go/v2,
exposing both a naming (service discovery) client and a config (configuration management) client
for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-nacos
```

## Quick Start

### 1. Import the `starter-nacos` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-nacos"
```

### 2. Configure the Nacos Instance

Add Nacos configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.nacos.ip-addr=127.0.0.1
spring.nacos.port=8848
```

### 3. Inject the Nacos Clients

Refer to the [example.go](example/example.go) file. Both a naming client and a
config client are provided; inject whichever you need by its interface type.

```go
import (
    "github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
    "github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
)

type Service struct {
    Config config_client.IConfigClient `autowire:"__default__"`
    Naming naming_client.INamingClient `autowire:"__default__"`
}
```

### 4. Use the Nacos Clients

Refer to the [example.go](example/example.go) file.

```go
_, err := s.Config.PublishConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP", Content: "value"})
content, err := s.Config.GetConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP"})
```

## Advanced Features

* **Naming + config clients**: Both `INamingClient` and `IConfigClient` are registered as
  `__default__` beans (distinct interface types).
* **Supports multiple naming instances**: You can define multiple naming clients under
  `spring.nacos.instances` in the configuration file and reference them by name.
