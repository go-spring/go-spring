<div>
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" height="*" alt="logo"/>
 <br/>
 <img src="https://img.shields.io/github/license/go-spring/go-spring" alt="license"/>
 <img src="https://img.shields.io/github/go-mod/go-version/go-spring/spring-boot" alt="go-version"/>
 <img src="https://img.shields.io/github/v/release/go-spring/go-spring?include_prereleases" alt="release"/>
</div>

Go-Spring 的愿景是让 Go 程序员也能用上如 Java Spring 那般威力强大的编程框架。

其特性如下：

1. 提供了完善的 IoC 容器，支持依赖注入、属性绑定；
2. 提供了强大的启动器框架，支持自动装配、开箱即用；
3. 提供了常见组件的抽象层，支持灵活地替换底层实现；

Go-Spring 当前使用 Go1.12 进行开发，使用 Go Modules 进行依赖管理。

### IoC 容器

Go-Spring 不仅实现了如 Java Spring 那般功能强大的 IoC 容器，还扩充了 Bean 的概念。在 Go 中，对象(即指针)、数组、Map、函数指针，这些都是 Bean，都可以放到 IoC 容器里。

| 常用的 Java Spring 注解				  | 对应的 Go-Spring 实现			|
| :-- 									| :-- 							|
| `@Value` 								| `value:"${}"` 				|
| `@Autowired` `@Qualifier` `@Required` | `autowire:"?"` 				|
| `@Configurable` 						| `WireBean()` 					|
| `@Profile` 							| `ConditionOnProfile()` 		|
| `@Primary` 							| `Primary()` 					|
| `@DependsOn` 							| `DependsOn()` 				|
| `@ConstructorBinding` 				| `RegisterBeanFn()` 			|
| `@ComponentScan` `@Indexed` 			| Package Import 				|
| `@Conditional` 						| `NewConditional()` 			|
| `@ConditionalOnExpression` 			| `NewExpressionCondition()` 	|
| `@ConditionalOnProperty` 				| `NewPropertyValueCondition()`	|
| `@ConditionalOnBean` 					| `NewBeanCondition()` 			|
| `@ConditionalOnMissingBean` 			| `NewMissingBeanCondition()`	|
| `@ConditionalOnClass` 				| Don't Need 					|
| `@ConditionalOnMissingClass` 			| Don't Need 					|
| `@Lookup` 							| —— 							|

### 属性绑定

Go-Spring 不仅支持对普通数据类型进行属性绑定，也支持对自定义值类型进行属性绑定，而且还支持对结构体属性的嵌套绑定。

```
type DB struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
	DB       string `value:"${db}"`
}

type DbConfig struct {
	DB []DB `value:"${db}"`
}
```

上面这段代码可以通过下面的配置进行绑定：

```
db:
  -
    username: root
    password: 123456
    url: 1.1.1.1
    port: 3306
    db: db1
  -
    username: root
    password: 123456
    url: 1.1.1.1
    port: 3306
    db: db2
```

### Boot 框架

Go-Spring 提供了一个功能强大的启动器框架，不仅实现了自动加载、开箱即用，而且可以非常容易的开发自己的启动器模块，使得代码不仅仅是库层面的复用。

### 快速示例

```
package main

import (
	"github.com/go-spring/spring-boot"
	_ "github.com/go-spring/starter-echo"
)

func init() {
	SpringBoot.RegisterBean(new(Echo)).Init(func(e *Echo) {
		SpringBoot.GetBinding("/", e.Call)
	})
}

type Echo struct {
	GoPath string `value:"${GOPATH}"`
}

func (e *Echo) Call() string {
	return e.GoPath
}

func main() {
	SpringBoot.RunApplication()
}
```

启动上面的程序，控制台输入 `curl http://localhost:8080/`， 可得到如下结果：
```
{"code":200,"msg":"SUCCESS","data":"/Users/didi/go"}
```

更多示例： https://github.com/go-spring/go-spring/tree/master/examples

### 项目成员

#### 发起者(负责人)

[lvan100 (LiangHuan)](https://github.com/lvan100)

#### 优秀贡献者

[@CoderPoet](https://github.com/CoderPoet) 、[@limpo1989](https://github.com/limpo1989) 

#### 特别鸣谢

[@shenqidebaozi](https://github.com/shenqidebaozi)

如何成为贡献者？提交有意义的 PR 或者需求，并被采纳。

### QQ 交流群

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="140" height="*" />
<br>QQ群号:721077608

### 微信公众号

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="140" height="*" />

### License

The Go-Spring is released under version 2.0 of the Apache License.