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
- [Contact Us](#contact-us)

## Submitting Issues

- Search existing issues before submitting to avoid duplicates.
- Provide clear reproduction steps, expected behavior, and actual results.
- Include error logs and environment information if applicable.

## Submitting Pull Requests

1. **Fork the repository and create a new branch**

   ```bash
   git checkout -b feature/your-feature-name
   ````

2. **Maintain consistent coding style**

    * Follow Go’s official style guidelines (`gofmt`, `golint`, `go vet`).
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
* [联系我们](#联系我们)

## 提交 Issue

* 在提交前，请先搜索现有 Issue，避免重复。
* 提供清晰的复现步骤、预期行为以及实际结果。
* 如有错误日志或运行环境信息，请一并附上。

## 提交 Pull Request

1. **Fork 仓库并创建新分支**

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **保持一致的代码风格**

    * 遵循 Go 官方代码规范（使用 `gofmt`、`golint`、`go vet`）。
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

## 联系我们

* 可通过 GitHub Issue 提问或反馈
* 参与项目讨论区交流

感谢你为 Go-Spring 做出的贡献！
