# IDL 到 OpenAPI 文档生成设计

## 背景

`gs-http-gen` 当前以 IDL 项目为输入，主要生成 Go 语言服务端和客户端代码。IDL 中已经包含 HTTP 接口文档所需的核心信息，包括项目元信息、RPC 方法、请求路径、请求参数、响应类型、字段必填性、枚举、注释和自定义注解。

本次目标是在现有代码生成能力之外，新增从 IDL 项目生成接口文档的能力。第一阶段先支持 OpenAPI 3.0，保留 Swagger 2.0 的命令行入口设计。

## 命令行目标

新增两个独立开关：

```bash
gs-http-gen --openapi --output ./docs
gs-http-gen --swagger --output ./docs
```

第一阶段实现：

- `--openapi`：生成 OpenAPI 3.0 JSON 文档。
- `--swagger`：预留 Swagger 2.0 开关，第一阶段可以返回明确的未支持错误。

文档生成命令作为单独模式使用，不和代码生成命令混合：

- `--openapi` 和 `--swagger` 互斥。
- `--openapi` / `--swagger` 不与 `--server` / `--client` 混用。
- 开启文档生成时不执行 `--language` 对应的代码生成器。

输出文件名按常见约定：

- OpenAPI 3.0：`openapi.json`
- Swagger 2.0：`swagger.json`

`--output` 仍表示输出目录，默认当前目录。第一阶段只负责生成文档文件，不额外校验生成结果是否符合 OpenAPI 规范。

## 第一阶段范围

第一阶段只实现 OpenAPI 3.0 JSON 输出，不引入 YAML 输出，不转换 `validate` 表达式。

### OpenAPI 顶层结构

生成文档使用 OpenAPI 3.0.x：

```json
{
  "openapi": "3.0.3",
  "info": {},
  "paths": {},
  "components": {
    "schemas": {}
  }
}
```

`meta.json` 映射规则：

- `meta.name` -> `info.title`
- `meta.description` -> `info.description`
- `meta.version` -> `info.version`

第一阶段预留 OpenAPI 扩展配置读取能力，优先从 `meta.config.openapi` 读取。可预留字段包括：

- `servers`
- `tags`
- `externalDocs`
- `contact`
- `license`

首版可以先实现 `servers` 的透传；其他字段只在结构设计中预留，不要求全部落地。

### RPC 到 Path 映射

每个 `rpc` 或 `sse` 生成一个 path operation：

- `RPC.Path` -> `paths`
- `RPC.Method` -> HTTP method，小写作为 operation key
- `RPC.Name` -> `operationId`
- `summary` 注解 -> `summary`
- RPC 上方注释 -> `description`
- `RPC.Request` -> 请求参数和请求体来源
- `RPC.Response` -> `200` 响应 schema

SSE 接口第一阶段按普通接口生成 operation，并将响应 content type 设置为 `text/event-stream`。同时在 operation 上输出 vendor extension：

```json
{
  "x-sse": true
}
```

### 请求参数映射

请求类型中的字段按绑定来源拆分：

- `path="id"` -> `parameters[].in = "path"`
- `query="name"` -> `parameters[].in = "query"`
- 未绑定 `path/query` 的字段 -> `requestBody`

path 参数在 OpenAPI 中必须为 required，因此无论 IDL 字段是否显式 `required`，path 参数都输出 `required: true`。

query 参数根据 IDL 字段的 `required` 输出 `required`。

### Request Body 映射

当请求类型存在未绑定 `path/query` 的字段时生成 `requestBody`。

content type 映射：

- `application/json` -> `requestBody.content.application/json`
- `application/x-www-form-urlencoded` -> `requestBody.content.application/x-www-form-urlencoded`
- 其他 `contentType` 保留原值

如果请求类型同时包含 path/query 字段和 body 字段，body schema 只包含未绑定字段，避免把 path/query 参数重复放入请求体。

### Response 映射

每个 operation 默认生成 `200` 响应：

- response content type 默认使用 `application/json`
- SSE response content type 使用 `text/event-stream`
- response schema 使用 RPC 响应类型

第一阶段不根据错误码枚举自动生成非 200 响应。

### Schema 映射

IDL 类型到 OpenAPI schema 的基本映射：

| IDL 类型 | OpenAPI schema |
| --- | --- |
| `bool` | `{ "type": "boolean" }` |
| `int` / `uint` | `{ "type": "integer", "format": "int64" }` |
| `float` | `{ "type": "number", "format": "double" }` |
| `string` | `{ "type": "string" }` |
| `bytes` | `{ "type": "string", "format": "byte" }` |
| `list<T>` | `{ "type": "array", "items": T }` |
| `map<string,T>` | `{ "type": "object", "additionalProperties": T }` |
| enum | integer enum，`enum_as_string` 字段输出 string enum |
| user type | `$ref` 到 `#/components/schemas/<TypeName>` |
| oneof | 使用 `oneOf` 引用候选类型 |

结构体字段映射：

- 字段名优先使用 `json` 注解名。
- form body 中字段名优先使用 `form` 注解名。
- 未配置时使用解析后的默认 JSON/Form 名称。
- `required` 字段加入 schema 的 `required` 数组。
- `deprecated` 字段输出 `deprecated: true`。
- 字段注释输出到 `description`。

泛型实例化类型使用解析后的实际字段生成 schema。泛型模板本身不单独输出。

### 注释与描述

第一阶段描述来源优先级：

- RPC summary：优先使用 `summary` 注解。
- RPC description：使用 RPC 上方注释。
- type description：使用 type 上方注释。
- field description：使用 field 上方或右侧注释。
- enum field description：使用枚举字段注释或 `errmsg` 注解。

## 暂不实现

第一阶段暂不处理以下内容：

- Swagger 2.0 实际文档生成。
- YAML 输出。
- `validate` 表达式到 `minimum`、`maximum`、`minLength`、`maxLength`、`pattern` 等 OpenAPI 约束的转换。
- 根据错误码枚举自动生成非 200 response。
- 认证、鉴权、安全方案。
- 复杂 vendor extension 规范。

## 建议实现路径

1. 在 `main.go` 中增加 `--openapi` 和 `--swagger` 开关，并校验它们与 `--server` / `--client` 的互斥关系。
2. 新增独立文档生成入口，例如 `gen/docgen`，避免把文档生成伪装成某个 `--language` 代码生成器。
3. 新增 `gen/docgen/openapi` 包，直接基于 `httpidl.ParseDir` 生成 OpenAPI 文档。
4. 实现 OpenAPI 内部结构体，并使用 `encoding/json` 输出带缩进的 `openapi.json`。
5. 为示例 IDL 增加结构化断言测试，验证 paths、parameters、requestBody、responses、components.schemas 的关键内容。测试只验证生成器行为，不引入 OpenAPI 规范校验器。

## 待确认问题

- `meta.config.openapi.servers` 的具体 JSON 结构是否完全沿用 OpenAPI 原生 `servers` 对象。
- `contact`、`license`、`tags` 等信息是否放在 `meta.config.openapi` 下，还是提升为 `meta.json` 顶层字段。
