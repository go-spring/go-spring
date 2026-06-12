# testing

[English](README.md) | [中文](README_CN.md)

Go-Spring Testing 是一个为 Go 语言设计的优雅测试断言库，提供了流畅（Fluent）的 API 风格，让你的测试代码更加清晰易读。

## 特性概述

- 📝 **双模式支持**：提供 `assert` 和 `require` 两种模式，满足不同场景需求
- 💧 **流畅 API**：链式调用让代码更易读，接近自然语言
- 🏷️ **类型安全**：泛型保证类型安全，编译期检查错误
- 🔧 **类型专属**：针对不同数据类型提供专门的断言方法
- 🧩 **功能丰富**：覆盖日常测试绝大多数断言需求，支持通用值、错误、数字、字符串、切片、字典、Panic 检测等
- ✅ **零依赖**：仅依赖 Go 标准库

## assert vs require

Go-Spring Testing 提供两个包来满足不同的测试需求：

### `assert` 包

`assert` 包提供的断言函数在**断言失败时不会终止测试函数的执行**。

当断言失败时，测试会继续运行，后续断言依然会被检查。这在希望在一次测试运行中报告多个失败，一次性看到所有问题的情况下非常有用。

### `require` 包

`require` 包提供的断言函数在**断言失败时会立即停止测试函数的执行**。

当断言失败时，测试立刻终止，后续断言不再被检查。这适合关键条件不满足时，后续断言可能会导致 panic 或其他问题的场景。
比如，当你需要验证一个对象非空才能继续后续操作时。

## 基本示例

```go
package main

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
	"github.com/go-spring/stdlib/testing/require"
)

func TestExample(t *testing.T) {
	// 通用断言 - 任何类型都可以使用
	assert.That(t, "hello").Equal("hello")        // 相等断言
	assert.That(t, user).NotNil()                 // 非空断言
	assert.That(t, 42).True()                     // 布尔值为真

	// 使用 require - 如果失败，测试立刻停止
	require.That(t, user).NotNil()

	// 错误断言
	err := someFunc()
	assert.Error(t, err).NotNil()                 // 期望发生错误
	assert.Error(t, err).Is(os.IsNotExist)         // 使用 errors.Is 检查错误类型

	// 数字断言
	assert.Number(t, 42).GreaterThan(40)          // 大于
	assert.Number(t, 100).Between(0, 200)          // 在区间内
	assert.Number(t, 0).Zero()                     // 等于零
	assert.Number(t, 3.14).InDelta(math.Pi, 0.01)  // 浮点数精度比较

	// 字符串断言
	assert.String(t, "user@example.com").IsEmail()      // 验证邮箱格式
	assert.String(t, "hello world").Contains("world")   // 包含子串
	assert.String(t, "hello").HasPrefix("he")            // 前缀检查
	assert.String(t, `{"name": "bob"}`).JSONEqual(`{"name":"bob"}`) // JSON 相等比较

	// 切片断言
	assert.Slice(t, []int{1, 2, 3}).Contains(2)         // 包含元素
	assert.Slice(t, []int{1, 2, 3}).Length(3)           // 长度检查
	assert.Slice(t, []int{1, 2, 3}).NotEmpty()           // 非空检查
	assert.Slice(t, []int{1, 2, 3}).AllUnique()         // 所有元素唯一

	// Map 断言
	m := map[string]int{"a": 1, "b": 2}
	assert.Map(t, m).ContainsKey("a")                    // 包含键
	assert.Map(t, m).ContainsKeyValue("a", 1)           // 包含键值对
	assert.Map(t, m).Length(2)                           // 长度检查

	// Panic 断言
	assert.Panic(t, func() {
		panic("something wrong happened")
	}, "wrong")  // 断言会 panic，且 panic 信息包含 "wrong"
}
```

## 断言方法大全

### 通用断言 (That)

所有类型都可以使用这些通用断言方法，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `True(...msg)` | 验证布尔值为 `true` |
| `False(...msg)` | 验证布尔值为 `false` |
| `Nil(...msg)` | 验证值为 `nil`（正确处理接口类型中的 nil）|
| `NotNil(...msg)` | 验证值不为 `nil` |
| `Equal(expected, ...msg)` | 使用 `reflect.DeepEqual` 深度比较是否相等 |
| `NotEqual(expected, ...msg)` | 验证不深度相等 |
| `Same(expected, ...msg)` | 使用 `==` 比较是否完全相同（指针地址相同）|
| `NotSame(expected, ...msg)` | 使用 `!=` 比较是否不同 |
| `TypeOf(interface, ...msg)` | 验证类型可赋值给目标类型 |
| `Implements(interface, ...msg)` | 验证类型实现了指定接口 |
| `Has(expected, ...msg)` | 调用值的 `Has` 方法，验证返回 `true` |
| `Contains(expected, ...msg)` | 调用值的 `Contains` 方法，验证返回 `true` |

### 错误断言 (Error)

专门用于 `error` 类型的断言，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Nil(...msg)` | 验证错误为 `nil` |
| `NotNil(...msg)` | 验证错误不为 `nil` |
| `Is(target, ...msg)` | 使用 `errors.Is` 验证错误是目标错误 |
| `NotIs(target, ...msg)` | 使用 `errors.Is` 验证不是目标错误 |
| `String(expect, ...msg)` | 验证错误信息字符串相等 |
| `Matches(pattern, ...msg)` | 验证错误信息匹配正则表达式 |

### 数字断言 (Number)

支持所有数字类型（`int`/`uint`/`float` 等）的断言，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Equal(expect, ...msg)` | 等于 |
| `NotEqual(expect, ...msg)` | 不等于 |
| `GreaterThan(expect, ...msg)` | 大于 |
| `GreaterOrEqual(expect, ...msg)` | 大于等于 |
| `LessThan(expect, ...msg)` | 小于 |
| `LessOrEqual(expect, ...msg)` | 小于等于 |
| `Zero(...msg)` | 等于零 |
| `NotZero(...msg)` | 不等于零 |
| `Positive(...msg)` | 正数 |
| `NotPositive(...msg)` | 非正数（≤ 0）|
| `Negative(...msg)` | 负数 |
| `NotNegative(...msg)` | 非负数（≥ 0）|
| `Between(lower, upper, ...msg)` | 在区间内（包含端点）|
| `NotBetween(lower, upper, ...msg)` | 不在区间内 |
| `InDelta(expect, delta, ...msg)` | 在期望误差范围内 |
| `IsNaN(...msg)` | 是 NaN（仅对浮点数有效）|
| `IsInf(sign, ...msg)` | 是无穷大（sign ≥ 0 为 +Inf，< 0 为 -Inf）|
| `IsFinite(...msg)` | 是有限数（不是 NaN 也不是 Inf）|

### 字符串断言 (String)

专门用于 `string` 类型的断言，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Length(length, ...msg)` | 验证长度 |
| `Blank(...msg)` | 验证为空或全空白字符 |
| `NotBlank(...msg)` | 验证不为空白 |
| `Equal(expect, ...msg)` | 等于 |
| `NotEqual(expect, ...msg)` | 不等于 |
| `EqualFold(expect, ...msg)` | 忽略大小写相等 |
| `JSONEqual(expect, ...msg)` | 反序列化 JSON 后比较结构相等 |
| `Matches(pattern, ...msg)` | 匹配正则表达式 |
| `HasPrefix(prefix, ...msg)` | 以指定前缀开头 |
| `HasSuffix(suffix, ...msg)` | 以指定后缀结尾 |
| `Contains(substr, ...msg)` | 包含子串 |
| `IsLowerCase(...msg)` | 全小写 |
| `IsUpperCase(...msg)` | 全大写 |
| `IsNumeric(...msg)` | 全数字 |
| `IsAlpha(...msg)` | 全字母 |
| `IsAlphaNumeric(...msg)` | 全字母数字 |
| `IsEmail(...msg)` | 验证是合法邮箱地址 |
| `IsURL(...msg)` | 验证是合法 URL |
| `IsIPv4(...msg)` | 验证是合法 IPv4 地址 |
| `IsHex(...msg)` | 验证是合法十六进制字符串 |
| `IsBase64(...msg)` | 验证是合法 Base64 编码 |

### 切片断言 (Slice)

专门用于切片类型 `[]T` 的断言，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Length(length, ...msg)` | 验证长度 |
| `Nil(...msg)` | 验证为 nil |
| `NotNil(...msg)` | 验证不为 nil |
| `Empty(...msg)` | 验证为空（长度为 0）|
| `NotEmpty(...msg)` | 验证不为空 |
| `Equal(expect, ...msg)` | 切片完全相等（元素顺序和值都一致）|
| `NotEqual(expect, ...msg)` | 验证不相等 |
| `Contains(element, ...msg)` | 包含元素 |
| `NotContains(element, ...msg)` | 不包含元素 |
| `ContainsSlice(sub, ...msg)` | 包含子切片（连续）|
| `NotContainsSlice(sub, ...msg)` | 不包含子切片 |
| `HasPrefix(prefix, ...msg)` | 以指定切片为前缀 |
| `HasSuffix(suffix, ...msg)` | 以指定切片为后缀 |
| `AllUnique(...msg)` | 所有元素都唯一 |
| `AllMatches(fn, ...msg)` | 所有元素都满足条件函数 |
| `AnyMatches(fn, ...msg)` | 至少有一个元素满足条件函数 |
| `NoneMatches(fn, ...msg)` | 没有元素满足条件函数 |

### Map 断言 (Map)

专门用于字典类型 `map[K]V` 的断言，**所有方法都支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Length(length, ...msg)` | 验证长度 |
| `Nil(...msg)` | 验证为 nil |
| `NotNil(...msg)` | 验证不为 nil |
| `Empty(...msg)` | 验证为空 |
| `NotEmpty(...msg)` | 验证不为空 |
| `Equal(expect, ...msg)` | 完全相等 |
| `NotEqual(expect, ...msg)` | 不相等 |
| `ContainsKey(key, ...msg)` | 包含键 |
| `NotContainsKey(key, ...msg)` | 不包含键 |
| `ContainsValue(value, ...msg)` | 包含值 |
| `NotContainsValue(value, ...msg)` | 不包含值 |
| `ContainsKeyValue(key, value, ...msg)` | 包含指定键值对 |
| `ContainsKeys(keys, ...msg)` | 包含所有指定键 |
| `NotContainsKeys(keys, ...msg)` | 不包含任何指定键 |
| `ContainsValues(values, ...msg)` | 包含所有指定值 |
| `NotContainsValues(values, ...msg)` | 不包含任何指定值 |
| `SubsetOf(expect, ...msg)` | 当前 map 是 expect 的子集（所有键值对都存在于 expect）|
| `SupersetOf(expect, ...msg)` | 当前 map 是 expect 的超集（expect 所有键值对都存在于当前）|
| `HasSameKeys(expect, ...msg)` | 与 expect 拥有完全相同的键集合 |
| `HasSameValues(expect, ...msg)` | 与 expect 拥有完全相同的值集合（不关心顺序）|

### Panic 断言

用于检测函数是否会 panic，顶层函数，**支持在最后添加 `msg ...string` 参数自定义错误信息**。

| 方法 | 说明 |
|------|------|
| `Panic(t, fn, pattern, ...msg)` | 断言 `fn` 会发生 panic，并且 panic 信息匹配正则表达式 `pattern` |

## 许可证

Apache License 2.0
