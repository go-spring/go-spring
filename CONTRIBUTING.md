# Contribution Guideline

Thanks for considering to contribute this project. All issues and pull requests are highly appreciated.

## Pull Requests

Before sending pull request to this project, please read and follow guidelines below.

1. Branch: We only accept pull request on `master` branch.
2. Coding style: Follow the coding style used in `go-spring`.
3. Commit message: Use English or Chinese and be aware of your spell.
4. Test: Make sure to test your code.

NOTE: We assume all your contribution can be licensed under the [Apache License 2.0](https://github.com/go-spring/go-spring/blob/master/LICENSE).

## Issues

We love clearly described issues. :)

Following information can help us to resolve the issue faster.

* Version.
* Logs.
* Screenshots.
* Steps to reproduce the issue.

## 命名规则

明确且统一的命名规则有助于帮助我们形成一致的思考和设计模式，经过长期实践，Go-Spring 归纳出了几条颇为有益的命名规则，如下：

* package 一般使用名词或者动词，不推荐使用形容词。
* interface 一般使用名词或者形容词，动词短语也可。习惯上以 able、ible、er 等结尾。
* struct 一般使用名词或者动词短语。
* function 如果只返回 bool 值则以 is、has 等打头，否则必须使用动词打头。

### 常用变量名

* 构造函数的变量名和结构体的字段名保持一致。
* arg.Arg 一般情况下命名为 a 或者 arg。
* cond.Condition 一般情况下命名为 c 或者 cond。
* function 一般情况下命名为 f 或者 fn。
* 返回结果一帮情况下命名为 result 或者 ret。
* node 一般命名为 n。
* element 一般命名为 e。

## 编程规约

* 慎用嵌套(继承)，避免暴露不必要的方法。
* 限制每行长度最大不超过 100 个字符。
* 放心使用选项模式。
* 不对外直接暴露指针类型，使用值或者接口。
* 包名不能和 Golang 标准库重名。
* 注释里面的 bean 都是小写格式。
* 函数内部调用的函数一般放在它的上方并且靠近它。

## 注释

* 不要在注释上浪费太多文字，不要详细阐述你的思考，写清楚结论即可。
* 具有返回值的函数注释应该以 return 开头。

## 优秀经验

* 不用尽早抽象接口。
* 在使用的地方定义接口，而不是实现的地方。
* 多数情况下不需要新增错误类型，只有深层嵌套的场景才需要。