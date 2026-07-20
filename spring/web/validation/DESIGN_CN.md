# validation 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`validation` 是 stdlib 层零依赖的结构体校验抽象。推荐生产 driver
(`go-playground/validator`)在 `starter-validation` 里,blank import 时自
注册。

## 1. 职责与边界

- 回答"这个 struct 是否合法?",失败时产出**扁平**的 `ValidationErrors` 列
  表——由中立 `FieldError` 组成,任何 driver 都能输出。
- 提供两条入口:配置绑定一次性(`Validate(ctx, name, v)`)与 Web 请求
  (`Handle[T]`)。其余要么在 driver 侧,要么在 i18n 侧。
- **不是**i18n 库。消息模板与本地化不在本包——`Localize` 接收
  `func(key, args...) string`。

## 2. 关键抽象与缝隙

- `Validator`——`Validate(ctx, v) error`。成功返 `nil`(不是空
  `ValidationErrors`),让调用方保持 `err != nil` 惯用法。
- `Driver.NewValidator()`——工厂,通过包级注册表登记。`starter-validation`
  把 `validator.ValidationErrors` 字段映射到 `FieldError`(tag→Rule、
  Namespace→Field、Param→Param)。
- `FieldError.MessageKey()`——确定的 i18n key 约定:`"validation." + Rule`
  (如 `validation.email`)。模板用位置参数 `{0}` = 字段名,`{1}` = param。
- `ValidationErrors.Localize(msg)`——让 validation 不 import i18n 的关键缝
  隙。翻译缺失(`msg` 返 `""`)时回退到 `FieldError.Default()`,输出永不为
  空。
- Web 缝隙:`Handle[T](v, decode, render, next)` 是 `aspect.NewHandler` /
  `resilience.NewHandler` 的传输层对应物。`WriteError` 已导出,做自定义 binder
  的 adapter 可复用 400 body 形状(`{"errors":[...]}`)。
- `Decoder[T] = func(*http.Request, *T) error`。默认 `JSONDecoder`;gin /
  echo / hertz 适配器可以提供自家 binder 而不丢失校验外壳。

## 3. 约束(禁止破坏)

- **Driver 成功返 `nil`,失败返 `ValidationErrors`**。其他类型的 error 是
  driver bug——`WriteError` 会把它作为纯文本 400 输出(有意的可见降级)。
- **`FieldError.Field` 必须是 struct 字段路径**,不是 JSON tag。struct 路径
  是稳定标识;消息渲染可按需覆盖。
- **`Localize` 绝不能返回空字符串**。消息查询返 `""`(缺 key 或空 lookup)
  时,回退 `FieldError.Default`。
- **`Handle[T]`:nil validator = 透传**(仍会解码)。缝隙在未接入 validator
  前保 no-op。

## 4. 权衡 / 未做的方案

- **不 import i18n**。会把翻译 bundle 拖进每个下游 stdlib 消费者。
  `Localize` 收任意查询函数;i18n 可选。
- **没有自家 struct-tag DSL**。tag 属于 driver(`validator.v10` 用
  `validate:"..."`);stdlib 只拥有中立上报形状。
- **`Validate` 便捷函数是一次性的**。解析 driver、构建 validator、校验一步
  到位——适合启动时一次性校验的配置绑定路径,不是热路径原语。
- **注册表与 `discovery.Register` 同构**——空名 / nil / 重名 panic;重复接
  线是接线 bug,init 时 fail-loud。
