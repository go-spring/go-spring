# ctxcache

[English](README.md) | [中文](README_CN.md)

## 简介

`ctxcache` 是一个强类型的、上下文作用域的缓存包，专为请求范围（request-scoped）的数据而设计。

`ctxcache` 将并发安全、写一次（write-once）的键值存储附加到 `context.Context` 上，允许值在调用边界之间隐式传播，而无需污染函数签名。

## 特性

- **强类型安全**：通过泛型结合字符串名称和 Go 类型参数作为键标识，确保类型安全，防止不同类型但相同字符串标识符的值发生冲突
- **并发安全**：内部使用互斥锁保护映射，支持并发访问
- **写一次语义**：每个键只能被赋值一次，之后可多次读取，直到缓存被清除
- **上下文生命周期**：缓存的生命周期由 `Init` 返回的取消函数显式控制
- **请求范围隔离**：缓存附加到上下文，天然支持请求级别的数据隔离

## 使用方式

### 初始化缓存

使用 `Init` 函数将缓存附加到上下文：

```go
ctx, cancel := ctxcache.Init(ctx)
defer cancel() // 在请求边界处清理缓存
```

`Init` 是幂等的：对同一上下文重复调用会返回原始上下文和无操作的取消函数。

### 设置值

使用 `Set` 函数为键赋值：

```go
err := ctxcache.Set(ctx, "user", userInfo)
if err != nil {
    // 处理错误
}
```

每个键只能设置一次，重复设置会返回 `ErrKeyAlreadySet` 错误。

### 获取值

使用 `Get` 函数检索值：

```go
value, err := ctxcache.Get[UserType](ctx, "user")
if err != nil {
    // 处理错误
}
```

`Get` 是泛型函数，需要指定类型参数来确保类型安全。

### 清除缓存

调用 `Init` 返回的取消函数会清除所有缓存值：

```go
cancel() // 清除缓存并使其永久不可用
```

缓存一旦清除，后续的 `Get` 或 `Set` 操作都会返回 `ErrCacheAlreadyCleared` 错误。

## 错误类型

包中定义了以下错误类型：

- `ErrCacheNotInitialized`: 缓存未初始化
- `ErrCacheAlreadyCleared`: 缓存已被清除
- `ErrKeyNotSet`: 键未设置
- `ErrKeyAlreadySet`: 键已被设置

## 典型使用场景

1. **HTTP 中间件**：在请求入口处初始化缓存，在出口处清理
2. **认证用户信息**：存储经过身份验证的用户对象
3. **权限数据**：传递用户的权限列表
4. **追踪元数据**：携带链路追踪相关的上下文信息
5. **计算中间结果**：在调用链中共享已计算的中间值

## 使用示例

```go
package main

import (
	"context"
	"fmt"
	"go-spring.org/stdlib/ctxcache"
)

type User struct {
	ID   int
	Name string
}

func main() {
	ctx := context.Background()

	// 初始化缓存
	ctx, cancel := ctxcache.Init(ctx)
	defer cancel()

	// 设置用户信息
	user := User{ID: 1, Name: "Alice"}
	if err := ctxcache.Set(ctx, "user", user); err != nil {
		panic(err)
	}

	// 在下游代码中获取用户信息
	retrievedUser, err := ctxcache.Get[User](ctx, "user")
	if err != nil {
		panic(err)
	}

	fmt.Printf("User: %+v\n", retrievedUser)
}
```

## 注意事项

- `ctxcache` 不是通用缓存，专为结构化、短生命周期、进程内的上下文数据设计
- 每个键只能赋值一次，这是有意为之的设计，确保数据的不可变性
- 取消函数应在请求边界处调用，以确保请求范围的数据被正确清理
- 缓存清除后永久不可用，不应再对其进行任何操作

## 关键设计

这个包只做一个模式："一次请求、一袋带类型的值、在边界清理"。值都挂在具体的 context 上，没有进程级全局 map。

- **`Cache`**：一个受互斥锁保护的 `map[any]any`，用私有 key 挂到 context 上。每个 context 恰好一个，重复 `Init` 是幂等操作。
- **`TypedKey[T]`**：key 是 `(string, 类型)` 对，由泛型生成。名字相同但 `T` 不同，就是两个互不相干的槽位——这正是 `Get`/`Set` 必须走泛型、而不是拿 `any` 的原因。
- **显式生命周期**：`Init` 返回的取消函数清空 map，并把缓存永久标记为已清理。刻意不做"第二生命"：一旦能复用，就会在请求之间悄悄共享状态，比漏一次清理更危险。
- **Key 写一次**：让"值不会在调用方眼皮底下被改掉"成为整个包的总契约。真需要可变，就存指针，或用稳定 key 放 `sync.Map`。
- **取消函数必须调用**：通常就在中间件边界 `defer`。漏了也最多漏一个小 map，不会漏 goroutine。
- **并发安全**：一把互斥锁是最简单也最正确的选择；请求作用域的数据大多顺序流转，锁竞争基本可以忽略。

## 许可证

`ctxcache` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
