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
