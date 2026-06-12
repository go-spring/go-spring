# gs-init

`gs-init` is a command-line tool for initializing Go projects. It is based on
the [go-spring/skeleton](https://github.com/go-spring/skeleton) project template, allowing you to quickly create a
well-structured Go project.

## Features

* Create projects based on the [go-spring/skeleton](https://github.com/go-spring/skeleton) template
* Automatically replace module name and package name
* Support specifying a Git branch
* Automatically generate project code

## Installation

* **Recommended way:**

Use the [gs](https://github.com/go-spring/gs) integrated development tool.

* To install this tool individually:

```bash
go install github.com/go-spring/gs-init@latest
```

## Usage

```bash
# Basic usage, module name is required
gs-init --module=github.com/your_name/your_project

# Specify a branch
gs-init --module=github.com/your_name/your_project --branch=main
```

## Flags

* `--module`: Specify the project module name (required)
* `--branch`: Specify the template branch to use, default is `main`

## How It Works

1. Clone the specified branch from the go-spring/skeleton repository
2. Remove the `.git` directory to detach from the template repository
3. Replace placeholders in the template with the actual module name and package name
4. Rename the project directory
5. Run the `gs gen` command to generate project code

## License

This project is licensed under the [Apache License 2.0](LICENSE).
