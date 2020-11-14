# spring-boot

### *_CommandLineRunner_*

```
func Run(ctx ApplicationContext)
```

### *_ApplicationEvent_*

```
OnStopApplication(ctx ApplicationContext)  // 应用停止的事件
```

```
OnStartApplication(ctx ApplicationContext) // 应用启动的事件
```

### *_Banner_*

#### SetBanner

设置自定义 Banner 字符串

```
func SetBanner(banner string)
```

### *_PropertySource_*

#### RegisterPropertySource

注册属性源

```
func RegisterPropertySource(ps PropertySource)
```

#### Scheme

返回属性源的标识

```
func Scheme() string
```

#### Load

加载属性文件，profile 配置文件剖面，fileLocation 和属性源相关。

```
Load(fileLocation string, profile string) map[string]interface{}
```

### *_ConfigReader_*

#### RegisterConfigReader

注册配置读取器

```
func RegisterConfigReader(reader ConfigReader)
```

#### RegisterFileConfigReader

注册基于文件的配置读取器

```
func RegisterFileConfigReader(fileExt string, fn FnReadBuffer)
```

#### FileExt

```
func FileExt() string // 文件扩展名
```

#### ReadFile

```
func ReadFile(filename string, out map[string]interface{})
```

#### ReadBuffer

```
func ReadBuffer(buffer []byte, out map[string]interface{})
```

### *_gRPC_*

#### RegisterGRpcServer

注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。

```
func RegisterGRpcServer(fn interface{}, serviceName string, server interface{}) *GRpcServer
```

#### RegisterGRpcClient

注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数

```
func RegisterGRpcClient(fn interface{}, endpoint string) *SpringCore.BeanDefinition
```

### *_Message_*

#### BindConsumer

注册 BIND 形式的消息消费者

```
func BindConsumer(topic string, fn interface{}) *ConditionalBindConsumer
```

### *_Application_*

#### SetBannerMode

设置 Banner 的显式模式

```
func SetBannerMode(mode BannerMode)
```

#### ExpectSysProperties

期望从系统环境变量中获取到的属性，支持正则表达式

```
func ExpectSysProperties(pattern ...string)
```

#### AfterPrepare

注册一个 app.prepare() 执行完成之后的扩展点

```
func AfterPrepare(fn AfterPrepareFunc)
```

#### RunApplication

快速启动 SpringBoot 应用

```
func RunApplication(configLocation ...string)
```

#### Exit

退出 SpringBoot 应用

```
func Exit()
```

#### GetProfile

返回运行环境

```
func GetProfile() string
```

#### SetProfile

设置运行环境

```
func SetProfile(profile string)
```

#### AllAccess

返回是否允许访问私有字段

```
func AllAccess() bool
```

#### SetAllAccess

设置是否允许访问私有字段

```
func SetAllAccess(allAccess bool)
```

#### RegisterBean

注册单例 Bean，不指定名称，重复注册会 panic。

```
func RegisterBean(bean interface{}) *SpringCore.BeanDefinition
```

#### RegisterNameBean

注册单例 Bean，需指定名称，重复注册会 panic。

```
func RegisterNameBean(name string, bean interface{}) *SpringCore.BeanDefinition
```

#### RegisterBeanFn

注册单例构造函数 Bean，不指定名称，重复注册会 panic。

```
func RegisterBeanFn(fn interface{}, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterNameBeanFn

注册单例构造函数 Bean，需指定名称，重复注册会 panic。

```
func RegisterNameBeanFn(name string, fn interface{}, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterMethodBean

注册成员方法单例 Bean，不指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。

```
func RegisterMethodBean(selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterNameMethodBean

注册成员方法单例 Bean，需指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。

```
func RegisterNameMethodBean(name string, selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterMethodBeanFn

@Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。

```
func RegisterMethodBeanFn(method interface{}, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterNameMethodBeanFn

@Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。

```
func RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *SpringCore.BeanDefinition
```


#### RegisterBeanDefinition

注册 BeanDefinition 对象，如果需要 Name 请在调用之前准备好。

```
func RegisterBeanDefinition(bd *SpringCore.BeanDefinition) *SpringCore.BeanDefinition
```

#### WireBean

对外部的 Bean 进行依赖注入和属性绑定

```
func WireBean(bean interface{})
```

#### GetBean

获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。

```
func GetBean(i interface{}, selector ...SpringCore.BeanSelector) bool
```

#### FindBean

查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。

```
func FindBean(selector SpringCore.BeanSelector) (*SpringCore.BeanDefinition, bool)
```

#### CollectBeans

收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据selectors 列表的顺序对收集结果进行排序。

```
func CollectBeans(i interface{}, selectors ...SpringCore.BeanSelector) bool
```

#### GetBeanDefinitions

获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!

```
func GetBeanDefinitions() []*SpringCore.BeanDefinition
```

#### GetProperty

返回 keys 中第一个存在的属性值，属性名称统一转成小写。

```
func GetProperty(keys ...string) interface{}
```

#### GetBoolProperty

返回 keys 中第一个存在的布尔型属性值，属性名称统一转成小写。

```
func GetBoolProperty(keys ...string) bool
```

#### GetIntProperty

返回 keys 中第一个存在的有符号整型属性值，属性名称统一转成小写。

```
func GetIntProperty(keys ...string) int64
```

#### GetUintProperty

返回 keys 中第一个存在的无符号整型属性值，属性名称统一转成小写。

```
func GetUintProperty(keys ...string) uint64
```

#### GetFloatProperty

返回 keys 中第一个存在的浮点型属性值，属性名称统一转成小写。

```
func GetFloatProperty(keys ...string) float64
```

#### GetStringProperty

返回 keys 中第一个存在的字符串型属性值，属性名称统一转成小写。

```
func GetStringProperty(keys ...string) string
```

#### GetDurationProperty

返回 keys 中第一个存在的 Duration 类型属性值，属性名称统一转成小写。

```
func GetDurationProperty(keys ...string) time.Duration
```

#### GetTimeProperty

返回 keys 中第一个存在的 Time 类型的属性值，属性名称统一转成小写。

```
func GetTimeProperty(keys ...string) time.Time
```

#### GetDefaultProperty

返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。

```
func GetDefaultProperty(key string, def interface{}) (interface{}, bool)
```

#### SetProperty

设置属性值，属性名称统一转成小写。

```
func SetProperty(key string, value interface{})
```

#### GetPrefixProperties

返回指定前缀的属性值集合，属性名称统一转成小写。

```
func GetPrefixProperties(prefix string) map[string]interface{}
```

#### GetProperties

返回所有的属性值，属性名称统一转成小写。

```
func GetProperties() map[string]interface{}
```

#### BindProperty

根据类型获取属性值，属性名称统一转成小写。

```
func BindProperty(key string, i interface{})
```

#### BindPropertyIf

根据类型获取属性值，属性名称统一转成小写。

```
func BindPropertyIf(key string, i interface{}, allAccess bool)
```

#### Run

根据条件判断是否立即执行一个一次性的任务

```
func Run(fn interface{}, tags ...string) *SpringCore.Runner
```

#### RunNow

立即执行一个一次性的任务

```
func RunNow(fn interface{}, tags ...string) error
```

#### Config

注册一个配置函数

```
func Config(fn interface{}, tags ...string) *SpringCore.Configer
```

#### ConfigWithName

注册一个配置函数，名称的作用是对 Config 进行排重和排顺序。

```
func ConfigWithName(name string, fn interface{}, tags ...string) *SpringCore.Configer
```

#### Go

安全地启动一个 goroutine

```
func Go(fn GoFuncWithContext)
```

#### RegisterFilter

注册 Web Filter 对象 Bean，如果需要 Name 请在调用之前准备好。

```
func RegisterFilter(bd *SpringCore.BeanDefinition) *SpringCore.BeanDefinition
```