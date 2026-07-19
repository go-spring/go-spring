# starter-repository-gorm

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布,欢迎使用!

`starter-repository-gorm` 是与框架无关的
[`go-spring.org/stdlib/repository`](../../stdlib/repository) 抽象的
[gorm](https://gorm.io) 后端实现。它把 `repository.Query` 翻译成 gorm 的链式构造器,
基于任意 `*gorm.DB` 返回一个开箱即用的泛型 `repository.Repository[T, ID]`——用 Go 惯用法达到
Spring Data JPA repository 的等价效果,而无需 JPA 或方法名查询解析。

它是**以库为先的集成模块**,而非空导入型 starter:repository 以应用自有的领域类型为参数,
因此没有可自动注册的东西。它**与驱动无关**——具体数据库(MySQL、Postgres、SQL Server、
ClickHouse、sqlite……)由发布 `*gorm.DB` bean 的那个 `starter-gorm-*` 决定。

## 安装

```bash
go get go-spring.org/starter-repository-gorm
```

## 用法

### 基于已持有的 `*gorm.DB` 内联构造

服务构造函数内的自然形态:

```go
import (
    reposgorm "go-spring.org/starter-repository-gorm"
    "go-spring.org/stdlib/repository"
    "gorm.io/gorm"
)

type UserService struct {
    repo repository.Repository[User, int64]
}

func newUserService(db *gorm.DB) *UserService {
    return &UserService{repo: reposgorm.For[User, int64](db, "users")}
}
```

`For` 会从 `T` 的 gorm schema 解析主键列(缺省回退到 `"id"`)。

### 作为具名 IoC bean

通过朴素的 `gs.Provide` 构造函数注册 `For`,使其他 bean 按接口自动装配 repository,
而无需知道它由 gorm 支撑:

```go
gs.Provide(func(db *gorm.DB) repository.Repository[User, int64] {
    return reposgorm.For[User, int64](db, "users",
        repository.WithPrincipal(currentUser)) // 开启审计 CreatedBy
}).Name("userRepo")
```

### CRUD、分页、组合条件

```go
_ = repo.Create(ctx, &User{Name: "Ann"})
u, found, _ := repo.FindByID(ctx, 1)

page, _ := repo.FindPage(ctx, repository.NewQuery().
    Where("age", repository.Ge, 18).
    Where("name", repository.Like, "A%").
    OrderByDesc("created_at").
    Slice(0, 20))
// page.Items 是当前窗口;page.Total 是全部匹配的总数。
```

### 审计字段

让实体实现 `repository.Auditable`,并用 `repository.WithPrincipal` 开启;写入时填充
`CreatedAt`/`UpdatedAt`,`CreatedBy` 来自请求 context:

```go
func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (u *User) SetCreatedBy(who string)  { u.CreatedBy = who }
```

## 查询翻译

| `repository.Op` | SQL |
|---|---|
| `Eq` / `Ne` | `field = ?` / `field <> ?` |
| `Gt` / `Ge` / `Lt` / `Le` | `field > ?` / `>= ?` / `< ?` / `<= ?` |
| `In` | `field IN ?`(绑定切片) |
| `Like` | `field LIKE ?`(通配符由调用方提供) |

`Sort` 变为 `ORDER BY field ASC|DESC`;`Page` 变为 `OFFSET`/`LIMIT`。`FindPage` 执行
一次带窗口的 `Find` 取数据、一次仅过滤的 `Count` 取总数。字段名在拼入前校验为标识符;
值始终经 gorm 的参数绑定。

## 设计

这是构建在共享 starter 约定之上的数据库集成模块——见
[starter/DESIGN.md](../DESIGN.md)。第二种存储(如 Mongo)是独立的 `repository.Backend`
实现,不触碰本模块或抽象。

## 示例

可运行的冒烟测试位于 [`example/`](example):它把 repository 作为 IoC bean 基于内存 sqlite
装配,并驱动 CRUD、分页、组合条件与审计填充。用 `example/check.sh` 运行。
