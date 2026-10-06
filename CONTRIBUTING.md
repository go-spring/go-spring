# Contributing to Go-Spring

First of all, thank you for your interest in and support of the Go-Spring project!
Before contributing, please read our [Contributor Code of Conduct](CODE_OF_CONDUCT.md).

We welcome all kinds of contributions, including reporting issues, improving documentation, fixing bugs, and developing
new features. Please follow the guidelines below to contribute.

## Table of Contents

- [Submitting Issues](#submitting-issues)
- [Submitting Pull Requests](#submitting-pull-requests)
- [Branch Naming Guidelines](#branch-naming-guidelines)
- [Local Development Environment](#local-development-environment)
- [Testing](#testing)
- [Coding Guidelines](#coding-guidelines)
- [Commit Message Format](#commit-message-format)
- [Contact Us](#contact-us)

## Submitting Issues

- Search existing issues before submitting to avoid duplicates.
- Provide clear reproduction steps, expected behavior, and actual results.
- Include error logs and environment information if applicable.
- Helpful information includes version, logs, screenshots, and steps to reproduce the issue.

## Submitting Pull Requests

1. **Fork the repository and create a new branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Maintain consistent coding style**

    * Follow Go’s official style guidelines (`gofmt`, `golint`, `go vet`).
    * Follow the coding style used in `go-spring`.
    * Recommended: [`golangci-lint`](https://github.com/golangci/golangci-lint) for local linting.

3. **Write tests**

    * All new features or bug fixes must include unit tests.
    * Use Go’s `testing` package; test files should be named `xxx_test.go`.
    * Example:

      ```go
      func TestAdd(t *testing.T) {
          result := Add(1, 2)
          if result != 3 {
              t.Errorf("expected 3, got %d", result)
          }
      }
      ```

4. **Update documentation**

    * If your changes affect usage or APIs, update README or code comments.

5. **Submit and create a Pull Request**

    * Clearly describe:

        * **What**: What changes are made
        * **Why**: Why the changes are needed
        * **How**: How it was implemented
        * **Testing**: How it was tested
    * Link related issues if applicable.

All contributions are assumed to be licensed under the [Apache License 2.0](LICENSE).

## Branch Naming Guidelines

* `feature/xxx` – New feature
* `fix/xxx` – Bug fix
* `doc/xxx` – Documentation updates
* `refactor/xxx` – Code refactoring

## Local Development Environment

* Recommended Go version: latest stable release (e.g., `go1.21+`)
* Use Go Modules for dependency management.
* Make sure all tests pass before submitting:

  ```bash
  go test ./...
  ```

## Testing

* Run `go test ./...` to ensure all tests pass.
* For examples or integration tests, provide instructions if needed.

## Build and Tooling

* **`go mod tidy` under `go.work` does not use the workspace replaces** — it pulls the published `spring` through GOPROXY and fails wholesale on an unpublished local module (404) or a lagging version. Workaround: `go mod tidy -e` (tolerate that error, still update `go.mod`/`go.sum`), then verify with `go build ./...` (which does use the `go.work` local tree). Do not read a tidy error as "the change is broken".
* **A module not in `go.work`** (`temp/`, ...) fails a plain `go build` with `directory prefix . does not contain modules listed in go.work`; build it with `GOWORK=off` (it points back to this repo through its own `replace`).
* **Build a member module from the repo root** with `go build -C <abs-module-dir> ./...` (the shell cwd resets; a `cd` may trigger a permission prompt).
* **`scripts/check-go-modules.sh` is not read-only** — it runs `go fix` + `modernize`, **rewriting sources in place**, which the automated commit sweep picks up. For read-only checks use `gofmt -l` / `go vet` / `go build` / `go test` / `bash scripts/check-config-namespace.sh` / `bash scripts/check-observability.sh`.
* **The contrib dubbo examples are verified end to end only by docker-gated smoke scripts** — one **per example** (`contrib/<example>/scripts/smoke-test.sh`, 23 of them); there is no repo-level `scripts/smoke-test.sh`. With no docker the script silently `exit 0`s, so a clean build/vet does **not** mean it runs.

## Examples

* Every example's `func init()` uses the same "chdir to the source directory" boilerplate (with the `workDir` print):

  ```go
  // init sets the working directory of the application to the directory
  // where this source file resides.
  // This ensures that any relative file operations are based on the source file location,
  // not the process launch path.
  func init() {
  	var execDir string
  	_, filename, _, ok := runtime.Caller(0)
  	if ok {
  		execDir = filepath.Dir(filename)
  	}
  	err := os.Chdir(execDir)
  	if err != nil {
  		panic(err)
  	}
  	workDir, err := os.Getwd()
  	if err != nil {
  		panic(err)
  	}
  	fmt.Println(workDir)
  }
  ```

  * Printing the working directory at startup is deliberate (it needs the `fmt` import) — do not delete it as noise.
  * When an example does something else in `init()` too (reading a script, setting an env var), do not replace the whole block — align only the boilerplate prefix. `temp/` is neither swept nor committed.
* `example/check.sh`'s four pitfalls must be copied from the existing new-starter skeleton: ① every example dir is named `example`, so `compose()` must pass an explicit `-p gs-<name>-example` (otherwise `down -v` tears down another's containers); ② `gs.Run` exits 0 on a startup failure, so after `go run . > smoke.out 2>&1` you must `grep -q "<success marker>" smoke.out`; ③ commands like `mqadmin` fail-successfully (return 0 on failure), so gate on the output text (`| grep -q success`); ④ in a foreground sandbox the `/dev/tcp` probe always fails — use an in-container probe (e.g. `docker exec <c> cqlsh -e "DESCRIBE CLUSTER"`) as the readiness gate. Skeleton = `-p` project name + marker gate + in-container readiness probe + watchdog.

## Coding Guidelines

### Naming Rules

Clear and consistent naming rules help us form consistent thinking and design patterns.

* package 一般使用名词或者动词，不推荐使用形容词。
* interface 一般使用名词或者形容词，动词短语也可。习惯上以 able、ible、er 等结尾。
* struct 一般使用名词或者动词短语。
* function 如果只返回 bool 值则尽量以 is、has 等打头，否则必须使用动词打头。

### Common Variable Names

* 构造函数的变量名和结构体的字段名保持一致。
* arg.Arg 一般情况下命名为 a 或者 arg。
* cond.Condition 一般情况下命名为 c 或者 cond。
* function 一般情况下命名为 f 或者 fn。
* 返回结果一般情况下命名为 result 或者 ret。
* node 一般命名为 n。
* element 一般命名为 e。

### Programming Rules

* 禁止导出全局变量。
* 错误分支优先处理，不要进行 err==nil 判断。
* 慎用嵌套(继承)，避免暴露不必要的方法。
* 限制每行长度最大不超过 100 个字符。
* 放心使用选项模式。
* 不对外直接暴露指针类型，使用值或者接口。
* 包名不能和 Golang 标准库重名。
* 注释里面的 bean 都是小写格式。
* 几乎所有的 panic 都应该打印其调用栈。
* 和包名同名的文件作为了解包的入口。
* 类型或函数的内容较短时也不能写在一行上，必须换行。
* 禁止程序在 init 阶段启动 goroutine。
* client 类型 starter 必须具有名字、排除同名同类型的 bean。
* 有共同抽象的 starter 实现使用相同的属性前缀，否则使用自身前缀。
* 在回放和测试模式下，尽量通过 panic 减少错误处理。
* 在录制模式下，通过打印日志的方式避免对主流程产生影响。
* 所有错误都需要打印错误发生位置的文件名和行号，保证错误排查的底线。
* boot 模块不提供全局读取和访问函数，推荐使用 ContextAware 注入。

### Comments

* 不要在注释上浪费太多文字，不要详细阐述你的思考，写清楚结论即可。
* 具有返回值的函数注释应该以 return 开头。
* 未导出函数尽量做到不使用注释也能知其意，而开放函数必须写注释。
* 代码中一些很少使用的场景必须写清楚其背景！

### Practices

* 不用尽早抽象接口。
* 异常判断应当尽早返回。
* 在使用的地方定义接口，而不是实现的地方。
* 多数情况下不需要新增错误类型，只有深层嵌套的场景才需要。
* 所有可变参数如果函数名不能提供有效信息都应该使用 Option 模式。
* 好的架构都是在改进的过程中逐渐浮现出来的。
* 并发程序难写在于很难想清楚所有出现并发的情况。
* 为大公司提供框架，为小公司提供实现。
* 方法的接收者尽量使用指针，避免不必要的指针到值的转换过程。
* 测试环境打开竞态检测功能。
* 不入流的功能放在各种 starter 而不是放在 core 里面。

### Examples

* 原型 bean 可以使用工厂模式进行注入，高并发场景下应该使用缓存以提高效率。

## Commit Message Format

项目名/模块名: 提交信息，可以多行。

Commit messages may use English or Chinese. Please be aware of spelling and clarity.

## Contact Us

* Open an issue on GitHub for questions or feedback.
* Join project discussions via the community forum or chat.

Thank you for contributing to Go-Spring!

---

# 贡献 Go-Spring 的指南

首先，感谢你关注并支持 Go-Spring 项目！
在贡献之前，请先阅读我们的 [贡献者行为准则](CODE_OF_CONDUCT.md)。

我们欢迎各种形式的贡献，包括提交 Issue、完善文档、修复 Bug、开发新功能等。请按照以下指引参与贡献。

## 目录

* [提交 Issue](#提交-issue)
* [提交 Pull Request](#提交-pull-request)
* [分支命名规范](#分支命名规范)
* [本地开发环境要求](#本地开发环境要求)
* [测试](#测试)
* [编码规范](#编码规范)
* [提交信息格式](#提交信息格式)
* [联系我们](#联系我们)

## 提交 Issue

* 在提交前，请先搜索现有 Issue，避免重复。
* 提供清晰的复现步骤、预期行为以及实际结果。
* 如有错误日志或运行环境信息，请一并附上。
* 有助于定位问题的信息包括版本、日志、截图、复现步骤等。

## 提交 Pull Request

1. **Fork 仓库并创建新分支**

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **保持一致的代码风格**

    * 遵循 Go 官方代码规范（使用 `gofmt`、`golint`、`go vet`）。
    * 遵循 `go-spring` 已有编码风格。
    * 推荐使用 [`golangci-lint`](https://github.com/golangci/golangci-lint) 进行本地检查。

3. **编写测试用例**

    * 所有新功能或 Bug 修复必须配备单元测试。
    * 使用 Go 内置 `testing` 包，测试文件命名为 `xxx_test.go`。
    * 示例：

      ```go
      func TestAdd(t *testing.T) {
          result := Add(1, 2)
          if result != 3 {
              t.Errorf("expected 3, got %d", result)
          }
      }
      ```

4. **更新文档**

    * 如果变更影响使用或接口，请同步更新 README 或代码注释。

5. **提交并创建 Pull Request**

    * 清晰说明：

        * **What**：本次修改的内容
        * **Why**：修改原因
        * **How**：实现方式
        * **Testing**：测试情况
    * 关联相关 Issue（如有）。

默认情况下，我们认为你的贡献可以按照 [Apache License 2.0](LICENSE) 授权。

## 分支命名规范

* `feature/xxx` – 新功能
* `fix/xxx` – Bug 修复
* `doc/xxx` – 文档更新
* `refactor/xxx` – 代码重构

## 本地开发环境要求

* Go 版本：推荐使用最新版稳定版（如 `go1.21+`）
* 使用 Go Modules 管理依赖
* 提交前确保所有测试通过：

  ```bash
  go test ./...
  ```

## 测试

* 运行 `go test ./...` 确保所有测试通过
* 对于示例或集成测试，请提供使用说明（如适用）

## 构建与工具

* **go.work 下 `go mod tidy` 不走 go.work 替换、走 GOPROXY 拉发布版 spring**，遇未发布本地包（404）或滞后版本会整体失败。对策：`go mod tidy -e` 容忍该错误改 `go.mod`/`go.sum`，再用 `go build ./...` 验证（走 go.work 本地树）。别因 tidy 报错认为改动有问题。
* **不在 go.work 的模块**（`temp/` 等）直接 `go build` 报 `directory prefix . does not contain modules listed in go.work`，必须 `GOWORK=off`（靠各自的 `replace` 块指回本仓）。
* **从仓库根编译成员模块**用 `go build -C <abs-module-dir> ./...`（shell cwd 会重置，`cd` 可能触发权限提示）。
* **`scripts/check-go-modules.sh` 不是只读校验**：它跑 `go fix` + `modernize` **原地改写源码**，会被自动提交扫走。只读手段用 `gofmt -l` / `go vet` / `go build` / `go test` / `bash scripts/check-config-namespace.sh` / `bash scripts/check-observability.sh`。
* **contrib dubbo examples 只由 docker-gated 的冒烟脚本端到端验证**——脚本是**每个 contrib example 各一份**（`contrib/<example>/scripts/smoke-test.sh`，共 23 份），不是仓库级的 `scripts/smoke-test.sh`；无 docker 时静默 `exit 0`，所以 build/vet 过**不**代表能跑。

## Example

* 全仓 example 的 `func init()` 统一成同一「chdir 到源码目录」完整版样板（含 `workDir` 打印）：

  ```go
  // init sets the working directory of the application to the directory
  // where this source file resides.
  // This ensures that any relative file operations are based on the source file location,
  // not the process launch path.
  func init() {
  	var execDir string
  	_, filename, _, ok := runtime.Caller(0)
  	if ok {
  		execDir = filepath.Dir(filename)
  	}
  	err := os.Chdir(execDir)
  	if err != nil {
  		panic(err)
  	}
  	workDir, err := os.Getwd()
  	if err != nil {
  		panic(err)
  	}
  	fmt.Println(workDir)
  }
  ```

  * 启动时打印工作目录是有意的（含 `fmt` import），别当噪音删。
  * init 里除样板外还有别的事的（如读脚本、设环境变量），不要整段替换，只对齐样板前缀。`temp/` 不参与扫改也不提交。
* `example/check.sh` 冒烟脚本四坑必须照抄现有新 starter 骨架：①所有 example 目录都叫 `example`，`compose()` 必须显式 `-p gs-<name>-example`（否则 `down -v` 会拆掉别家的容器）；②`gs.Run` 启动失败退出码是 0，`go run . > smoke.out 2>&1` 后必须 `grep -q "<成功标记>" smoke.out`；③mqadmin 等命令假成功（失败也退 0），gate 在输出文本上（`| grep -q success`）；④前台沙箱里 `/dev/tcp` 探测恒失败，用容器内探测（如 `docker exec <c> cqlsh -e "DESCRIBE CLUSTER"`）做就绪门槛。骨架 = `-p` 项目名 + marker 门 + 容器内就绪探测 + watchdog。

## 编码规范

### 命名规则

明确且统一的命名规则有助于帮助我们形成一致的思考和设计模式。

* package 一般使用名词或者动词，不推荐使用形容词。
* interface 一般使用名词或者形容词，动词短语也可。习惯上以 able、ible、er 等结尾。
* struct 一般使用名词或者动词短语。
* function 如果只返回 bool 值则尽量以 is、has 等打头，否则必须使用动词打头。

### 常用变量名

* 构造函数的变量名和结构体的字段名保持一致。
* arg.Arg 一般情况下命名为 a 或者 arg。
* cond.Condition 一般情况下命名为 c 或者 cond。
* function 一般情况下命名为 f 或者 fn。
* 返回结果一般情况下命名为 result 或者 ret。
* node 一般命名为 n。
* element 一般命名为 e。

### 编程规约

* 禁止导出全局变量。
* 错误分支优先处理，不要进行 err==nil 判断。
* 慎用嵌套(继承)，避免暴露不必要的方法。
* 限制每行长度最大不超过 100 个字符。
* 放心使用选项模式。
* 不对外直接暴露指针类型，使用值或者接口。
* 包名不能和 Golang 标准库重名。
* 注释里面的 bean 都是小写格式。
* 几乎所有的 panic 都应该打印其调用栈。
* 和包名同名的文件作为了解包的入口。
* 类型或函数的内容较短时也不能写在一行上，必须换行。
* 禁止程序在 init 阶段启动 goroutine。
* client 类型 starter 必须具有名字、排除同名同类型的 bean。
* 有共同抽象的 starter 实现使用相同的属性前缀，否则使用自身前缀。
* 在回放和测试模式下，尽量通过 panic 减少错误处理。
* 在录制模式下，通过打印日志的方式避免对主流程产生影响。
* 所有错误都需要打印错误发生位置的文件名和行号，保证错误排查的底线。
* boot 模块不提供全局读取和访问函数，推荐使用 ContextAware 注入。

### 注释

* 不要在注释上浪费太多文字，不要详细阐述你的思考，写清楚结论即可。
* 具有返回值的函数注释应该以 return 开头。
* 未导出函数尽量做到不使用注释也能知其意，而开放函数必须写注释。
* 代码中一些很少使用的场景必须写清楚其背景！

### 优秀经验

* 不用尽早抽象接口。
* 异常判断应当尽早返回。
* 在使用的地方定义接口，而不是实现的地方。
* 多数情况下不需要新增错误类型，只有深层嵌套的场景才需要。
* 所有可变参数如果函数名不能提供有效信息都应该使用 Option 模式。
* 好的架构都是在改进的过程中逐渐浮现出来的。
* 并发程序难写在于很难想清楚所有出现并发的情况。
* 为大公司提供框架，为小公司提供实现。
* 方法的接收者尽量使用指针，避免不必要的指针到值的转换过程。
* 测试环境打开竞态检测功能。
* 不入流的功能放在各种 starter 而不是放在 core 里面。

### 实战

* 原型 bean 可以使用工厂模式进行注入，高并发场景下应该使用缓存以提高效率。

## 提交信息格式

项目名/模块名: 提交信息，可以多行。

提交信息可使用英文或中文，请注意拼写和表达清晰。

## 联系我们

* 可通过 GitHub Issue 提问或反馈
* 参与项目讨论区交流

感谢你为 Go-Spring 做出的贡献！
