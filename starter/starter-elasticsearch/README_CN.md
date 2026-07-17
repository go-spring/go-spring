# starter-elasticsearch

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-elasticsearch` 提供了基于官方 [go-elasticsearch](https://github.com/elastic/go-elasticsearch)
客户端的封装，方便在 Go-Spring 服务中快速集成和使用 Elasticsearch。

## 安装

```bash
go get go-spring.org/starter-elasticsearch
```

## 快速开始

### 1. 引入 `starter-elasticsearch` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-elasticsearch"
```

### 2. 配置 Elasticsearch 实例

在项目的[配置文件](example/conf/app.properties)中添加 Elasticsearch 配置，比如：

```properties
spring.elasticsearch.docs.addresses=http://127.0.0.1:9200
```

### 3. 注入 Elasticsearch 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/elastic/go-elasticsearch/v8"

type Service struct {
    ES *elasticsearch.Client `autowire:"docs"`
}
```

### 4. 使用 Elasticsearch 实例

参见 [example.go](example/example.go) 文件。

```go
res, err := s.ES.Index("index", strings.NewReader(`{"title":"hello"}`), s.ES.Index.WithDocumentID("1"))
res, err := s.ES.Get("index", "1")
res, err := s.ES.Search(s.ES.Search.WithIndex("index"), s.ES.Search.WithBody(query))
```

## 核心功能

[example.go](example/example.go) 文件演示了以下核心 Elasticsearch 功能：

* **集群信息**：使用 `Info` 验证与集群的连通性。
* **写入文档**：使用 `Index` 存储 JSON 文档，并通过 `WithRefresh` 让其立即可被检索。
* **读取文档**：使用 `Get` 按 ID 读取文档。
* **检索文档**：使用 `Search` 配合 `match` 查询检索文档。

## 高级功能

* **支持多 Elasticsearch 实例**：可以在配置文件中定义多个 Elasticsearch 实例，并在项目中使用 name 进行引用。
* **支持 Elasticsearch 扩展**：可以通过实现 `Driver` 接口来扩展 Elasticsearch 功能，参见示例中的 `AnotherESDriver` 实现。
