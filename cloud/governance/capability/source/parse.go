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

package source

import (
	"strings"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// Parse binds one control-plane DOCUMENT through spring/conf's value-tag
// machinery — the one place a document is bound — so a rules file uses the same
// keys an app.properties entry would. It is generic over the document type and
// takes NO prefix: the type declares its key space itself, by carrying the full
// key on every top-level value tag:
//
//	type Document struct {
//	    Client ClientConfig `value:"${spring.fault.client:=}"`
//	    Server ServerConfig `value:"${spring.fault.server:=}"`
//	}
//
// Descendants stay RELATIVE to their parent (a field of Client writes
// `${rules:=}`, not the whole path) — that is the same rule go-spring uses
// everywhere, applied once at the document root.
//
// name identifies the document (path, URL or dataId): it picks the format by
// extension and labels parse/bind errors. An explicit format overrides the
// extension.
//
// The WHOLE document is bound: keys T does not name are ignored, and a document
// that carries none of T's keys binds the zero value. There is no root-key
// guard — the source contract is "everything you push, you vouch for", and a
// root that cannot tolerate an accidental empty document enforces that in its
// own binding (fault's required run-id does exactly that).
func Parse[T any](name string, data []byte, format string) (T, error) {
	var doc T
	if format == "" {
		format = formatOf(name)
	}
	parsed, err := reader.Read(format, data)
	if err != nil {
		return doc, errutil.Explain(err, "source %s: parse failed", name)
	}
	storage := flatten.NewPropertiesStorage(flatten.MapProperties(parsed))
	if err = conf.Bind(storage, &doc); err != nil {
		return doc, errutil.Explain(err, "source %s: bind failed", name)
	}
	return doc, nil
}

// formatOf infers the document format from a name (path, dataId, URL) by
// extension, defaulting to properties — the most common config-center format.
func formatOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:] // reader.Read accepts a dotted extension
	}
	return ".properties"
}
