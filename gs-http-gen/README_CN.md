# gs-http-gen

<div>
   <img src="https://img.shields.io/github/license/go-spring/gs-http-gen" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/gs-http-gen" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/gs-http-gen?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/gs-http-gen" > 
      <img src="https://codecov.io/gh/go-spring/gs-http-gen/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://deepwiki.com/go-spring/gs-http-gen"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> 本项目处于持续迭代阶段，功能和特性将不断完善。

`gs-http-gen` 是一款 **面向 HTTP/RPC 场景的接口定义语言（IDL）代码生成工具**，
可根据统一的接口描述自动生成 **Go 语言** 服务端与 **其他语言** 客户端代码，服务端代码包括：

* 数据模型
* 验证逻辑
* HTTP 路由绑定
* 普通与流式（SSE）接口

通过声明式的 IDL 描述，开发者可以更专注于业务逻辑，显著减少样板代码编写和手动出错的风险。

此外，IDL 还可以作为 **跨团队、前后端统一的接口契约与文档**，帮助开发团队减少沟通成本，提升协作效率。

## 功能特性

### 🌟 IDL 驱动

* 使用简洁的接口定义语言描述服务接口与数据模型
* 支持常量、枚举、结构体、`oneof` 类型、泛型和字段修饰符
* 支持 RPC 与 SSE 流式接口定义
* 支持自定义注解（如 `json`、`go.type`、`enum_as_string`、`validate` 等）

### ⚙️ 自动代码生成

根据 IDL 文件自动生成 Go 语言服务端及其他语言客户端代码：

* 数据模型结构体
* 参数与数据验证逻辑
* HTTP 请求参数绑定（路径、查询、头部、请求体）
* 普通与流式（SSE）接口实现
* 服务端接口定义与路由绑定
* 客户端调用代码
* 错误码定义与管理

### 📦 丰富的数据类型支持

* 基本类型：`bool`、`int`、`float`、`string`、`bytes`
* 容器类型：`list`、`map`
* 联合类型：`oneof`
* 泛型支持：支持泛型结构体定义和实例化
* 字段修饰符：支持 `required`（必填）和 `optional`（可选）

### 🔎 高效数据验证

* 无反射实现，高性能
* 支持基于表达式的验证规则（通过 `validate` 注解）
* 枚举类型自动生成验证函数
* 支持自定义验证函数

### 🌐 HTTP 友好

* 自动绑定 HTTP 请求参数（路径、查询、头部、请求体）
* 支持 `form`、`json`、`multipart-form` 等格式
* 原生支持流式 RPC（SSE）接口
* 支持 RPC 与 SSE 接口定义

### 📝 注释与文档

* 支持单行注释（`//`、`#`）和多行注释（`/* */`）
* 支持错误码描述注解（`errmsg`、`desc`）

## 安装

- **推荐方式：**

使用 [gs](https://github.com/go-spring/gs) 集成开发工具。

- 单独安装本工具：

```bash
go install github.com/go-spring/gs-http-gen@latest
```

## 使用方法

### 第一步：定义 IDL 文件

创建 `.idl` 文件描述服务接口和数据模型。项目必须包含 `meta.json` 配置文件。

> **项目结构：**
>
> * 项目必须包含 `meta.json` 元信息配置文件
> * IDL 文件以 `.idl` 作为扩展名
> * 所有 IDL 文件共享同一命名空间，可相互引用

> **语法说明：**
>
> * 支持单行注释 (`//` 或 `#`) 和多行注释 (`/* */`)
> * 标识符必须以字母开头，后续字符可包含字母、数字、下划线和点号
> * 支持 `required` 和 `optional` 修饰符控制字段必填性

示例：

```idl
// 常量定义
const string APP_NAME = "MyApp"
const int MAX_SIZE = 100
const float PI = 3.14159
const bool DEBUG = true

// 枚举定义
type UserStatus {
    ACTIVE = 1 (desc="活跃")
    INACTIVE = 2 (desc="非活跃")
    PENDING = 3 (desc="待审核")
}

// 错误码定义
enum ErrCode {
    ERR_OK = 0 (errmsg="成功")
    PARAM_ERROR = 1003 (errmsg="参数错误")
    NOT_FOUND = 404 (errmsg="资源未找到")
}

// 扩展错误码
type enum extends ErrCode {
    USER_NOT_FOUND = 404 (errmsg="用户未找到")
    PERMISSION_DENIED = 403 (errmsg="权限不足")
}

// 普通结构体
type User {
    required string name       // 必填字段
    int age                    // 可选字段
    optional string email      // 显式声明为可选字段
    list<string> tags          // 字符串列表
    UserStatus status          // 枚举字段
}

// 泛型结构体
type Response<T> {
    int code              // 返回码
    string message        // 返回消息
    T data                // 泛型数据
}

// 泛型实例化
type UserResponse Response<User>

// oneof 联合类型
type Payload {
    string text_data
    int? number_data (json="number_data")
    bool boolean_data (json="")
}

// 请求参数
type GetUserRequest {
    required string id (path="id")
    optional string name (query="name")
}

// 响应参数
type GetUserResponse {
    required User user
}

// RPC 接口定义
rpc GetUser(GetUserRequest) GetUserResponse {
    method="GET"
    path="/users/{id}"
    summary="获取用户信息"
}

// SSE 流式接口定义
rpc UserEvents(GetUserRequest) stream<User> {
    method="GET"
    path="/users/{id}/events"
    summary="用户事件流"
}
```

### 第二步：生成代码

使用命令行工具生成代码：

```bash
# 仅生成服务端代码（默认）
gs-http-gen --server --output ./generated --go_package myservice

# 同时生成服务端和客户端代码
gs-http-gen --server --client --output ./generated --go_package myservice
```

**参数说明：**

| 参数             | 说明                    | 默认值     |
|----------------|-----------------------|---------|
| `--server`     | 生成服务端代码（HTTP 处理与路由绑定） | 否       |
| `--client`     | 生成客户端代码（HTTP 调用封装）    | 否       |
| `--output`     | 输出目录                  | `·`     |
| `--go_package` | 生成的 Go 包名             | `proto` |
| `--language`   | 目标语言（目前仅支持 `go`）      | `go`    |

### 第三步：使用生成的代码

示例：

```
// 实现服务接口
type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, req *proto.GetUserRequest) *proto.GetUserResponse {
    // 普通响应
    return &proto.GetUserResponse{
        User: &proto.User{
            Name: "Jim",
            Status: proto.UserStatus_ACTIVE,
        },
    }
}

func (s *UserService) UserEvents(ctx context.Context, req *proto.GetUserRequest, resp chan<- *proto.User) {
    // 流式响应
    for i := 0; i < 5; i++ {
        resp <- &proto.User{
            Name: fmt.Sprintf("User%d", i),
            Status: proto.UserStatus_ACTIVE,
        }
    }
}

// 注册路由
mux := http.NewServeMux()
proto.InitRouter(mux, &UserService{})

http.ListenAndServe(":8080", mux)
```

## ⚠️ 注意事项

* 生成的代码不会自动强制字段必填，需在业务逻辑中自行保证。
* 不自动调用验证逻辑 `Validate()`，如需深度校验可自行组合。
* 建议统一管理生成的代码并保持与 IDL 一致，避免手动修改导致差异。

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。
