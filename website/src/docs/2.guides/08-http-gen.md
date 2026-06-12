# http-gen

http-gen 是一个基于 IDL 定义生成 Go HTTP 服务端和客户端代码的工具。

## 项目结构与依赖

典型的 http-gen 项目结构：
```
project/
├── meta.json        # 元信息（包名、输出路径等）
├── api/             # IDL 文件目录
│   ├── user.idl
│   └── order.idl
└── ...
```

- 支持 `import` 机制引入外部 IDL 文件
- 多个 IDL 共享同一个命名空间

## IDL 语法与语义

### 注释

```idl
// 单行注释

/*
   多行注释
*/
```

### 关键字

| 关键字 | 说明 |
|--------|------|
| `extends` | 扩展枚举 |
| `const` | 常量定义 |
| `enum` | 枚举定义 |
| `type` | 类型定义 |
| `oneof` | 联合类型 |
| `rpc` | RPC 方法定义 |
| `sse` | 流式响应（Server-Sent Events） |
| `true`/`false` | 布尔值 |
| `optional` | 可选字段 |
| `required` | 必填字段 |

### 基础类型

| 类型 | 说明 |
|------|------|
| `bool` | 布尔 |
| `int` | 有符号整数（不内置 uint，可自定义） |
| `float` | 浮点数 |
| `string` | 字符串 |
| `bytes` | 字节数组 |
| `list` | 列表切片 |
| `map` | 映射字典 |

支持多层嵌套。

### 常量

```idl
const MAX_USERS = 1000;
```

### 注解

```idl
@route("/api/users")      // 单行注解
@method("POST")
```

语法：`@key(=value)?`，value 省略表示 `true` 布尔语义。

### 枚举

```idl
enum ErrorCode {
    OK = 0 @errmsg("success")
    NOT_FOUND = 1 @errmsg("not found")
}
```

- 支持 `errmsg` 注解存储错误信息
- 支持 `enum_as_string` 选项生成字符串枚举
- 支持 `extends` 扩展合并其他枚举

### 类型（结构体）

```idl
type User {
    required int64 id;
    required string username;
    optional string avatar;
    @pattern("^[a-z]+$") string name;
}
```

- 支持普通结构体、泛型结构体
- 支持嵌套字段、嵌入类型（字段合并）
- 支持联合类型 `oneof`（枚举 + 结构体）
- 支持字段校验：
  - 内置校验函数
  - 自定义校验函数
  - 运算符表达式，`$` 表示当前字段值

## 接口定义

```idl
rpc CreateUser(CreateUserRequest) returns (CreateUserResponse) @method("POST") @path("/api/users");
```

- 支持 `rpc` 普通方法
- 支持 `sse` 流式响应方法
- 接口注解支持 `method`、`path`、`contentType`、`timeout` 等
- Restful 路径参数支持 `:name` 或 `{name}` 语法，仅支持 `int`/`string` 类型

## 代码生成器

- 生成 Go 服务端骨架代码
- 生成 Go 客户端调用代码
- 流式解析优化：支持 `required` 校验，流式 JSON/form 解析，性能更好

使用方式：
```bash
go run github.com/go-spring/gs-http-gen/cmd/gs-http-gen --config meta.json
```
