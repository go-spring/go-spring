# spring-boot

提供基于 IoC 容器的启动框架。

- [Application](#application)
  - [启动&停止](#启动停止)
    - [RunApplication](#runapplication)
    - [Exit](#exit)
  - [应用配置](#应用配置)
    - [ExpectSysProperties](#expectsysproperties)
    - [AfterPrepare](#afterprepare)
  - [Banner](#banner)
    - [SetBannerMode](#setbannermode)
    - [SetBanner](#setbanner)
  - [IoC 容器](#ioc-容器)
    - [GetProfile](#getprofile)
    - [SetProfile](#setprofile)
    - [AllAccess](#allaccess)
    - [SetAllAccess](#setallaccess)
    - [RegisterBean](#registerbean)
    - [RegisterNameBean](#registernamebean)
    - [RegisterBeanFn](#registerbeanfn)
    - [RegisterNameBeanFn](#registernamebeanfn)
    - [RegisterMethodBean](#registermethodbean)
    - [RegisterNameMethodBean](#registernamemethodbean)
    - [RegisterMethodBeanFn](#registermethodbeanfn)
    - [RegisterNameMethodBeanFn](#registernamemethodbeanfn)
    - [RegisterBeanDefinition](#registerbeandefinition)
    - [WireBean](#wirebean)
    - [GetBean](#getbean)
    - [FindBean](#findbean)
    - [CollectBeans](#collectbeans)
    - [GetBeanDefinitions](#getbeandefinitions)
    - [GetProperty](#getproperty)
    - [GetBoolProperty](#getboolproperty)
    - [GetIntProperty](#getintproperty)
    - [GetUintProperty](#getuintproperty)
    - [GetFloatProperty](#getfloatproperty)
    - [GetStringProperty](#getstringproperty)
    - [GetDurationProperty](#getdurationproperty)
    - [GetTimeProperty](#gettimeproperty)
    - [GetDefaultProperty](#getdefaultproperty)
    - [SetProperty](#setproperty)
    - [GetPrefixProperties](#getprefixproperties)
    - [GetProperties](#getproperties)
    - [BindProperty](#bindproperty)
    - [BindPropertyIf](#bindpropertyif)
    - [Run](#run)
    - [RunNow](#runnow)
    - [Config](#config)
    - [ConfigWithName](#configwithname)
    - [Go](#go)
- [配置扩展](#配置扩展)
  - [PropertySource](#propertysource)
  - [RegisterPropertySource](#registerpropertysource)
  - [ConfigReader](#configreader)
    - [RegisterConfigReader](#registerconfigreader)
    - [RegisterFileConfigReader](#registerfileconfigreader)
- [CommandLineRunner](#commandlinerunner)
- [ApplicationEvent](#applicationevent)
- [Web](#web)
  - [Handler](#handler)
    - [Route](#route)
    - [HandleRequest](#handlerequest)
    - [RequestMapping](#requestmapping)
    - [RequestBinding](#requestbinding)
    - [HandleGet](#handleget)
    - [GetMapping](#getmapping)
    - [GetBinding](#getbinding)
    - [HandlePost](#handlepost)
    - [PostMapping](#postmapping)
    - [PostBinding](#postbinding)
    - [HandlePut](#handleput)
    - [PutMapping](#putmapping)
    - [PutBinding](#putbinding)
    - [HandleDelete](#handledelete)
    - [DeleteMapping](#deletemapping)
    - [DeleteBinding](#deletebinding)
  - [Filter](#filter)
    - [Filter](#filter-1)
    - [FilterBean](#filterbean)
  - [ConditionalWebFilter](#conditionalwebfilter)
    - [Or](#or)
    - [And](#and)
    - [ConditionOn](#conditionon)
    - [ConditionNot](#conditionnot)
    - [ConditionOnProperty](#conditiononproperty)
    - [ConditionOnMissingProperty](#conditiononmissingproperty)
    - [ConditionOnPropertyValue](#conditiononpropertyvalue)
    - [ConditionOnOptionalPropertyValue](#conditiononoptionalpropertyvalue)
    - [ConditionOnBean](#conditiononbean)
    - [ConditionOnMissingBean](#conditiononmissingbean)
    - [ConditionOnExpression](#conditiononexpression)
    - [ConditionOnMatches](#conditiononmatches)
    - [ConditionOnProfile](#conditiononprofile)
  - [RegisterFilter](#registerfilter)
- [Message](#message)
  - [BindConsumer](#bindconsumer)
  - [ConditionalBindConsumer](#conditionalbindconsumer)
    - [Or](#or-1)
    - [And](#and-1)
    - [ConditionOn](#conditionon-1)
    - [ConditionNot](#conditionnot-1)
    - [ConditionOnProperty](#conditiononproperty-1)
    - [ConditionOnMissingProperty](#conditiononmissingproperty-1)
    - [ConditionOnPropertyValue](#conditiononpropertyvalue-1)
    - [ConditionOnOptionalPropertyValue](#conditiononoptionalpropertyvalue-1)
    - [ConditionOnBean](#conditiononbean-1)
    - [ConditionOnMissingBean](#conditiononmissingbean-1)
    - [ConditionOnExpression](#conditiononexpression-1)
    - [ConditionOnMatches](#conditiononmatches-1)
    - [ConditionOnProfile](#conditiononprofile-1)
- [gRPC](#grpc)
  - [Server](#server)
    - [RegisterGRpcServer](#registergrpcserver)
  - [Client](#client)
    - [RegisterGRpcClient](#registergrpcclient)

## Application

### 启动&停止

#### RunApplication

快速启动 SpringBoot 应用。

```
func RunApplication(configLocation ...string)
```

#### Exit

退出 SpringBoot 应用。

```
func Exit()
```

### 应用配置

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

### Banner

#### SetBannerMode

设置 Banner 的显式模式

```
func SetBannerMode(mode BannerMode)
```

#### SetBanner

设置自定义 Banner 字符串

```
func SetBanner(banner string)
```

### IoC 容器

#### GetProfile

返回运行环境。

```
func GetProfile() string
```

#### SetProfile

设置运行环境。

```
func SetProfile(profile string)
```

#### AllAccess

返回是否允许访问私有字段。

```
func AllAccess() bool
```

#### SetAllAccess

设置是否允许访问私有字段。

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

注册成员方法单例 Bean，不指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有
receiver，两者类型也不好匹配。

```
func RegisterMethodBean(selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition
```

#### RegisterNameMethodBean

注册成员方法单例 Bean，需指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有
receiver，两者类型也不好匹配。

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
func RegisterBeanDefinition(bd *SpringCore.BeanDefinition)
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

收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组
Bean (是指数组的元素符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且必须满足 selector
条件。另外，自动模式下不对收集结果进行排序，指定模式下根据selectors 列表的顺序对收集结果进行排序。

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

## 配置扩展

### PropertySource

属性源接口。

```
type PropertySource interface {

	// Scheme 返回属性源的标识
	Scheme() string

	// Load 加载属性文件，profile 配置文件剖面，fileLocation 和属性源相关。
	Load(fileLocation string, profile string) map[string]interface{}
}
```

### RegisterPropertySource

注册属性源

```
func RegisterPropertySource(ps PropertySource)
```

### ConfigReader

配置读取器接口。

```
type ConfigReader interface {
	FileExt() string // 文件扩展名
	ReadFile(filename string, out map[string]interface{})
	ReadBuffer(buffer []byte, out map[string]interface{})
}
```

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

## CommandLineRunner

命令行启动器接口。

```
type CommandLineRunner interface {
	Run(ctx ApplicationContext)
}
```

## ApplicationEvent

应用运行过程中的事件。

```
type ApplicationEvent interface {
	OnStartApplication(ctx ApplicationContext) // 应用启动的事件
	OnStopApplication(ctx ApplicationContext)  // 应用停止的事件
}
```

## Web

### Handler

#### Route

返回和 Mapping 绑定的路由分组。

    func Route(basePath string, filters ...SpringWeb.Filter) *Router

#### HandleRequest

注册任意 HTTP 方法处理函数。

    func HandleRequest(method uint32, path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping

#### RequestMapping

注册任意 HTTP 方法处理函数。

    func RequestMapping(method uint32, path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping 

#### RequestBinding

注册任意 HTTP 方法处理函数。

    func RequestBinding(method uint32, path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping 

#### HandleGet

注册 GET 方法处理函数。

    func HandleGet(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping 

#### GetMapping

注册 GET 方法处理函数。

    func GetMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping 

#### GetBinding

注册 GET 方法处理函数。

    func GetBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping 

#### HandlePost

注册 POST 方法处理函数。

    func HandlePost(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping 

#### PostMapping

注册 POST 方法处理函数。

    func PostMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping 

#### PostBinding

注册 POST 方法处理函数。

    func PostBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping 

#### HandlePut

注册 PUT 方法处理函数。

    func HandlePut(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping 

#### PutMapping

注册 PUT 方法处理函数。

    func PutMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping 

#### PutBinding

注册 PUT 方法处理函数。

    func PutBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping 

#### HandleDelete

注册 DELETE 方法处理函数。

    func HandleDelete(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping 

#### DeleteMapping

注册 DELETE 方法处理函数。

    func DeleteMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping 

#### DeleteBinding

注册 DELETE 方法处理函数。

    func DeleteBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping 

}

### Filter

#### Filter

封装一个 SpringWeb.Filter 对象。

    func Filter(filters ...SpringWeb.Filter) *ConditionalWebFilter 

#### FilterBean

封装一个 Bean 选择器。

    func FilterBean(selectors ...SpringCore.BeanSelector) *ConditionalWebFilter 

### ConditionalWebFilter

#### Or

c=a||b。

    func (f *ConditionalWebFilter) Or() *ConditionalWebFilter 

#### And

c=a&&b。

    func (f *ConditionalWebFilter) And() *ConditionalWebFilter 

#### ConditionOn

设置一个 Condition。

    func (f *ConditionalWebFilter) ConditionOn(cond SpringCore.Condition) *ConditionalWebFilter 

#### ConditionNot

设置一个取反的 Condition。

    func (f *ConditionalWebFilter) ConditionNot(cond SpringCore.Condition) *ConditionalWebFilter 

#### ConditionOnProperty

设置一个 PropertyCondition。

    func (f *ConditionalWebFilter) ConditionOnProperty(name string) *ConditionalWebFilter 

#### ConditionOnMissingProperty

设置一个 MissingPropertyCondition。

    func (f *ConditionalWebFilter) ConditionOnMissingProperty(name string) *ConditionalWebFilter 

#### ConditionOnPropertyValue

设置一个 PropertyValueCondition。

    func (f *ConditionalWebFilter) ConditionOnPropertyValue(name string, havingValue interface{},
	options ...SpringCore.PropertyValueConditionOption) *ConditionalWebFilter 

#### ConditionOnOptionalPropertyValue

设置一个 PropertyValueCondition，当属性值不存在时默认条件成立。

    func (f *ConditionalWebFilter) ConditionOnOptionalPropertyValue(name string, havingValue interface{}) *ConditionalWebFilter 

#### ConditionOnBean

设置一个 BeanCondition。

    func (f *ConditionalWebFilter) ConditionOnBean(selector SpringCore.BeanSelector) *ConditionalWebFilter 

#### ConditionOnMissingBean

设置一个 MissingBeanCondition。

    func (f *ConditionalWebFilter) ConditionOnMissingBean(selector SpringCore.BeanSelector) *ConditionalWebFilter 

#### ConditionOnExpression

设置一个 ExpressionCondition。

    func (f *ConditionalWebFilter) ConditionOnExpression(expression string) *ConditionalWebFilter 

#### ConditionOnMatches

设置一个 FunctionCondition。

    func (f *ConditionalWebFilter) ConditionOnMatches(fn SpringCore.ConditionFunc) *ConditionalWebFilter 

#### ConditionOnProfile

设置一个 ProfileCondition。

    func (f *ConditionalWebFilter) ConditionOnProfile(profile string) *ConditionalWebFilter 

### RegisterFilter

注册 Web Filter 对象 Bean，如果需要 Name 请在调用之前准备好。

```
func RegisterFilter(bd *SpringCore.BeanDefinition) *SpringCore.BeanDefinition
```

## Message

### BindConsumer

注册 BIND 形式的消息消费者

```
func BindConsumer(topic string, fn interface{}) *ConditionalBindConsumer
```

### ConditionalBindConsumer

#### Or

c=a||b。

    func (c *ConditionalBindConsumer) Or() *ConditionalBindConsumer 

#### And

c=a&&b。

    func (c *ConditionalBindConsumer) And() *ConditionalBindConsumer 

#### ConditionOn

设置一个 Condition。

    func (c *ConditionalBindConsumer) ConditionOn(cond SpringCore.Condition) *ConditionalBindConsumer 

#### ConditionNot

设置一个取反的 Condition。

    func (c *ConditionalBindConsumer) ConditionNot(cond SpringCore.Condition) *ConditionalBindConsumer 

#### ConditionOnProperty

设置一个 PropertyCondition。

    func (c *ConditionalBindConsumer) ConditionOnProperty(name string) *ConditionalBindConsumer 

#### ConditionOnMissingProperty

设置一个 MissingPropertyCondition。

    func (c *ConditionalBindConsumer) ConditionOnMissingProperty(name string) *ConditionalBindConsumer 

#### ConditionOnPropertyValue

设置一个 PropertyValueCondition。

    func (c *ConditionalBindConsumer) ConditionOnPropertyValue(name string, havingValue interface{},
	options ...SpringCore.PropertyValueConditionOption) *ConditionalBindConsumer 

#### ConditionOnOptionalPropertyValue

设置一个 PropertyValueCondition，当属性值不存在时默认条件成立。

    func (c *ConditionalBindConsumer) ConditionOnOptionalPropertyValue(name string, havingValue interface{}) *ConditionalBindConsumer 

#### ConditionOnBean

设置一个 BeanCondition。

    func (c *ConditionalBindConsumer) ConditionOnBean(selector SpringCore.BeanSelector) *ConditionalBindConsumer 

#### ConditionOnMissingBean

设置一个 MissingBeanCondition。

    func (c *ConditionalBindConsumer) ConditionOnMissingBean(selector SpringCore.BeanSelector) *ConditionalBindConsumer 

#### ConditionOnExpression

设置一个 ExpressionCondition。

    func (c *ConditionalBindConsumer) ConditionOnExpression(expression string) *ConditionalBindConsumer 

#### ConditionOnMatches

设置一个 FunctionCondition。

    func (c *ConditionalBindConsumer) ConditionOnMatches(fn SpringCore.ConditionFunc) *ConditionalBindConsumer 

#### ConditionOnProfile

设置一个 ProfileCondition。

    func (c *ConditionalBindConsumer) ConditionOnProfile(profile string) *ConditionalBindConsumer 

## gRPC

### Server

#### RegisterGRpcServer

注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server
是服务具体提供者对象。

```
func RegisterGRpcServer(fn interface{}, serviceName string, server interface{}) *GRpcServer
```

### Client

#### RegisterGRpcClient

注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数

```
func RegisterGRpcClient(fn interface{}, endpoint string) *SpringCore.BeanDefinition
```