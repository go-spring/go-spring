# repository
[English](README.md) | [中文](README_CN.md)

`repository` 是一个零依赖、与框架无关的通用数据访问抽象——用 Go 惯用法达到 Spring Data
`CrudRepository` + `PagingAndSortingRepository` 的等价效果。单一的
[`Repository[T, ID]`](repository.go) 接口为领域类型提供 CRUD、排序/分页查询与自动审计字段,
任意存储只需实现 [`Backend[T, ID]`](repository.go) 接缝即可接入。没有查询 DSL、没有方法名解析:
一次查询用一个朴素的 [`Query`](query.go) 值表达。

## 特性

- 泛型 `Repository[T, ID]`——`Create`、`Save`、`FindByID`、`ExistsByID`、`Delete`、
  `Count`、`FindAll`、`FindPage`。不做基于反射的方法名解析;类型参数直接承载实体与主键类型。
- `Query` = `Filters []Cond` + `Sort []Order` + `Page Pageable`——一个小而存储中立的
  Specification。`Op` 是封闭集合(`Eq/Ne/Gt/Ge/Lt/Le/In/Like`),保证每个后端都能全量覆盖。
  用链式 `NewQuery().Where(...).OrderBy(...).Slice(...)` 构造。
- `Page[T]{ Items, Total, Offset, Limit }` 带 `HasNext()`——`FindPage` 由列表加一次独立计数
  组合而成,一次调用即可渲染分页器。
- `Auditable` 可选接口——实现 `SetCreatedAt/SetUpdatedAt/SetCreatedBy`,写入时由 repository
  自动填充。`CreatedBy` 经 `PrincipalFunc` 接缝(`WithPrincipal`)从 context 取得,与安全层的
  当前用户对齐。时间戳使用可注入的 `Clock`(`WithClock`)。
- `Backend[T, ID]` 接缝——存储实现的唯一接口(把 `Query` 翻译成自身查询)。它是 bean 类型替换,
  而非全局 driver 注册表,因为后端绑定的是一个活跃客户端。

## 用法

```go
package main

import (
    "context"
    "time"

    "go-spring.org/stdlib/repository"
)

type User struct {
    ID        int64
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
    CreatedBy string
}

// 实现 Auditable 即开启自动审计。
func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (u *User) SetCreatedBy(who string)  { u.CreatedBy = who }

func demo(repo repository.Repository[User, int64]) error {
    ctx := context.Background()

    // 创建 + 审计
    if err := repo.Create(ctx, &User{ID: 1, Name: "Ann"}); err != nil {
        return err
    }

    // 组合条件 + 排序 + 分页
    page, err := repo.FindPage(ctx, repository.NewQuery().
        Where("name", repository.Like, "A%").
        OrderByDesc("created_at").
        Slice(0, 20))
    if err != nil {
        return err
    }
    _ = page.Items
    _ = page.Total // 匹配的总数,而非当前窗口
    return nil
}
```

`Repository` 由 `repository.New(backend, opts...)` 基于 `Backend` 构造。gorm 实现及其
`For[T, ID](db, table)` 工厂位于 [`starter-repository-gorm`](../../starter/starter-repository-gorm);
第二种存储(Mongo)只是再实现一份 `Backend`,不改动本包。

## 设计

见 [DESIGN_CN.md](DESIGN_CN.md)。
