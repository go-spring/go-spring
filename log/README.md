# Go-Spring :: Log

<div>
   <img src="https://img.shields.io/github/license/go-spring/log" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/log" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/log?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/log" > 
      <img src="https://codecov.io/gh/go-spring/log/graph/badge.svg?token=QBCHVEK97Q" alt="test-coverage"/> 
   </a>
   <a href="https://deepwiki.com/go-spring/log"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

**Go-Spring :: Log** is a high-performance and extensible logging library designed specifically for the Go programming
language. It offers flexible and structured logging capabilities, including context field extraction, multi-level
logging configuration, and multiple output options, making it ideal for a wide range of server-side applications.

## Features

* **Multi-Level Logging**: Supports standard log levels such as `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Panic`, and
  `Fatal`, suitable for debugging and monitoring in various scenarios.
* **Structured Logging**: Records logs in a structured format with key fields like `trace_id` and `span_id`, making them
  easy to parse and analyze by log aggregation systems.
* **Context Integration**: Extracts additional information from `context.Context` (e.g., request ID, user ID) and
  automatically attaches them to log entries.
* **Tag-Based Logging**: Introduces a tag system to distinguish logs across different modules or business lines.
* **Plugin Architecture**:
    * **Appender**: Supports multiple output targets including console and file.
    * **Layout**: Provides both plain text and JSON formatting for log output.
    * **Logger**: Offers both synchronous and asynchronous loggers; asynchronous mode avoids blocking the main thread.
* **Performance Optimizations**: Utilizes buffer management and event pooling to minimize memory allocation overhead.
* **Dynamic Configuration Reload**: Supports runtime reloading of logging configurations from external files.
* **Well-Tested**: All core modules are covered with unit tests to ensure stability and reliability.

## Core Concepts

### Tag

Tag is the core concept in this logging library, used to categorize logs. You can register tag via the `RegisterTag`
function and match them with regular expressions.
This approach enables a unified logging API without the need to explicitly create logger instances, even for third-party
libraries, allowing them to write logs in a standardized way.

### Logger

A `Logger` is the actual component that processes logs. You can retrieve a logger instance using the `GetLogger`
function, mainly for backward compatibility with older projects.
This allows you to fetch a logger by name and use the `Write` function to record pre-formatted messages.

### Contextual Field Extraction

You can configure functions to extract contextual data from `context.Context` and include them in log entries:

* `StringFromContext`: extracts string values from the context (e.g., request ID).
* `FieldsFromContext`: returns structured fields from the context, such as trace ID or user ID.

## Installation

```bash
go get github.com/go-spring/log
```

## Quick Start

Here's a simple example demonstrating how to use Go-Spring :: Log:

```go
package main

import (
	"context"

	"github.com/go-spring/log"
)

func main() {
	// Set the function to extract fields from context.
	// You may also use StringFromContext if only strings are needed.
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		return []log.Field{
			log.String("trace_id", "0a882193682db71edd48044db54cae88"),
			log.String("span_id", "50ef0724418c0a66"),
		}
	}

	// Load configuration file
	err := log.RefreshFile("log.properties")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Record logs
	log.Infof(ctx, log.TagAppDef, "This is an info message")
	log.Errorf(ctx, log.TagBizDef, "This is an error message")

    // Log with structured fields
    log.Info(ctx, log.TagAppDef,
        log.String("key1", "value1"),
        log.Int("key2", 123),
        log.Msg("structured log message"),
    )
}
```

## Configuration

Go-Spring :: Log supports defining logging behavior via JSON, or YAML configuration files. For example:

```properties
bufferCap=1KB
bufferSize=1000

appender.file.type=File
appender.file.file=log.txt
appender.file.layout.type=JSONLayout

appender.console.type=Console
appender.console.layout.type=TextLayout

logger.root.type=Logger
logger.root.level=warn
logger.root.appenderRef.ref=console

logger.myLogger.type=AsyncLogger
logger.myLogger.level=trace
logger.myLogger.tag=_com_request_in,_com_request_*
logger.myLogger.bufferSize=${bufferSize}
logger.myLogger.appenderRef[0].ref=file
```

## Plugin Development

Go-Spring :: Log offers rich plugin interfaces for developers to easily implement custom `Appender`, `Layout`, and
`Logger` components.

## License

Go-Spring :: Log is licensed under the [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).
