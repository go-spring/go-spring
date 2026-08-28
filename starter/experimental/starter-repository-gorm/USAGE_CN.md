# starter-repository-gorm 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照本模块源码
（`starter.go`、`backend.go`）、它实现的抽象（`cloud/data/repository` 的 `repository.go`、
`query.go`、`audit.go`）以及可运行、自带断言的 [example/](example/)（内存 sqlite +
`check.sh` 冒烟）核对。**GORM 自身的链式构建器语义见 [gorm 官方文档](https://gorm.io/docs/)**——
下文只写 go-spring 的增量。

**激活方式**：无。这是 library-first 集成模块，不是 blank-import starter——
`repository.Repository` 以应用拥有的领域类型为参数，没有任何可自动注册的东西
（`starter.go:22-25`）。唯一入口是 `For`。

---

## 1. 完整工程示例

一个带审计字段、分页、可用冒烟验证行为契约的 person-service：

```
demo/
├── go.mod
├── main.go
├── person.go          # 实体 + Auditable 实现
├── service.go         # 业务 service，只依赖 Repository 接口
└── conf/app.properties
```

**go.mod**（关键依赖）：

```
require (
    gorm.io/gorm                      latest
    gorm.io/driver/sqlite             latest   // 或经 starter-gorm-* starter 使用 mysql/postgres
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-gorm-sqlite latest   // 发布 *gorm.DB bean（任意方言均可）
    go-spring.org/starter-repository-gorm latest
)
```

**person.go** —— 实体自己承担审计接线：

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/data/repository"
)

type principalKey struct{}

func withUser(ctx context.Context, user string) context.Context {
    return context.WithValue(ctx, principalKey{}, user)
}

func currentUser(ctx context.Context) string {
    s, _ := ctx.Value(principalKey{}).(string)
    return s
}

// Person 实现 repository.Auditable：三个 setter，无需逐字段代码。
type Person struct {
    ID        int64     `gorm:"primaryKey;column:id"`
    Name      string    `gorm:"column:name"`
    Age       int       `gorm:"column:age"`
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
    CreatedBy string    `gorm:"column:created_by"`
}

func (Person) TableName() string { return "people" }
func (p *Person) SetCreatedAt(t time.Time) { p.CreatedAt = t }
func (p *Person) SetUpdatedAt(t time.Time) { p.UpdatedAt = t }
func (p *Person) SetCreatedBy(who string)  { p.CreatedBy = who }
```

**service.go** —— 业务代码完全不知道后端是 gorm：

```go
package main

import (
    "context"

    "go-spring.org/cloud/data/repository"
    "go-spring.org/spring/gs"
    reposgorm "go-spring.org/starter-repository-gorm"
    "gorm.io/gorm"
)

type PersonService struct {
    Repo repository.Repository[Person, int64] `autowire:""`
}

func newPersonService(repo repository.Repository[Person, int64]) *PersonService {
    return &PersonService{Repo: repo}
}

func init() {
    gs.Provide(func(db *gorm.DB) repository.Repository[Person, int64] {
        return reposgorm.For[Person, int64](db, "people",
            repository.WithPrincipal(currentUser)) // CreatedBy 取自请求 context
    })
    gs.Provide(newPersonService).Export(gs.As[gs.Rooter]())
}
```

**main.go**：

```go
package main

import "go-spring.org/spring/gs"

func main() { gs.Run() }
```

**conf/app.properties**：无需配置——本模块**零配置 key**（`schema.json` 声明空属性集；
example 的配置文件存在只是让 gs 有目录可加载）。具体数据库由你 import 的
`starter-gorm-*` 方言 starter 决定。

**验证**（与 example 冒烟同构）：

```bash
cd demo && CGO_ENABLED=1 go run .
# create + audit OK: 4 rows, createdBy=alice, timestamps set
# findByID / existsByID OK
# composite condition + sort OK: [Cate Bob Dan]
# paging OK: window=[Dan Bob] total=3 hasNext=true
# save OK: updatedAt refreshed, createdBy preserved
# delete OK: 3 rows remain
```

或直接跑自带 example：
`cd starter/experimental/starter-repository-gorm/example && ./check.sh`（首个偏差即非零退出；
断言清单见 `example/example.go:143-245`）。

---

## 2. 装配与时序

没有需要等待的生命周期：`For` 是普通构造函数。真正值得走读的是**每次调用**发生了什么，
因为泛型包装在 gorm backend 之上叠加了存储无关关注点（`repository.go:150-162`）：

```
For[T, ID](db, "people", opts...)
  ├─ resolvePrimaryKey[T](db)          解析 T 的 gorm schema → PK 列（回退 "id"）
  └─ repository.New(backend, opts...)  包装 backend；nil backend 直接 panic（装配期暴露）

repo.Create(ctx, entity)
  ├─ applyCreateAudit                  SetCreatedAt/SetUpdatedAt/SetCreatedBy  (audit.go:55-65)
  └─ backend.Create                    INSERT ...（审计已填好）

repo.Save(ctx, entity)
  ├─ applyUpdateAudit                  仅 SetUpdatedAt；created-by/at 创建后不可变 (audit.go:70-74)
  └─ backend.Save                      gorm Save = upsert

repo.FindPage(ctx, q)
  ├─ backend.FindAll(q)                filters → sort → offset/limit
  └─ backend.CountBy(q)                仅 filters——Total 按设计忽略 sort/窗口
```

设计理由（源自源码注释）：审计放在泛型层，同一实体无论落到 SQL、Mongo 还是内存存储
时间戳都正确（`repository.go:42-44`）；`CountBy` 忽略排序与窗口，因为分页器需要的是
**过滤器**匹配的全部行数（`backend.go:113-115`）。

经 `gs.Provide` 注册时，repository bean 与普通 Provide bean 一样惰性实例化——要么导出、
要么被注入，否则（prod 下）永远不会构建。

## 3. API 参考

### 3.1 工厂

```go
func For[T any, ID comparable](db *gorm.DB, table string, opts ...repository.Option) repository.Repository[T, ID]
```

- `resolvePrimaryKey` 解析 T 的 gorm schema，取 `PrioritizedPrimaryField.DBName`，
  回退 `"id"`——因此 `FindByID`/`ExistsByID`/`Delete` 即使在覆写表名下也构造显式
  `WHERE <pk> = ?`（`backend.go:47-59`）。
- 返回值的并发安全性与底层 `*gorm.DB` 完全一致。
- Options：`repository.WithPrincipal(fn PrincipalFunc)`（从 ctx 填 CreatedBy；不配则
  CreatedBy 留空但时间戳照填）、`repository.WithClock(Clock)`（测试确定性；生产用
  `time.Now`）。

### 3.2 Repository 方法（你得到什么）

| 方法 | 行为 |
|------|------|
| `Create(ctx, *T) error` | INSERT；先填全部三个审计字段；指针参数让 gorm 回写生成键 |
| `Save(ctx, *T) error` | upsert；仅刷新 UpdatedAt |
| `FindByID(ctx, ID) (T, bool, error)` | 未命中 = `found=false, err=nil`；显式 PK WHERE + `Take` |
| `ExistsByID(ctx, ID) (bool, error)` | `Count ... Limit 1`，不物化实体 |
| `Delete(ctx, ID) error` | 删除不存在的 id **不是**错误 |
| `Count(ctx) (int64, error)` | 总行数（空 Query 的 CountBy） |
| `FindAll(ctx, Query) ([]T, error)` | 零值 Query = 全部行；不计算总数 |
| `FindPage(ctx, Query) (Page[T], error)` | 窗口内条目 + 过滤器匹配的 Total；窗口无界时 `Page.HasNext()` 恒 false |

Query 流式构建：`repository.NewQuery().Where(field, op, value).OrderBy(f) /
.OrderByDesc(f).Slice(offset, limit)`。操作符：`Eq Ne Gt Ge Lt Le In Like`——`In` 绑定
slice 值；`Like` 期望调用方的值自带通配符（`backend.go:143-167`）。

### 3.3 标识符校验（拒绝什么、何时拒绝）

每个进入 SQL 文本的字段名——`Cond.Field` 或 `Order.Field`——必须匹配
`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`（`backend.go:34`）；违反则调用
以 `repository-gorm: invalid filter field %q` / `invalid sort field %q` 失败，**发生在任何
查询执行之前**。字段名来自开发者代码而非终端用户输入——该校验是廉价保险；**值永远走
gorm 的 `?` 参数绑定**，绝不内插。不支持的 `Op` 同样被拒绝（`unsupported operator`）。
表名原样传给 gorm 的 `Table()`——若表名是动态的请自行校验。

### 3.4 扩展点

- **`repository.Backend[T, ID]`** —— 第二个存储实现的唯一 seam；`New` 在其上叠审计与
  分页。不是驱动注册表：选 backend 是 bean 类型选择。
- **`repository.Auditable`** —— 按实体 opt-in；在 setter 里丢弃值即不追踪该字段。
- **`PrincipalFunc`** —— 接到安全层放进 context 的同一 subject 上。
- 抽象自带成熟度说明：目前只有 gorm backend；多存储契约未经第二存储验证。

## 4. 验证与故障演练

1. **审计填充**：以 `withUser(ctx, "alice")` 创建，断言 `CreatedBy == "alice"` 且
   `CreatedAt` 非零（example 正是这么做的，`example/example.go:159-163`）。
2. **创建审计不可变**：`Save` 已有行、reload，断言 `CreatedBy` 不变、`UpdatedAt` 前进
   （example `example.go:216-229`）。
3. **分页契约**：3 条匹配上 `Where("age", Ge, 30).OrderBy("age").Slice(0, 2)` →
   2 条、`Total == 3`、`HasNext() == true`。
4. **注入防护（负向）**：`repo.FindAll(ctx, repository.NewQuery().Where("age = 1; DROP TABLE people", repository.Eq, 1))`
   → 报错 `invalid filter field`，表完好。`OrderBy("name; DROP ...")` 同理。
5. **PK 回退**：实体没有 gorm primaryKey tag → `FindByID` 查 `WHERE id = ?`
   （Spring Data 惯例）。

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| `invalid filter field` / `invalid sort field` | 字段名不过标识符正则（含一切注入尝试） | 用裸列名；需要限定时写 `table.column`——正则允许一个点 |
| `unsupported operator` | `repository.Op` 值超出封闭集合 | 只用 Eq/Ne/Gt/Ge/Lt/Le/In/Like |
| `CreatedBy` 始终为空 | 未配 `WithPrincipal`，或 ctx 里没有 principal | 接 `WithPrincipal(currentUser)`；在写操作使用的 ctx 放入 subject |
| `UpdatedAt` 不刷新 | 实体未实现 `Auditable`（三个 setter 缅一不可） | 实现完整接口 |
| `FindByID` 查不到实际存在的行 | PK 列走了 `"id"` 回退而真实 PK 列名不同 | 给字段打 `gorm:"primaryKey"` 让 schema 解析命中 |
| repository bean 从未构造 | Provide bean 既未被注入也未导出 | `.Export(gs.As[gs.Rooter]())` 或注入它（gs 只装配根可达 bean） |
| `LIKE` 查不到 | 值缺通配符——backend 不补 | 自己传 `"%bob%"` |
| `FindPage` 的 Total ≠ len(Items) | 按设计——Total 忽略窗口 | 这就是契约；用 `HasNext()` 翻页 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 0（已核对：模块内无 `value:` tag；`schema.json` 为空） |
| 其中必填 | 0 |
| quickstart 前置外部依赖数 | 0（内存 sqlite） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单：`For` 收 `*gorm.DB` 而方言 starter 发布的是 gormcore 包装类型，每个调用点
被迫 `.DB` 解包（gormcore 层加一个 `For` 重载可隐藏）；仅有一个 backend 存在，`Backend`
seam 的多存储契约未验证；Repository 透不出 gorm 的 soft-delete / hook / preload（属范围
裁剪——逃生口是底层 `*gorm.DB`）。
