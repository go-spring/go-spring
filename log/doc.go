/*
 * Copyright 2025 The Go-Spring Authors.
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

/*
Package log is a high-performance and extensible logging library designed specifically for the Go programming
language. It provides flexible and structured logging capabilities, including context field extraction, multi-level
logging configuration, and multiple output options, making it ideal for server-side applications.

## Core Concepts:

Tag:

Tag is a core concept in the log package used to categorize logs. By registering a tag via the `RegisterTag`
function, you can use regular expressions to match the user-defined tags. This approach allows for a unified API
for logging without explicitly creating logger instances. Even third-party libraries can write logs without
setting up a logger object.

Loggers:

A Logger is the object that actually handles the logging process. You can obtain a logger instance using the
`GetLogger` function, which is mainly provided for compatibility with legacy projects. This allows you to
directly retrieve a logger by its name and log pre-formatted messages using the `Write` function.

Context Field Extraction:

Contextual data can be extracted and included in log entries via configurable functions:
- `log.StringFromContext`: Extracts a string value (e.g., a request ID) from the context.
- `log.FieldsFromContext`: Returns a list of structured fields from the context, such as trace IDs or user IDs.

Configuration from File:

The `log.RefreshFile` function allows loading the logger's configuration from an external file (e.g., yaml or JSON).

Logger Initialization and Logging:

- Using `GetLogger`, you can fetch a logger instance (often for compatibility with older systems).
- You can also register custom tags using `RegisterTag` to classify logs according to your needs.

Logging Messages:

The package provides various logging functions, such as `Tracef`, `Debugf`, `Infof`, `Warnf`, etc.,
which log messages at different levels (e.g., Trace, Debug, Info, Warn).
These functions can either accept structured fields or formatted messages.

Structured Logging:

The logger also supports structured logging, where fields are captured as key-value pairs and logged with the message.
The fields can be provided directly in the log functions or through a map.

## Examples:

Using a pre-registered tag:

	log.Tracef(ctx, TagRequestOut, "hello %s", "world")
	log.Debugf(ctx, TagRequestOut, "hello %s", "world")
	log.Infof(ctx, TagRequestIn, "hello %s", "world")
	log.Warnf(ctx, TagRequestIn, "hello %s", "world")
	log.Errorf(ctx, TagRequestIn, "hello %s", "world")
	log.Panicf(ctx, TagRequestIn, "hello %s", "world")
	log.Fatalf(ctx, TagRequestIn, "hello %s", "world")

Using structured fields:

	log.Trace(ctx, TagRequestOut, func() []log.Field {
		return []log.Field{
			log.Msgf("hello %s", "world"),
		}
	})

	log.Error(ctx, TagRequestIn, log.FieldsFromMap(map[string]any{
		"key1": "value1",
		"key2": "value2",
	}))
*/
package log
