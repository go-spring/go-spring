# formutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`formutil` 是 HTTP 表单 / 查询串与 Go 值之间的
泛型桥梁，供上层绑定器使用。

## 1. 职责与边界

- 只提供**单字段**、**原始类型**级别的编解码，结构体级别的绑定交给调用方
  （HTTP 框架的 binder / 声明式 client）。
- 编码与解码函数对称，同一段代码写出的字段可以被同一层读回。
- 不做校验。这里唯一的横向规则是范围检查（通过 `mathutil.Overflow*`），
  其它都交由上层。

## 2. 关键设计决策

- 使用**泛型函数集**（`Decode/EncodeInt[T]`）而非每类型一个文件。这样接口
  更扁平，代码生成器也能按字段类型直接对应到一个函数。
- 解码入参统一是 `[]string`，与 `url.Values[key]` 对齐；非 List 变体在长度
  大于 1 时报错。
- 编码侧用 **`nil` 指针表示缺省**。`Ptr` 变体让绑定器无需额外位图就能区分
  "未设置"与"零值"。
- 字节切片用 **base64**，JSON 委托给 `stdlib/jsonflow`；两处都锁死了一种
  规范表达，使得同一 binder 的两端天生对齐。

## 3. 约束

- 除 `stdlib` 内部包外，无第三方依赖。
- `EncodeFloat` / `DecodeFloat` 无论底层类型是 float32/64 都用
  `strconv.FormatFloat(..., 'f', -1, 64)`；`T = float32` 时会丢失精度信息。
- 溢出错误只是通过 `errutil.Explain` 返回的普通错误串，没有专门的
  sentinel。调用方如果要匹配，需要在上层做类型区分或字符串比对。
