# 项目结构先服务依赖方向

`course/06-layered-bookman` 把前几篇的单文件示例拆成一个接近真实项目的目录结构。

拆目录的目标不该停留在让项目看起来更大。BookMan Pro 现在已经有 HTTP 路由、Controller、Service、DAO、SDK、后台任务、静态文件和配置。继续放在一个 `main.go` 里，改任何地方都要先扫一遍整文件。

这一篇只关心一个问题：代码放到不同目录后，依赖方向是否更清楚。

## 当前目录

示例结构大致是：

```text
conf/
public/
internal/
  app/
    common/httpsvr/
    controller/
  biz/
    job/
    service/book_service/
  dao/book_dao/
  idl/http/proto/
  sdk/book_sdk/
main.go
init.go
```

可以先按这条方向理解：

```text
app -> biz -> dao/sdk
```

`app` 是应用入口层，处理 HTTP Server、路由、中间件和 Controller。

`biz` 放业务逻辑和后台任务。

`dao` 处理数据访问，当前是内存 Map。

`sdk` 代表外部服务调用，当前价格 SDK 返回固定价格。

`idl/http/proto` 这一层放 HTTP 接口结构和路由注册。示例里它像一层生成代码的占位，Controller 实现接口，路由注册集中在这里。

## main.go 保持短

入口文件只导入模块并启动：

```go
import (
	"go-spring.org/spring/gs"

	_ "bookman/internal/app"
	_ "bookman/internal/biz"
)

func main() {
	gs.Run()
}
```

空白导入的作用是触发各模块自己的 `init()`。入口不再负责列出每一个 Controller、DAO、SDK 和 Job。

这种写法的好处是模块可以自己声明提供什么 Bean。坏处也要接受：读代码时需要顺着包的 `init()` 找注册点。示例项目规模不大，这个取舍是合理的。

## app 层只管应用入口

`internal/app/common/httpsvr` 创建 `http.ServeMux`：

```go
func NewServeMux(c *controller.Controller) *gs.HttpServeMux {
	mux := http.NewServeMux()
	proto.RegisterRouter(mux, c, Access())
	mux.Handle("GET /", http.FileServer(http.Dir("./public")))
	return &gs.HttpServeMux{Handler: mux}
}
```

这里放路由、中间件和静态文件服务。

`internal/app/controller` 处理 HTTP 编解码：

```go
type BookController struct {
	BookService *book_service.BookService `autowire:""`
}
```

Controller 可以依赖 Service，因为它需要调用业务能力。它不应该直接操作 DAO 的存储细节，也不应该把 HTTP 状态码传进 Service。

当前代码里 `SaveBook` 的请求体类型来自 `book_dao.Book`。示例为了少引入 DTO 做了简化。长期维护时，可以把 HTTP 输入输出类型放到 `idl/http/proto` 或单独 DTO 包里，避免 Controller 直接暴露 DAO 类型。

## biz 层写业务组合

`internal/biz/service/book_service` 负责把 DAO 数据和 SDK 价格组合成接口返回：

```go
type BookService struct {
	BookDao     *book_dao.BookDao `autowire:""`
	BookSDK     *book_sdk.BookSDK `autowire:""`
	RefreshTime gs.Dync[int64]    `value:"${dync.refresh.time:=0}"`
}
```

这版 Service 直接依赖具体 DAO 和 SDK。对教学示例来说，能让读者少追一层接口；对需要替换存储或外部客户端的项目，下一步可以把这里改成接口依赖。

无论是否抽接口，Service 层都不应该依赖 `http.Request` 或 `http.ResponseWriter`。它接收 `context.Context`，返回业务数据和错误，由 Controller 决定 HTTP 响应。

后台任务放在 `internal/biz/job`：

```go
func (x *Job) Run(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(time.Millisecond * 300)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Println(time.Now().UnixMilli(), "job exit")
				return
			case <-ticker.C:
				fmt.Println(time.Now().UnixMilli(), "job sleep end")
			}
		}
	}()
	return nil
}
```

它注册成 `gs.Runner`，并监听应用 Context。位置放在 `biz` 下，是因为它属于业务运行逻辑，不是 HTTP 层能力。

## dao 和 sdk 保持外部细节

`internal/dao/book_dao` 当前用内存 Map 存图书：

```go
type BookDao struct {
	Store map[string]Book
}
```

这个包负责 `ListBooks`、`GetBook`、`SaveBook`、`DeleteBook`。排序、缺失图书错误、ISBN 校验这些和存储直接相关的逻辑放在这里。

`internal/sdk/book_sdk` 现在只有一个固定价格实现：

```go
func (s *BookSDK) GetPrice(isbn string) string {
	return "￥10"
}
```

即使实现很小，也值得单独放。外部服务调用一旦散到 Service 各处，后面补超时、日志、重试和测试替换都会变麻烦。

## 怎么判断结构有效

目录结构有效不看层数多少，看修改时影响范围是否变小。

新增 HTTP 中间件，应该主要动 `internal/app/common/httpsvr`。

调整图书保存规则，应该主要动 `internal/biz/service/book_service` 或 DAO 边界。

替换价格服务，应该主要动 `internal/sdk/book_sdk` 或对应 starter。

如果改一个 DAO 字段会牵连 Controller 和路由注册，说明目录只是移动了文件，依赖方向还没有立起来。

运行方式保持不变：

```bash
go run .
curl http://127.0.0.1:9090/books
go test ./...
```

行为没变，代码位置和依赖方向更清楚，这次拆分就达到了目的。
