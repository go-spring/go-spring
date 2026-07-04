# build-test 子流程

在 Go-Spring 项目里跑 `gofmt` / `go test` / `go build`,重点是定位到正确的子 module。

## 何时使用

- 用户说「编译一下」「跑测试」「gofmt 一下」「验证下能不能编」。
- 上一个子流程结束前的收敛验证。

## 前置检查

- Go 工具链在 PATH(`go version`)。
- 确认仓库结构:根目录未必有 `go.mod`,Go-Spring 仓库通常是多 module。

## 定位目标 module

按优先级:

1. **用户显式指定**:「跑 gs 的测试」「build starter-gin」→ 直接进对应目录。
2. **当前上下文**:刚改过的文件所在子 module(向上找到最近的 `go.mod`)。
3. **当前 cwd**:如果 cwd 本身在某个子 module 内,以它为目标。
4. **全量**:用户明确要求「所有 module」时,走 `go.work` 列表遍历;否则不要默认全量。

定位不到目标 module → 用 `AskUserQuestion` 让用户在候选列表中选一个。

## 工作流程

### 1. gofmt

在目标 module 根目录:

```bash
gofmt -l -w .
```

有输出的文件即被格式化的文件,记录下来一起报告。

### 2. go test

```bash
go test ./...
```

流式输出。默认不加 `-race` / `-count=1`,除非用户要求。测试失败 → 停下,把关键失败堆栈原文交出,不要重跑掩盖。

### 3. go build

```bash
go build ./...
```

流式输出。仅当用户要求「验证能否编译」或子流程末尾的收敛验证时执行;单纯改了测试代码时可跳过。

### 4. 多 module 场景

用户明确「所有 module」:

- 读取 `go.work` 或直接 `go list -f '{{.Dir}}' -m` 取 module 列表。
- 逐个进入执行,任一失败继续记录其它 module 结果,最后统一汇报。

## 完成后输出

- 目标 module 路径;
- gofmt 修改的文件清单;
- test 结果(通过数 / 失败数,失败用例名);
- build 结果(成功 / 失败原因);
- 若有失败,给出下一步定位建议(路径 + 关键错误行)。

## 关键约束

- **禁止**在根目录盲跑 `go test ./...`(仓库根没有 `go.mod`,会报错或跑错范围)。
- **禁止**吞掉失败:失败原文流式给用户,不重排/不概括后再重跑。
- **禁止**用 `--no-verify` 或跳过 hook 类的 flag 绕过失败。
- 只跑必要的验证;别用户没要 build 就顺手 build 完整个仓库。
