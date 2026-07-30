/*
 * Copyright 2024 The Go-Spring Authors.
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

package conf

// Validator is an optional interface that configuration structs can implement
// to perform cross-field, object-integrity checks after all fields are bound.
//
// Unlike per-field expr tags (which validate a single value in isolation),
// Validator has access to the entire populated struct, so it can check
// relationships between fields — for example, "host is required when port is
// non-zero", "min must not exceed max", etc.
//
// Validate is called automatically by the binding system after all struct
// fields are resolved, including for nested structs inside slices and maps.
// This means each nested struct can independently carry its own cross-field
// validation rules.
//
// The implementation must be safe to call on a zero-value receiver: the binding
// system uses reflect.New(t).Elem() to create struct values, so the struct is
// always addressable when Validate is called.
//
// Usage:
//
//	type ServerConfig struct {
//	    Host string `value:"${host}"`
//	    Port int    `value:"${port:=0}"`
//	}
//
//	func (c *ServerConfig) Validate() error {
//	    if c.Port > 0 && c.Host == "" {
//	        return fmt.Errorf("host is required when port is set")
//	    }
//	    if c.Port < 0 || c.Port > 65535 {
//	        return fmt.Errorf("port %d out of range [0, 65535]", c.Port)
//	    }
//	    return nil
//	}
//
// Comparison with expr tag:
//
//	expr       — per-field, single-value check (e.g., expr:"$ > 0")
//	Validator  — cross-field, multi-field check (e.g., "min ≤ max")
//
// Both approaches compose: a type can use expr tags for field-level constraints
// and implement Validator for inter-field integrity at the same time.
type Validator interface {
	Validate() error
}
