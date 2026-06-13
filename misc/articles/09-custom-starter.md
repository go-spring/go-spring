# Starter 封装组件注册，不替业务层做决定

`misc/course/09-starter` 单独演示一个价格客户端 starter。

BookMan Pro 现在的价格能力还很简单：图书列表里带一个价格字段。这个能力有两个实现：

- 应用内置的 `FixedPriceClient`，默认返回 `9.9`。
- starter 提供的 `bookprice.Client`，配置了 `bookman.price.base-url` 后才启用。

这篇的重点是 starter 的边界。它负责把一个可复用组件注册进容器，业务层仍然只依赖自己的接口。

## 业务层只认接口

接口放在 `internal/domain`：

```go
type PriceClient interface {
	GetPrice(ctx context.Context, isbn string) (float64, error)
}
```

Service 依赖这个接口：

```go
type BookService struct {
	repo  domain.BookRepository
	price domain.PriceClient
}

func NewBookService(repo domain.BookRepository, price domain.PriceClient) *BookService {
	return &BookService{repo: repo, price: price}
}
```

业务层不 import starter 包，也不知道最终用的是固定价格客户端还是 HTTP 客户端。

应用入口通过空白导入启用 starter：

```go
import (
	_ "bookman-pro-09/starter/bookprice"
)
```

空白导入只注册 Bean 定义。真正创建哪个 Bean，交给容器根据条件决定。

## 应用内置一个 fallback

没有配置价格服务时，示例仍然能启动：

```go
type FixedPriceClient struct{}

func (c *FixedPriceClient) GetPrice(ctx context.Context, isbn string) (float64, error) {
	return 9.9, nil
}
```

注册时加 `OnMissingBean`：

```go
gs.Provide(&FixedPriceClient{}).
	Condition(gs.OnMissingBean[domain.PriceClient]()).
	Export(gs.As[domain.PriceClient]())
```

这行代码的意思是：如果容器里还没有 `domain.PriceClient`，就用这个默认实现。

这样本地运行不需要价格服务。生产或集成环境配置真实地址后，starter 会提供另一个 `PriceClient`，fallback 就不会生效。

## Starter 绑定自己的配置前缀

starter 里的配置结构：

```go
type Config struct {
	BaseURL string        `value:"${base-url}"`
	Timeout time.Duration `value:"${timeout:=500ms}"`
}
```

注册时使用：

```go
gs.TagArg("${bookman.price}")
```

所以 `base-url` 实际对应：

```properties
bookman.price.base-url
```

`timeout` 对应：

```properties
bookman.price.timeout
```

组件内部写相对 key，注册时绑定前缀。starter 代码会更干净，应用侧也能看到统一的配置命名空间。

## 条件注册和生命周期

starter 的注册代码是这一篇最核心的部分：

```go
func init() {
	gs.Provide(NewClient, gs.TagArg("${bookman.price}")).
		Condition(gs.OnProperty("bookman.price.base-url")).
		Export(gs.As[domain.PriceClient]()).
		Destroy(CloseClient)
}
```

拆开看：

`Condition(gs.OnProperty("bookman.price.base-url"))`
只有配置了价格服务地址，才创建真实客户端。

`Export(gs.As[domain.PriceClient]())`
把具体类型导出成业务层需要的接口。

`Destroy(CloseClient)`
应用关闭时释放资源。当前示例的 `CloseClient` 没有实际工作，但真实 HTTP 连接池、数据库、Redis、消息客户端都需要考虑关闭。

构造函数可以返回 error：

```go
func NewClient(c Config) (*Client, error) {
	return &Client{baseURL: c.BaseURL, client: &http.Client{Timeout: c.Timeout}}, nil
}
```

配置不合法时应该在启动阶段失败，不要等到某次请求才发现客户端不可用。

## 两条运行路径

不配置价格服务，使用 fallback：

```bash
go run .
curl http://127.0.0.1:9090/books
```

启用 starter 提供的客户端：

```bash
go run . -Dbookman.price.base-url=http://127.0.0.1:18080
curl http://127.0.0.1:9090/books
```

示例里的 `bookprice.Client` 仍然返回固定值 `42`，没有真的访问远端服务。这里故意把网络调用简化掉，重点放在 starter 的注册、条件、导出和销毁流程。

## 什么时候需要 starter

一个组件满足这些特征时，适合封成 starter：

- 多个项目都会用。
- 有自己的配置前缀和默认值。
- 创建过程不应该散落在业务项目里。
- 需要条件启用。
- 需要关闭资源。
- 业务层应该依赖接口，而不是组件具体类型。

访问日志、价格 SDK、数据库、Redis、消息客户端都属于这类候选。小到只有一个纯函数的工具包，不需要强行做成 starter。
