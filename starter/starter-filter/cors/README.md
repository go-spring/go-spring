# starter-filter/cors

## Installation

### Prerequisites

- Go >= 1.12

### Using go get

```
go get github.com/go-spring/starter-filter@v1.1.0-rc4
```

## Quick Start

```
import "github.com/go-spring/starter-filter/cors"
```

`configure`

```
cors.allow-origins=*
cors.allow-methods=GET,POST,PUT,UPDATE
cors.allow-headers=Origin,Content-Type,Accept,Authorization
cors.expose-headers=Content-Length,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type
cors.max-age=86400s
```

## Customization

