# 复制粘贴通用代码几次以后，我才明白 Starter 是干什么的

BookMan Pro 写到这里，我开始有一种熟悉的不安。

访问日志中间件、价格 SDK、Redis 客户端、数据库初始化，这些代码看起来都不是纯业务。一个项目里写一份还好，如果以后每个服务都要复制一份，很快就会出现多个版本。

配置 key 不一样，默认值不一样，关闭资源的方式也不一样。

我以前会把这些东西叫“工具包”。但 Go-Spring 里更推荐用 Starter 来封装这类能力。

## 我希望应用侧怎么用

如果要启用一个价格 SDK，我希望业务项目里只写：

```go
import _ "bookman-pro/starter/book-price"
```

再加配置：

```properties
bookman.price.base-url=http://price.internal
bookman.price.timeout=500ms
```

如果没有配置 `bookman.price.base-url`，那就不要创建真实客户端。这样本地学习时不需要价格服务，生产环境再打开。

这就是我对 starter 的第一层理解：它把“怎么创建一个通用组件”藏起来，只把“怎么启用和配置”留给应用。

## Starter 也从配置开始

价格客户端需要两个配置：

```go
type Config struct {
	BaseURL string        `value:"${base-url}"`
	Timeout time.Duration `value:"${timeout:=500ms}"`
}
```

这里的 `base-url` 是相对 key。注册时会绑定前缀：

```go
gs.TagArg("${bookman.price}")
```

所以它最终对应 `bookman.price.base-url`。

我觉得这个写法挺适合 starter，因为组件内部不用反复写完整前缀。

## 注册 Bean 时要带条件

Starter 通常在 `init()` 里注册：

```go
func init() {
	gs.Provide(NewClient, gs.TagArg("${bookman.price}")).
		Condition(gs.OnProperty("bookman.price.base-url")).
		Export(gs.As[book_service.PriceClient]()).
		Destroy(CloseClient)
}
```

这一行刚开始看有点长，我拆开理解：

`Condition`：只有配置了价格服务地址，才创建客户端。

`Export`：把具体客户端导出成业务层需要的接口。

`Destroy`：应用关闭时释放资源。

我以前写工具包时，经常只管创建，不管关闭。starter 让我把资源生命周期也一起考虑进去。

## 构造函数保持简单

```go
func NewClient(c Config) (*Client, error) {
	return &Client{
		baseURL: c.BaseURL,
		client: &http.Client{Timeout: c.Timeout},
	}, nil
}

func CloseClient(c *Client) error {
	return nil
}
```

如果配置错了，构造函数就应该返回 error，让应用启动失败。

我现在越来越接受“启动时失败”这件事。比起运行到某个请求时才发现客户端没配好，启动阶段失败要好排查得多。

## 业务层还是依赖接口

应用导入 starter：

```go
import (
	_ "bookman-pro/internal/app"
	_ "bookman-pro/internal/biz"
	_ "bookman-pro/starter/book-price"
)
```

Service 仍然只认接口：

```go
type PriceClient interface {
	GetPrice(ctx context.Context, isbn string) (float64, error)
}
```

这样测试时可以给 fake，生产时由 starter 提供真实实现。

这也是我慢慢理解的一点：starter 不应该把业务代码绑到自己的具体类型上。

## Provide、Module、Group 怎么选

我一开始看到 `gs.Provide`、`gs.Module`、`gs.Group` 有点晕。

现在先粗略这样理解：

`Provide` 适合一个默认实例。价格 SDK 这种单实例先用它就够了。

`Module` 适合根据配置动态注册不同 Bean。比如 `mode=mock` 注册 mock，`mode=http` 注册 HTTP 客户端。

`Group` 适合多个同类实例。比如多个 Redis、多个数据库、多个价格源。

不是一开始就要用最复杂的。先用 `Provide`，等复杂度真的出现再升级。

## 验证两条路径

不配置价格服务：

```bash
go run .
```

应用应该能启动。

配置价格服务：

```bash
go run . -Dbookman.price.base-url=http://127.0.0.1:18080
```

这时 starter 才应该创建真实价格客户端。

我会用日志确认这一点：没配置时不初始化，配置后才初始化。

## 我这次踩到的坑

空白导入后就直接创建资源。导入只应该注册 Bean 定义，真正创建交给容器。

没有关键配置也创建客户端。资源型组件应该有条件。

业务层依赖 starter 的具体类型。这样测试和替换都会困难。

忘记关闭资源。数据库、Redis、长连接、后台任务都要考虑释放。

## 给自己留个小练习

把访问日志中间件也封装成 starter。

配置：

```properties
bookman.access-log.enabled=false
```

为 `false` 时关闭访问日志，默认开启。

写完这一篇，我对 starter 的理解变成了：它不是简单工具包，而是一套带配置、条件注册和生命周期的组件封装方式。
