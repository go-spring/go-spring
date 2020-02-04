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

1. 值类型和引用类型

怎么区分值类型和引用类型？简单来讲，值类型是变量直接存储数据,而引用类
型是变量存储数据的引用。在 Go-Spring 框架内，值类型包括基本数据类型
(如整数、浮点数等)以及字符串和结构体类型，而引用类型则包括了指针、数
组、通道、函数、接口和 Map 类型。

为什么要区分值类型和引用类型？因为只有引用类型的数据才能成为 Bean。

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
















当使用对象注册时，无论是否转成 Interface 都能获取到对象的真实类型，
当使用构造函数注册时，如果返回的是非引用类型会强制转成对应的引用类型，
如果返回的是 Interface 那么这种情况下会使用 Interface 的类型注册。

哪些类型可以成为 Bean 的接收者？除了使用 Bean 的真实类型去接收，还可
以使用 Bean 实现的 Interface 去接收，而且推荐用 Interface 去接收。

***************************************************************/
