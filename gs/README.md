# Go-Spring Tools Manager (gs)

[English](README.md) | [中文](README_CN.md)

Go-Spring Tools Manager (gs) is a command-line program for managing and using various tools in the Go-Spring ecosystem.

## Installation

```shell
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"
```

This script will automatically install the following tools:

- `gs`: the tool manager itself
- `gs-init`: creates new Go-Spring projects
- `gs-gen`: generates code from idl files
- `gs-mock`: generates mock code based on configuration

### Requirements

- Go language environment (1.24+)
- GOPATH and GOBIN properly configured
- GOBIN path needs to be added to the system PATH

## Usage

After installation, you can use the following command:

```shell
gs --help
```

This will display all available tools with their versions and descriptions.

### Using Specific Tools

```shell
gs <tool> [args]
```

For example:

- Create a new project: `gs init ...`
- Generate idl code: `gs gen ...`
- Generate mock code: `gs mock ...`

### View Tool Help

```shell
gs <tool> --help
```

## How It Works

The tool manager looks for executable files prefixed with `gs-` in its directory (usually `$GOPATH/bin`) and manages
them as available tools.
When a user invokes a tool, the manager executes the corresponding executable and passes the arguments.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.