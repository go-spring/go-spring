# Go-Spring 实战第 9 课 —— IoC 容器：复杂 Go 应用如何统一对象装配

上一篇咱们结束了 Go-Spring 的配置体系，大家系统性地了解了 Go-Spring 是怎样解决 Go 应用的配置绑定、校验、加载、优先级等问题的。

从这一篇开始，咱们进入 Go-Spring 的 IoC 容器体系。大家先不用着急把它想复杂，咱们从最普通的对象创建和构造函数开始看起。我会一步一步带领大家看清楚什么是依赖注入，它有什么好处，以及为什么值得使用它。

## 职责分离

在进入 IoC 容器之前，咱们先来看一个小例子。

```go
type UserController struct {
	service *UserService
}

func NewUserController() *UserController {
	return &UserController{
		service: NewUserService(),
	}
}
```

在上面的代码中，业务对象 `UserController` 在构造时自己创建了下游依赖 `UserService`。这段代码看起来很简单，也很自然，但它实际上把两件不同的事情混在了一起：`UserController` 既要负责处理用户请求，又要负责决定 `UserService` 应该如何创建。

这就是问题所在。

一个对象最核心的职责，应该是使用它的依赖完成自己的业务逻辑，而不是顺手把下游依赖也创建出来。创建者和使用者一旦混在一起，代码在早期可能不会暴露出明显问题，但是当依赖变多、实现需要替换、测试需要隔离时，就会变得越来越难维护。

**我们更推荐把创建者和使用者分开**。一个对象只需要通过构造函数声明自己需要什么依赖，至于这个依赖从哪里来、使用真实实现还是 mock 实现，则交给外部来决定就可以了。

看下面的代码。

```go
type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

在这个版本里，`UserController` 不再关心 `UserService` 如何创建，它只是通过入参表达自己需要一个 `UserService`。换句话说，我们可以将对象需要什么依赖直接写在构造函数的签名上，而不是藏在内部实现里。

这种显式声明非常重要。它能让对象的边界更加清楚，也能让测试更加容易。

看下面的代码。

```go
func TestUserController_GetUser(t *testing.T) {
	repo := NewUserRepository(db)
	service := NewUserService(repo, cache)
	controller := NewUserController(service)
	// ...
}
```

在写测试代码时，我们可以选择创建真实的 `UserService`，也可以选择创建一个 mock 实现。但最关键的在于，`UserController` 本身已经不再绑定具体的创建过程，它只是声明需求，然后使用这个依赖。

**这条编程原则是 Go-Spring 得以存在的基础**。如果我们否认这条原则，认为对象就应该在内部把所有下游依赖都创建出来，那么 Go-Spring 确实就没有存在的意义了。反过来说，只要我们认可“对象声明依赖，外部负责装配”这件事，IoC 容器就有了明确的需求。

## IoC 容器

在代码中应用了职责分离以后，对象本身会变得很干净，但是装配工作不会消失。它只是从对象内部移动到了外部：谁先创建，谁传给谁，哪些对象只需要一份，哪些对象需要在启动时完成初始化，这些事情总要有一个地方负责。

如果对象不多，那我们手工负责这个“外部装配”完全没有问题。看下面的例子。

```go
repo := NewUserRepository(db)
service := NewUserService(repo, cache)
controller := NewUserController(service)
server := NewHTTPServer(controller)
```

当应用里只有几个对象时，手工装配足够清楚。读代码的人很容易知道：`controller` 依赖 `service`，`service` 依赖 `repo` 和 `cache`，最后 `server` 依赖 `controller`。

但是当对象数量更多时，手工装配就会开始变得麻烦。

```go
db := NewDB(config.DB)
redis := NewRedis(config.Redis)
logger := NewLogger(config.Logging)
tx := NewTransactionManager(db)

userRepo := NewUserRepository(db, logger)
orderRepo := NewOrderRepository(db, logger)
couponRepo := NewCouponRepository(db, redis, logger)

userService := NewUserService(userRepo, redis, logger)
couponService := NewCouponService(couponRepo, redis, logger)
orderService := NewOrderService(orderRepo, userService, couponService, tx, logger)

userController := NewUserController(userService, logger)
orderController := NewOrderController(orderService, logger)

mux := NewHTTPMux()
mux.Handle("/users", userController)
mux.Handle("/orders", orderController)

server := NewHTTPServer(mux, logger)
```

上面这个例子里仍然使用手工依赖注入，但它已经暴露出了几个问题。

第一，装配代码变得越来越长。业务组件越多，构造顺序越复杂，入口文件越容易堆满和业务逻辑无关的对象创建代码。

第二，共享依赖会被到处传递。比如 `db`、`redis`、`logger` 这些基础设施对象，几乎每个组件都可能需要。它们本身不复杂，但在装配层反复出现，会让代码越来越臃肿。

第三，装配代码需要靠人脑维护。只要某个构造函数新增一个参数，装配代码就必须跟着调整。如果类似的装配逻辑散落在多个入口、多个测试或者多个命令行程序里，修改成本就会被放大。

当构造函数的入参越来越多，当需要共享的对象越来越多，当初始化顺序越来越依赖经验时，手工装配就不再是一个轻松的问题。我们希望找到一种方式，让对象仍然通过构造函数清楚地声明自己需要什么，但具体怎么创建、怎么传递、怎么复用，可以交给一个统一的机制来完成。

这种机制就是 **IoC 容器**。

IoC 容器要做的事情，并不是改变对象本身的业务逻辑，而是接管对象之间的装配过程。对象只需要告诉容器：我的构造函数是什么，我需要哪些参数。有了这些信息，容器就可以按需创建对象，并把创建好的对象传给下一个对象。

在此之上，IoC 容器还可以统一对象的生命周期管理。这一点在后面的文章中也会逐渐展开。

## 简单示例

大家先来看一个简单的使用 Go-Spring IoC 容器的例子。

```go
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) GetUser() string {
	return "Alice"
}

type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", c.service.GetUser())
}

func init() {
	gs.Provide(NewUserService)
	gs.Provide(NewUserController)

	// 这一步很重要，它把 IoC 容器和 HTTP 路由、控制器关联起来。
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
```

这里，我不想强调 Go-Spring 的能力有多强大，而是想和大家讨论一个更符合直觉的问题：如果我们想让容器帮忙装配对象，最自然的办法是什么？

答案其实很简单：**让对象把自己的信息暴露给容器**。

在上面的例子里，`NewUserService` 和 `NewUserController` 都被注册到了容器中。`NewUserService` 不需要入参，说明它可以直接创建。`NewUserController` 需要一个 `*UserService`，说明它依赖 `UserService`。这些信息都已经写在构造函数签名里了。

于是，容器能做的事情也很明确：

1. 看到了 `NewUserController` 需要 `*UserService`。
2. 找到了 `NewUserService` 可以创建 `*UserService`。
3. 然后调用 `NewUserService` 创建 `UserService`。
4. 再把创建好的 `UserService` 传给 `NewUserController`。
5. 最后把得到的 `UserController` 继续提供给其他需要它的对象。

也就是说，每个对象在注册自己的时候，已经通过构造函数声明了它需要什么样的依赖。只要相关对象都注册到了容器里，IoC 容器就可以根据这些声明，自动完成对象之间的装配。

这也是 Go-Spring IoC 容器最核心的使用直觉：不是把对象藏进一个神秘框架里，而是把对象的创建方式交给容器，让容器替我们完成那些重复的创建和传参工作。

另外，从这个例子里大家也可以看到，Go-Spring 的 API 设计非常简单，真正和 Go-Spring 相关的代码占比很少。主要就是在 `init` 中用 `gs.Provide` 注册构造函数，然后在 `main` 中调用 `gs.Run` 启动应用。除此之外，其他部分仍然是普通的 Go 代码：定义结构体、编写构造函数、实现 HTTP 处理方法。

这也是 Go-Spring 在 API 设计上刻意保持的风格：容器注入应该是一件很轻的事情，不应该让业务代码被框架代码淹没。对象该怎么写还是怎么写，只是在需要交给容器管理时，把构造函数注册进去即可。

## Bean

Bean 是一个约定俗成的概念，是 Go-Spring 从 Java Spring 中借来的。

在 Go-Spring 里，Bean 可以先简单理解为“由容器管理的对象”。在一个构造函数被注册到容器之后，它创建出来的对象就不再只是普通的局部变量，而是进入了容器管理的对象体系。其他对象可以通过声明依赖来使用它，容器也可以统一处理它的创建、注入和生命周期。

但 Go-Spring 对 Bean 的理解不只是“一个对象”或者“一个组件”。它也是对 Go 包组织方式的一种补充。

Go 的包非常适合组织代码文件和导出符号，但包本身并不负责创建对象，也不负责把对象装配起来。我们可以在包里定义很多函数，也可以通过包名到处引用这些函数。但如果复杂应用里的模块协作全部依赖包级函数调用，就很容易把抽象边界打散：代码看起来分了包，实际使用时却还是到处手动创建和传递对象。

复杂程序讲究抽象，也讲究模块化。包解决的是代码归属问题，Bean 解决的是对象协作问题。一个 Bean 可以代表一个服务、一个仓储、一个控制器、一个基础设施组件，或者任何需要被统一装配和管理的对象。

这就是我们说 Go-Spring 发扬了 Bean 概念的原因。它不是为了让 Go 代码变得像 Java，而是为了给 Go 应用补上一层运行时对象组织能力，让包、构造函数和依赖注入能够协同起来。

## 启动期装配

Go-Spring 只完成启动期装配，而且它也只需要启动期装配。

所谓启动期装配，就是在应用启动时完成对象创建、参数注入和初始化。等应用正式运行起来之后，这些对象就稳定下来，不再随着请求动态变化。

这个选择很重要。

如果容器支持运行时装配，表面上看起来会更灵活，但代价也很明显：系统行为会更难预测，错误可能延迟到某个请求路径才暴露。对绝大多数后端服务来说，这种复杂度并不值得。

启动期装配则相反。所有对象都在启动阶段完成创建和注入，缺少依赖、循环依赖、类型不匹配等问题都应该尽早暴露。应用一旦启动成功，我们也就能确认：该创建的对象已经创建好，该传入的依赖已经传进去，运行时只需要使用这些对象即可。

所以，不必纠结 IoC 容器会不会影响运行时逻辑，也不用担心依赖错误拖到运行时才发现。Go-Spring 的定位很清楚：**在启动阶段统一完成对象装配，然后把稳定的对象关系交给应用运行**。

---

到这里，咱们就可以给 IoC 容器做一个初步的定位了：它不是为了取代 Go 的构造函数，也不是为了让对象关系变得更神秘，而是为了把原本散落在入口代码里的对象创建、传参和复用过程，交给一个统一的机制来完成。
