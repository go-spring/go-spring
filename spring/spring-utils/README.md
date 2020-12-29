# spring-utils

提供一些针对单元测试、原生类型、集合、网络、错误、反射、同步等方面的工具函数。

- [Assert](#assert)
    - [AssertEqual](#assertequal)
    - [AssertPanic](#assertpanic)
    - [AssertMatches](#assertmatches)
- [Primitives](#primitives)
    - [SafeCloseChan](#safeclosechan)
    - [ToJson](#tojson)
    - [CopyBeanUseJson](#copybeanusejson)
- [Collection](#collection)
    - [ContainsInt](#containsint)
    - [ContainsString](#containsstring)
    - [NewList](#newlist)
    - [FindInList](#findinlist)
- [Encoding](#encoding)
    - [MD5](#md5)
    - [BASE64](#base64)
- [Error](#error)
    - [WithCause](#withcause)
    - [Cause](#cause)
    - [ErrorToString](#errortostring)
    - [ErrorWithFileLine](#errorwithfileline)
- [Panic](#panic)
    - [PanicCond](#paniccond)
    - [Panic](#panic-1)
    - [Panicf](#panicf)
- [Reflect](#reflect)
    - [PatchValue](#patchvalue)
    - [Indirect](#indirect)
    - [FileLine](#fileline)
    - [IsNil](#isnil)
- [网络](#网络)
    - [LocalIPv4](#localipv4)
- [时间](#时间)
    - [CurrentMilliSeconds](#currentmilliseconds)
    - [MilliSeconds](#milliseconds)
- [Sync](#sync)
    - [WaitGroup](#waitgroup)

### Assert

提供一系列 Assert 函数用于编写单元测试。

#### AssertEqual

asserts that expect and got are equal as defined by `reflect.DeepEqual`.

    func AssertEqual(t *testing.T, expect interface{}, got interface{})

#### AssertPanic

asserts that function `fn()` would panic. It fails if the panic message does not match the regular expression in `expr`.

    func AssertPanic(t *testing.T, fn func(), expr string) 

#### AssertMatches

asserts that a got value matches a given regular expression.

    func AssertMatches(t *testing.T, expr string, got string) 

### Primitives

提供一些针对原生类型的工具函数。

#### SafeCloseChan

安全地关闭一个管道。

    func SafeCloseChan(ch chan struct{}) 

#### ToJson

将对象序列化为 Json 字符串，错误信息以结果返回。

    func ToJson(i interface{}) string

#### CopyBeanUseJson

使用 Json 序列化框架进行拷贝，支持匿名字段，支持类型转换。

    func CopyBeanUseJson(src interface{}, dest interface{}) error

### Collection

提供一些针对集合的工具函数。

#### ContainsInt

在一个 int 数组中进行查找，找不到返回 -1。

    func ContainsInt(array []int, val int) int

#### ContainsString

在一个 string 数组中进行查找，找不到返回 -1。

    func ContainsString(array []string, val string) int 

#### NewList

使用输入的元素创建列表。

    func NewList(v ...interface{}) *list.List 

#### FindInList

在列表中查询指定元素，存在则返回列表项指针，不存在返回 nil。

    func FindInList(v interface{}, l *list.List) (*list.Element, bool) 

### Encoding

提供一些针对数据编码的工具函数。

#### MD5

获取 MD5 计算后的字符串.

    func MD5(str string) string 

#### BASE64

返回 BASE64 加密后的字符串。

    func BASE64(str string) string 

### Error

提供一些针对 error 的工具函数。

#### WithCause

封装一个异常源。

    func WithCause(r interface{}) error

#### Cause

获取异常源。

    func Cause(err error) interface{} 

#### ErrorToString

获取 error 的字符串。

    func ErrorToString(err error) string 

#### ErrorWithFileLine

返回 error 发生的文件行信息。

    func ErrorWithFileLine(err error, skip ...int) error 

### Panic

提供一些针对 panic 的工具函数。

#### PanicCond

封装触发 panic 的内容和条件。

    type PanicCond struct {}

    // NewPanicCond PanicCond 的构造函数。
    func NewPanicCond(fn func() interface{}) *PanicCond {}

    // When 满足给定条件时抛出一个 panic。
    func (p *PanicCond) When(isPanic bool) {}

#### Panic

抛出一个异常值。

    func Panic(err error) *PanicCond 

#### Panicf

抛出一段需要格式化的错误字符串。

    func Panicf(format string, a ...interface{}) *PanicCond

### Reflect

提供一些针对反射的工具函数。

#### PatchValue

allAccess 为 true 时开放 v 的私有字段，返回修改后的副本。

    func PatchValue(v reflect.Value, allAccess bool) reflect.Value 

#### Indirect

解除 Type 的指针。

    func Indirect(t reflect.Type) reflect.Type

#### FileLine

获取函数所在文件、行数以及函数名。

    func FileLine(fn interface{}) (file string, line int, fnName string) 

#### IsNil

返回 reflect.Value 的值是否为 nil，比原生方法更安全。

    func IsNil(v reflect.Value) bool 

### 网络

提供一些针对网络的工具函数。

#### LocalIPv4

获取本机的 IPv4 地址。

    func LocalIPv4() string 

### 时间

提供一些针对时间的工具函数。

#### CurrentMilliSeconds

返回当前的毫秒时间。

    func CurrentMilliSeconds() int64

#### MilliSeconds

返回 Duration 返回对应的毫秒时长。

    func MilliSeconds(d time.Duration) int64 

### Sync

#### WaitGroup

封装 sync.WaitGroup，提供更简单的 API。

    type WaitGroup struct {
      wg sync.WaitGroup
    }
    
    // Add 添加一个任务，任务在 Goroutine 中执行
    func (wg *WaitGroup) Add(fn func()) 
    
    // Wait 等待所有任务执行完成
    func (wg *WaitGroup) Wait() 