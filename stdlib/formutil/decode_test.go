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

package formutil_test

import (
	"net/url"
	"testing"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/formutil"
	"github.com/go-spring/stdlib/testing/assert"
)

const ExpectedStr = "Bool=true&BoolPtr=true&Bytes=MQ%3D%3D&Float32=1&Float32Ptr=1&Int=1&Int64=1&Int64Ptr=1&IntPtr=1&Item=%7B%22Name%22%3A%221%22%7D&ItemList=%7B%22Name%22%3A%221%22%7D&String=1&StringIntMap=%7B%221%22%3A1%7D&StringIntPtrMap=%7B%221%22%3A1%7D&StringItemMap=%7B%221%22%3A%7B%22Name%22%3A%221%22%7D%7D&StringList=1&StringPtr=1&StringPtrList=1&Uint=1&Uint64=1&Uint64Ptr=1&UintPtr=1"

func (x *Request) DecodeForm(data string) error {
	m, err := url.ParseQuery(data)
	if err != nil {
		return err
	}

	var (
		hashBool bool
		hashInt  bool
	)

	for key, values := range m {
		if len(values) == 0 {
			continue
		}
		switch key {
		case "Bool":
			hashBool = true
			if x.Bool, err = formutil.DecodeBool(key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "BoolPtr":
			if x.BoolPtr, err = formutil.DecodeBoolPtr(key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Int":
			hashInt = true
			if x.Int, err = formutil.DecodeInt[int](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "IntPtr":
			if x.IntPtr, err = formutil.DecodeIntPtr[int](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Int64":
			if x.Int64, err = formutil.DecodeInt[int64](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Int64Ptr":
			if x.Int64Ptr, err = formutil.DecodeIntPtr[int64](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Uint":
			if x.Uint, err = formutil.DecodeUint[uint](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "UintPtr":
			if x.UintPtr, err = formutil.DecodeUintPtr[uint](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Uint64":
			if x.Uint64, err = formutil.DecodeUint[uint64](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Uint64Ptr":
			if x.Uint64Ptr, err = formutil.DecodeUintPtr[uint64](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Float32":
			if x.Float32, err = formutil.DecodeFloat[float32](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Float32Ptr":
			if x.Float32Ptr, err = formutil.DecodeFloatPtr[float32](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "String":
			if x.String, err = formutil.DecodeString(key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringPtr":
			if x.StringPtr, err = formutil.DecodeStringPtr(key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Bytes":
			if x.Bytes, err = formutil.DecodeBytes(key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "Item":
			if x.Item, err = formutil.DecodeJSON[*Item](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringList":
			if x.StringList, err = formutil.DecodeList(key, values, formutil.DecodeString); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringPtrList":
			if x.StringPtrList, err = formutil.DecodeList(key, values, formutil.DecodeStringPtr); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "ItemList":
			if x.ItemList, err = formutil.DecodeList[*Item](key, values, formutil.DecodeJSON[*Item]); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringIntMap":
			if x.StringIntMap, err = formutil.DecodeJSON[map[string]int](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringIntPtrMap":
			if x.StringIntPtrMap, err = formutil.DecodeJSON[map[string]*int](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		case "StringItemMap":
			if x.StringItemMap, err = formutil.DecodeJSON[map[string]*Item](key, values); err != nil {
				return errutil.Explain(err, "decode form field %s error", key)
			}
		default: // for linter
		}
	}

	if !hashBool {
		return errutil.Explain(nil, "missing required field Bool")
	}
	if !hashInt {
		return errutil.Explain(nil, "missing required field Int")
	}

	return nil
}

func TestDecode(t *testing.T) {
	var x Request
	if err := x.DecodeForm(ExpectedStr); err != nil {
		t.Fatal(err)
	}
	assert.That(t, x).Equal(ExpectedReq)
}
