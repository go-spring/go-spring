# repository 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`repository` 是 stdlib 层零依赖的通用数据访问抽象。它用 Go 泛型达到 Spring Data
`CrudRepository` + `PagingAndSortingRepository` 的等价效果——一套面向领域类型的现成持久化操作,
而非代理生成的方法名解析。存储(gorm、Mongo)通过实现 `Backend` 接缝接入,gorm 实现位于
`starter-repository-gorm`。

## 1. 职责与边界

- 通过单一泛型 `Repository[T, ID]` 接口,为领域类型 `T`(以 `ID` 为主键)提供 CRUD、
  排序/分页查询与自动审计字段,让业务代码依赖抽象而非具体存储。
- 把查询表达为存储中立的 `Query`(过滤/排序/窗口)——一个简单 Specification,而非表达式语言。
- 在与后端无关的位置填充审计字段(`Auditable`),使时间戳与 `CreatedBy` 不因存储而异。
- 不做 ORM、不做查询构造器、不做方法名解析。没有派生查询魔法,没有关联/懒加载模型;复杂或
  存储特有的查询留在存储自身的客户端里。

## 2. 关键抽象与接缝

- **`Backend[T, ID]` 接口作为存储接缝。** 没有全局 driver 注册表。后端绑定的是活跃客户端
  (`*gorm.DB`、Mongo collection),因此选择后端是 bean 类型替换——与 `spring/batch`.`JobRepository`、
  `spring/lock` 相同的取舍。`Backend` 刻意与 `Repository` 同形、仅去掉与存储无关的关切,
  使实现只是一层薄薄的 `Query` 翻译。
- **`New` 叠加与存储无关的关切。** 审计与 `FindPage` 组合(列表 + 计数)位于任何后端之上、
  收敛在 `New` 中,新后端无需重复实现。
- **`Query` 是封闭而小的 Specification。** `Op` 为 `Eq/Ne/Gt/Ge/Lt/Le/In/Like`——刻意有限,
  保证每个后端全量覆盖,避开开放表达式语言"部分支持"的陷阱。链式构造器(`Where/OrderBy/Slice`)
  让调用点保持可读。
- **`Page` 携带独立的 `Total`。** `FindPage` 分别执行后端的 `FindAll`(带窗口)与 `CountBy`
  (仅过滤、无窗口),使分页器一次调用即得到页数据与总数。
- **审计的 `who` 经 `PrincipalFunc`。** `CreatedBy` 来源是读取 context 的接缝,与安全层放入
  context 的内容对齐,而本包不 import security。

## 3. 约束

- **创建填充三个审计字段;更新只刷新 `UpdatedAt`。** `CreatedAt`/`CreatedBy` 创建后不可变,
  `Save` 不触碰它们。
- **`New` 对 nil 后端 panic。** nil 后端永远无法服务请求;在装配期失败比首次调用时失败更安全。
- **`FindByID` 未命中不是错误。** 返回 `found=false` 且 error 为 nil,使"查无结果"读起来干净。
- **字段名是可信的开发者输入,值始终参数绑定。** `Cond.Field`/`Order.Field` 来自代码而非终端用户;
  后端仍会在拼入前将其校验为标识符,而每个 `Value` 都经存储的参数绑定。

## 4. 扩展到其他存储

基于存储客户端实现 `Backend[T, ID]`,并暴露一个用 `New` 包装它的 `For` 工厂。抽象、`Query` 模型
与审计处理都不改动——Mongo 后端只需编写 `Query`→过滤文档的翻译。
