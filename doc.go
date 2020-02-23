/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package GoSpring

/**************************************************************

1. 什么是值类型和引用类型？

值类型保存的是数据本身，引用类型保存的则是数据的指针(或者叫引用、地址都行)。

像 int、bool 等基础类型都是值类型，字符串也是值类型，结构体也是值类型。

假如对 int 等类型重定义并加入成员方法之后，新类型也是值类型吗？是的。换句话说，
判断一个类型是值类型还是引用类型依据是它的 kind。

除了指针是引用类型之外，数组、集合、通道等也都是引用类型，这些类型都不直接保存
数据，而是保存数据的地址。

由此可见，多个引用类型的对象是可以共享同一个数据的，而 Bean 指的就是这些可以
被共享的数据。

2. Bean 的三种定义形式

根据定义形式的不同可以将 Bean 分为三种，分别是对象 Bean、构造函数 Bean 和成
员方法 Bean，后两者又称为函数 Bean。

对象 Bean 保存的是 Bean 的原始数据，函数 Bean 保存的则是创建 Bean 的函数，
Go-Spring 会在合适的时机调用这些函数创建 Bean 的数据。

顾名思义，构造函数 Bean 保存的是可以创建 Bean 的构造函数(或者叫工厂函数)，而
成员方法 Bean 保存的则是可以创建 Bean 的成员方法。这两者的不同之处在于成员方
法 Bean 必须和一个与成员方法的接收者类型相同的 Bean 进行绑定。

此外，函数 Bean 的函数不仅支持可变参数，还支持 Option 模式，这在复用现有第三
方代码时会非常方便。

函数 Bean 的函数是不是必须返回引用类型呢？不是的。Go-Spring 在检测到函数的返
回值是值类型的时候会自动将其转换成引用类型。

函数 Bean 的函数可以返回接口类型吗？是的。而且 Go-Spring 推荐尽可能返回接口类
型，这不仅在代码层面有好处，在 Bean 注册和注入的时候也更方便。

另外，为了简称大部分情况下函数 Bean 指的是构造函数 Bean，而成员方法 Bean 则简
称为方法 Bean。这一点需要读者在后续章节注意分辨。

3. 什么是 IoC 容器和 Bean 注入？

举个例子，假设有一个结构体 Controller，包含了很多叫 Service 的字段，

type Controller struct{
  userService UserService
  roleService RoleService
  deptService DeptService
  ...
}

现在想给 Controller 的这些 Service 字段赋值，传统方式是通过构造函数参数传入或
者直接对字段进行赋值。但无论哪一种方式，当 Service 字段变多或者变少之后都会导致修
改很多的代码，那么有没有办法可以减少这种修改呢？

试着将这些 Service 的实例想象成服务提供方，将 Controller 的这些 Service 字段
想象成服务消费方，这样只需要服务提供方说明自己的能力，服务消费方说明自己的需求，就可
以设计一个服务平台来匹配这种供需关系。

如果这种方案是可行的，稍微想象一下就能感受到这种方式带来的新力量！幸运的是，Go 语言
提供了这种可能，通过反射机制很容易就能实现这种新的方式。

在这种方式下，服务平台就是 IoC 容器，服务消费的过程就是 Bean 注入。而 Go-Spring
就是 Go 语言中 IoC 容器的一个非常优秀的实现，后续章节会一点一点地揭开它的面纱。

4. 初识 SpringContext

SpringContext 是一个功能完善的 IoC 容器，是 Go-Spring 绝对的核心，其主要功能如下：

RegisterBean 和 RegisterNameBean 注册对象 Bean；
RegisterBeanFn 和 RegisterNameBeanFn 注册构造函数 Bean；
RegisterMethodBean 和 RegisterNameMethodBean 注册成员方法 Bean。

WireBean 对一个外部 Bean 执行注入过程；
AutoWireBeans 对已注册的 Bean 执行注入过程。

GetBean 和 GetBeanByName 获取一个 Bean，并确保已完成注入过程；
FindBean 和 FindBeanByName 查找一个 Bean，但不保证已完成注入过程；
CollectBeans 收集所有符合条件的 Bean，甚至包括符合条件的数组项，并确保均已完成注入过程。

Close 关闭 IoC 容器，并向已注册的 Bean 发送 destroy 消息，用于执行资源清理等工作。

实现 context.Context 接口，用于控制 goroutine 的生命周期。
实现 Properties 接口，用于实现一系列和属性相关的操作，后面会有专门的章节详细讲解。











2. 类型全限定名

Go 语言允许不同目录下存在相同名称的包，仅靠类型名是没有办法保证任何情
况下都能区分两个类型的，还需要再加上包路径，而"包路径+路径名"就是类型
全限定名。这个可能是 Go-Spring 独创的概念。

内置数据类型(如整数、浮点数、布尔、通道、Map等)的类型全限定名就是类型
名，指针和数组的类型全限定名是构成指针和数组的基础元素的类型全限定名。

原始类型: 全限定名
"int": "int"
"*int": "int"
"[]int": "int"
"*[]int": "int"
"map[int]int": "map[int]int"
"chan struct {}": "chan struct {}"
"func(int, int, int)": "func(int, int, int)"
"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"
"pkg.SamePkg": "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg"
"*pkg.SamePkg": "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg"
"[]pkg.SamePkg": "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg"
"*[]pkg.SamePkg": "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg"

3. Bean 类型

Go-Spring 有两种 Bean: 对象 Bean 和函数 Bean。对象 Bean 是指由原始对
象定义的 Bean，函数 Bean 是指由函数返回值定义的 Bean。函数 Bean 又可以
分为构造函数 Bean 和成员方法 Bean 两种。

函数 Bean 要求函数的返回值必须是 1 个或 2 个，当返回值是 2 个时第 2个返
回值必须是 error 类型。如果函数的返回值是值类型，Go-Spring 在存储返回值
时会自动转换成指针类型。函数 Bean 对函数的参数没有要求，而且支持不定参数。

因为 Go 语言不支持函数重载，所以 Option 模式被广泛使用，而 Go-Spring 也
支持由这种模式定义的函数 Bean。

4. Bean 生命周期

5. 属性配置文件

当前支持三种类型的配置文件: properties 文件、yaml 文件和 toml 文件。

6. 属性转换器

支持 time.Duration 和 time.Time。














当使用对象注册时，无论是否转成 Interface 都能获取到对象的真实类型，
当使用构造函数注册时，如果返回的是非引用类型会强制转成对应的引用类型，
如果返回的是 Interface 那么这种情况下会使用 Interface 的类型注册。

哪些类型可以成为 Bean 的接收者？除了使用 Bean 的真实类型去接收，还可
以使用 Bean 实现的 Interface 去接收，而且推荐用 Interface 去接收。

***************************************************************/
