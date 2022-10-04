/*
 * Copyright 2012-2019 the original author or authors.
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

package database

//
//var errNilPtr = errors.New("destination pointer is nil")
//
//func convertAssign(dest, src interface{}) error {
//	// Common cases, without reflect.
//	switch s := src.(type) {
//	case string:
//		switch d := dest.(type) {
//		case *string:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = s
//			return nil
//		case *[]byte:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = []byte(s)
//			return nil
//		case *sql.RawBytes:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = append((*d)[:0], s...)
//			return nil
//		}
//	case []byte:
//		switch d := dest.(type) {
//		case *string:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = string(s)
//			return nil
//		case *interface{}:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = cloneBytes(s)
//			return nil
//		case *[]byte:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = cloneBytes(s)
//			return nil
//		case *sql.RawBytes:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = s
//			return nil
//		}
//	case time.Time:
//		switch d := dest.(type) {
//		case *time.Time:
//			*d = s
//			return nil
//		case *string:
//			*d = s.Format(time.RFC3339Nano)
//			return nil
//		case *[]byte:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = []byte(s.Format(time.RFC3339Nano))
//			return nil
//		case *sql.RawBytes:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = s.AppendFormat((*d)[:0], time.RFC3339Nano)
//			return nil
//		}
//	case decimalDecompose:
//		switch d := dest.(type) {
//		case decimalCompose:
//			return d.Compose(s.Decompose(nil))
//		}
//	case nil:
//		switch d := dest.(type) {
//		case *interface{}:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = nil
//			return nil
//		case *[]byte:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = nil
//			return nil
//		case *sql.RawBytes:
//			if d == nil {
//				return errNilPtr
//			}
//			*d = nil
//			return nil
//		}
//	}
//
//	var sv reflect.Value
//
//	switch d := dest.(type) {
//	case *string:
//		sv = reflect.ValueOf(src)
//		switch sv.Kind() {
//		case reflect.Bool,
//			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
//			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
//			reflect.Float32, reflect.Float64:
//			*d = asString(src)
//			return nil
//		}
//	case *[]byte:
//		sv = reflect.ValueOf(src)
//		if b, ok := asBytes(nil, sv); ok {
//			*d = b
//			return nil
//		}
//	case *sql.RawBytes:
//		sv = reflect.ValueOf(src)
//		if b, ok := asBytes([]byte(*d)[:0], sv); ok {
//			*d = sql.RawBytes(b)
//			return nil
//		}
//	case *bool:
//		bv, err := driver.Bool.ConvertValue(src)
//		if err == nil {
//			*d = bv.(bool)
//		}
//		return err
//	case *interface{}:
//		*d = src
//		return nil
//	}
//
//	dpv := reflect.ValueOf(dest)
//	if dpv.Kind() != reflect.Ptr {
//		return errors.New("destination not a pointer")
//	}
//	if dpv.IsNil() {
//		return errNilPtr
//	}
//
//	if !sv.IsValid() {
//		sv = reflect.ValueOf(src)
//	}
//
//	dv := reflect.Indirect(dpv)
//	if sv.IsValid() && sv.Type().AssignableTo(dv.Type()) {
//		switch b := src.(type) {
//		case []byte:
//			dv.Set(reflect.ValueOf(cloneBytes(b)))
//		default:
//			dv.Set(sv)
//		}
//		return nil
//	}
//
//	if dv.Kind() == sv.Kind() && sv.Type().ConvertibleTo(dv.Type()) {
//		dv.Set(sv.Convert(dv.Type()))
//		return nil
//	}
//
//	// The following conversions use a string value as an intermediate representation
//	// to convert between various numeric types.
//	//
//	// This also allows scanning into user defined types such as "type Int int64".
//	// For symmetry, also check for string destination types.
//	switch dv.Kind() {
//	case reflect.Ptr:
//		if src == nil {
//			dv.Set(reflect.Zero(dv.Type()))
//			return nil
//		}
//		dv.Set(reflect.New(dv.Type().Elem()))
//		return convertAssign(dv.Interface(), src)
//	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//		if src == nil {
//			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
//		}
//		s := asString(src)
//		i64, err := strconv.ParseInt(s, 10, dv.Type().Bits())
//		if err != nil {
//			err = strconvErr(err)
//			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
//		}
//		dv.SetInt(i64)
//		return nil
//	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//		if src == nil {
//			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
//		}
//		s := asString(src)
//		u64, err := strconv.ParseUint(s, 10, dv.Type().Bits())
//		if err != nil {
//			err = strconvErr(err)
//			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
//		}
//		dv.SetUint(u64)
//		return nil
//	case reflect.Float32, reflect.Float64:
//		if src == nil {
//			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
//		}
//		s := asString(src)
//		f64, err := strconv.ParseFloat(s, dv.Type().Bits())
//		if err != nil {
//			err = strconvErr(err)
//			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
//		}
//		dv.SetFloat(f64)
//		return nil
//	case reflect.String:
//		if src == nil {
//			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
//		}
//		switch v := src.(type) {
//		case string:
//			dv.SetString(v)
//			return nil
//		case []byte:
//			dv.SetString(string(v))
//			return nil
//		}
//	}
//
//	return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, dest)
//}
//
//func strconvErr(err error) error {
//	if ne, ok := err.(*strconv.NumError); ok {
//		return ne.Err
//	}
//	return err
//}
//
//func cloneBytes(b []byte) []byte {
//	if b == nil {
//		return nil
//	}
//	c := make([]byte, len(b))
//	copy(c, b)
//	return c
//}
//
//func asString(src interface{}) string {
//	switch v := src.(type) {
//	case string:
//		return v
//	case []byte:
//		return string(v)
//	}
//	rv := reflect.ValueOf(src)
//	switch rv.Kind() {
//	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//		return strconv.FormatInt(rv.Int(), 10)
//	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//		return strconv.FormatUint(rv.Uint(), 10)
//	case reflect.Float64:
//		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
//	case reflect.Float32:
//		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
//	case reflect.Bool:
//		return strconv.FormatBool(rv.Bool())
//	}
//	return fmt.Sprintf("%v", src)
//}
//
//func asBytes(buf []byte, rv reflect.Value) (b []byte, ok bool) {
//	switch rv.Kind() {
//	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//		return strconv.AppendInt(buf, rv.Int(), 10), true
//	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//		return strconv.AppendUint(buf, rv.Uint(), 10), true
//	case reflect.Float32:
//		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 32), true
//	case reflect.Float64:
//		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 64), true
//	case reflect.Bool:
//		return strconv.AppendBool(buf, rv.Bool()), true
//	case reflect.String:
//		s := rv.String()
//		return append(buf, s...), true
//	}
//	return
//}
//
//// decimal composes or decomposes a decimal value to and from individual parts.
//// There are four parts: a boolean negative flag, a form byte with three possible states
//// (finite=0, infinite=1, NaN=2), a base-2 big-endian integer
//// coefficient (also known as a significand) as a []byte, and an int32 exponent.
//// These are composed into a final value as "decimal = (neg) (form=finite) coefficient * 10 ^ exponent".
//// A zero length coefficient is a zero value.
//// The big-endian integer coefficient stores the most significant byte first (at coefficient[0]).
//// If the form is not finite the coefficient and exponent should be ignored.
//// The negative parameter may be set to true for any form, although implementations are not required
//// to respect the negative parameter in the non-finite form.
////
//// Implementations may choose to set the negative parameter to true on a zero or NaN value,
//// but implementations that do not differentiate between negative and positive
//// zero or NaN values should ignore the negative parameter without error.
//// If an implementation does not support Infinity it may be converted into a NaN without error.
//// If a value is set that is larger than what is supported by an implementation,
//// an error must be returned.
//// Implementations must return an error if a NaN or Infinity is attempted to be set while neither
//// are supported.
////
//// NOTE(kardianos): This is an experimental interface. See https://golang.org/issue/30870
//type decimal interface {
//	decimalDecompose
//	decimalCompose
//}
//
//type decimalDecompose interface {
//	// Decompose returns the internal decimal state in parts.
//	// If the provided buf has sufficient capacity, buf may be returned as the coefficient with
//	// the value set and length set as appropriate.
//	Decompose(buf []byte) (form byte, negative bool, coefficient []byte, exponent int32)
//}
//
//type decimalCompose interface {
//	// Compose sets the internal decimal value from parts. If the value cannot be
//	// represented then an error should be returned.
//	Compose(form byte, negative bool, coefficient []byte, exponent int32) error
//}
