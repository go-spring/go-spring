# Go-Spring Tools Manager (gs)

[English](README.md) | [中文](README_CN.md)

Go-Spring Tools Manager (gs) is a command-line program for managing and using various tools in the Go-Spring ecosystem.

## Installation

```shell
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/HEAD/gs/gs/install.sh)"
```

The script installs `gs` itself (with built-in `init`, `gen` and `add` subcommands).

### Optional External Tools

The following tools are dispatched via `gs <tool>` and installed on demand:

- `gs-http-gen`: HTTP IDL code generator (invoked by `gs gen`)
- `gs-mock`: generates mock code based on configuration

### Requirements

- Go language environment (1.26+)
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

- Create a new project: `gs init -m github.com/you/hello`
- Generate idl code: `gs gen` (run from a project root containing `gs.json` and `idl/`)
- Generate mock code: `gs mock ...` (requires `gs-mock` installed)

### View Tool Help

```shell
gs <tool> --help
```

## How It Works

The tool manager ships built-in subcommands (`init`, `gen`, `add`) directly.
For any other command, it looks for an executable prefixed with `gs-` in its directory (usually `$GOPATH/bin`) and dispatches to it.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.