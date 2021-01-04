# spring-core

实现了一个功能完善的运行时 IoC 容器。

- [SpringContext](#springcontext)
  - [属性操作](#属性操作)
    - [LoadProperties](#loadproperties)
    - [ReadProperties](#readproperties)
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
    - [GetGroupedProperties](#getgroupedproperties)
    - [GetProperties](#getproperties)
    - [BindProperty](#bindproperty)
    - [BindPropertyIf](#bindpropertyif)
  - [容器配置](#容器配置)
    - [Context](#context)
    - [GetProfile](#getprofile)
    - [SetProfile](#setprofile)
    - [AllAccess](#allaccess)
    - [SetAllAccess](#setallaccess)
  - [Bean 注册](#bean-注册)
    - [RegisterBean](#registerbean)
    - [RegisterNameBean](#registernamebean)
    - [RegisterBeanFn](#registerbeanfn)
    - [RegisterNameBeanFn](#registernamebeanfn)
    - [RegisterMethodBean](#registermethodbean)
    - [RegisterNameMethodBean](#registernamemethodbean)
    - [RegisterMethodBeanFn](#registermethodbeanfn)
    - [RegisterNameMethodBeanFn](#registernamemethodbeanfn)
    - [RegisterBeanDefinition](#registerbeandefinition)
  - [依赖注入](#依赖注入)
    - [AutoWireBeans](#autowirebeans)
    - [WireBean](#wirebean)
  - [获取 Bean](#获取-bean)
    - [GetBean](#getbean)
    - [FindBean](#findbean)
    - [CollectBeans](#collectbeans)
    - [GetBeanDefinitions](#getbeandefinitions)
  - [任务配置](#任务配置)
    - [Run](#run)
    - [RunNow](#runnow)
    - [Config](#config)
    - [ConfigWithName](#configwithname)
  - [容器销毁](#容器销毁)
    - [Close](#close)
  - [其他功能](#其他功能)
    - [SafeGoroutine](#safegoroutine)
- [Condition](#condition)
- [BeanDefinition](#beandefinition)
    - [ObjectBean](#objectbean)
    - [ConstructorBean](#constructorbean)
    - [MethodBean](#methodbean)
    - [Bean](#bean)
    - [Type](#type)
    - [Value](#value)
    - [TypeName](#typename)
    - [Name](#name)
    - [BeanId](#beanid)
    - [FileLine](#fileline)
    - [Description](#description)
    - [WithName](#withname)
    - [Options](#options)
    - [DependsOn](#dependson)
    - [Primary](#primary)
    - [Init](#init)
    - [Destroy](#destroy)
    - [Export](#export)

## SpringContext

SpringContext 定义了 IoC 容器接口。它的工作过程可以分为三个大的阶段：注册 Bean 列表、加载属性配置文件、自动绑定。其中自动绑定又分为两个小阶段：解析（决议）和绑定。

一条需要谨记的注册规则是: `AutoWireBeans` 调用后就不能再注册新的 Bean 了，这样做是因为实现起来更简单而且性能更高。

### 属性操作

#### LoadProperties

加载属性配置文件，支持 properties、yaml 和 toml 三种文件格式。

```
func LoadProperties(filename string)
```

#### ReadProperties

读取属性配置文件，支持 properties、yaml 和 toml 三种文件格式。

```
func ReadProperties(reader io.Reader, configType string)
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

#### GetGroupedProperties

返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。

```
func GetGroupedProperties(prefix string) map[string]map[string]interface{}
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

### 容器配置

#### Context

返回上下文接口

```
func Context() context.Context
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

### Bean 注册

#### RegisterBean

注册单例 Bean，不指定名称，重复注册会 panic。

```
func RegisterBean(bean interface{}) *BeanDefinition
```

#### RegisterNameBean

注册单例 Bean，需指定名称，重复注册会 panic。

```
func RegisterNameBean(name string, bean interface{}) *BeanDefinition
```

#### RegisterBeanFn

注册单例构造函数 Bean，不指定名称，重复注册会 panic。

```
func RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition
```

#### RegisterNameBeanFn

注册单例构造函数 Bean，需指定名称，重复注册会 panic。

```
func RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition
```

#### RegisterMethodBean

注册成员方法单例 Bean，不指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。

```
func RegisterMethodBean(selector BeanSelector, method string, tags ...string) *BeanDefinition
```

#### RegisterNameMethodBean

RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。

```
func RegisterNameMethodBean(name string, selector BeanSelector, method string, tags ...string) *BeanDefinition
```

#### RegisterMethodBeanFn

注册成员方法单例 Bean，不指定名称，重复注册会 panic。method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。

```
func RegisterMethodBeanFn(method interface{}, tags ...string) *BeanDefinition
```

#### RegisterNameMethodBeanFn

注册成员方法单例 Bean，需指定名称，重复注册会 panic。method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。

```
func RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *BeanDefinition
```

#### RegisterBeanDefinition

注册 BeanDefinition 对象。

```
func RegisterBeanDefinition(bd *BeanDefinition)
```

### 依赖注入

#### AutoWireBeans

对所有 Bean 进行依赖注入和属性绑定

```
func AutoWireBeans()
```

#### WireBean

对外部的 Bean 进行依赖注入和属性绑定

```
func WireBean(i interface{})
```

### 获取 Bean

#### GetBean

获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。

```
func GetBean(i interface{}, selector ...BeanSelector) bool
```

#### FindBean

查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。

```
func FindBean(selector BeanSelector) (*BeanDefinition, bool)
```

#### CollectBeans

收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据selectors 列表的顺序对收集结果进行排序。

```
func CollectBeans(i interface{}, selectors ...BeanSelector) bool
```

#### GetBeanDefinitions

获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!

```
func GetBeanDefinitions() []*BeanDefinition
```

### 任务配置

#### Run

根据条件判断是否立即执行一个一次性的任务

```
func Run(fn interface{}, tags ...string) *Runner
```

#### RunNow

立即执行一个一次性的任务

```
func RunNow(fn interface{}, tags ...string) error
```

#### Config

注册一个配置函数

```
func Config(fn interface{}, tags ...string) *Configer
```

#### ConfigWithName

注册一个配置函数，名称的作用是对 Config 进行排重和排顺序。

```
func ConfigWithName(name string, fn interface{}, tags ...string) *Configer
```

### 容器销毁

#### Close

关闭容器上下文，用于通知 Bean 销毁等。该函数可以确保 Bean 的销毁顺序和注入顺序相反。

```
func Close(beforeDestroy ...func())
```

### 其他功能

#### SafeGoroutine

安全地启动一个 goroutine

```
func SafeGoroutine(fn GoFunc)
```

## Condition

定义一个判断条件。

`NewFunctionCondition` 基于 Matches 方法的 Condition 实现  
`NewNotCondition` 对 Condition 取反的 Condition 实现  
`NewPropertyCondition` 基于属性值存在的 Condition 实现  
`NewMissingPropertyCondition` 基于属性值不存在的 Condition 实现  
`NewPropertyValueCondition` 基于属性值匹配的 Condition 实现  
`NewBeanCondition` 基于 Bean 存在的 Condition 实现  
`NewMissingBeanCondition` 基于 Bean 不能存在的 Condition 实现  
`NewExpressionCondition` 基于表达式的 Condition 实现  
`NewProfileCondition` 基于运行环境匹配的 Condition 实现  
`NewConditions` 基于条件组的 Condition 实现  
`NewConditional` Condition 计算式  

- `Or` c=a||b
- `And` c=a&&b
- `OnCondition` 设置一个 Condition
- `OnConditionNot` 设置一个取反的 Condition
- `ConditionOnProperty` 返回设置了 propertyCondition 的 Conditional 对象
- `ConditionOnMissingProperty` 返回设置了 missingPropertyCondition 的 Conditional 对象
- `ConditionOnPropertyValue` 返回设置了 propertyValueCondition 的 Conditional 对象
- `ConditionOnOptionalPropertyValue` 返回属性值不存在时默认条件成立的 Conditional 对象
- `OnOptionalPropertyValue` 设置一个 propertyValueCondition，当属性值不存在时默认条件成立
- `ConditionOnBean` 返回设置了 beanCondition 的 Conditional 对象
- `ConditionOnMissingBean` 返回设置了 missingBeanCondition 的 Conditional 对象
- `ConditionOnExpression` 返回设置了 expressionCondition 的 Conditional 对象
- `ConditionOnMatches` 返回设置了 functionCondition 的 Conditional 对象
- `ConditionOnProfile` 返回设置了 profileCondition 的 Conditional 对象

## BeanDefinition

#### ObjectBean

将 Bean 转换为 BeanDefinition 对象

```
func ObjectBean(i interface{}) *BeanDefinition
```

#### ConstructorBean

将构造函数转换为 BeanDefinition 对象

```
func ConstructorBean(fn interface{}, tags ...string) *BeanDefinition
```

#### MethodBean

将成员方法转换为 BeanDefinition 对象

```
func MethodBean(selector BeanSelector, method string, tags ...string) *BeanDefinition
```

#### Bean

返回 Bean 的源

```
func (d *BeanDefinition) Bean() interface{}
```

#### Type

返回 Bean 的类型

```
func (d *BeanDefinition) Type() reflect.Type
```

#### Value

返回 Bean 的值

```
func (d *BeanDefinition) Value() reflect.Value
```

#### TypeName

返回 Bean 的原始类型的全限定名

```
func (d *BeanDefinition) TypeName() string
```

#### Name

返回 Bean 的名称

```
func (d *BeanDefinition) Name() string
```

#### BeanId

返回 Bean 的唯一 ID

```
func (d *BeanDefinition) BeanId() string
```

#### FileLine

返回 Bean 的注册点

```
func (d *BeanDefinition) FileLine() string
```

#### Description

返回 Bean 的详细描述

```
func (d *BeanDefinition) Description() string
```

#### WithName

设置 Bean 的名称

```
func (d *BeanDefinition) WithName(name string) *BeanDefinition
```

#### Options

设置 Option 模式函数的 Option 参数绑定

```
func (d *BeanDefinition) Options(options ...*optionArg) *BeanDefinition
```

#### DependsOn

设置 Bean 的间接依赖项

```
func (d *BeanDefinition) DependsOn(selectors ...BeanSelector) *BeanDefinition
```

#### Primary

设置 Bean 为主版本

```
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition
```

#### Init

设置 Bean 的初始化函数，tags 是初始化函数的一般参数绑定

```
func (d *BeanDefinition) Init(fn interface{}, tags ...string) *BeanDefinition
```

#### Destroy

设置 Bean 的销毁函数，tags 是销毁函数的一般参数绑定

```
func (d *BeanDefinition) Destroy(fn interface{}, tags ...string) *BeanDefinition
```

#### Export

显式指定 Bean 的导出接口

```
func (d *BeanDefinition) Export(exports ...TypeOrPtr) *BeanDefinition
```
