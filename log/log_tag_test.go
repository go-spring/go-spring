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

package log

import (
	"strings"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestIsValidTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{"valid_1segments", "_def", true},
		{"valid_4segments", "service_module_submodule_component00", true},
		{"too_short_2", "ab", false},
		{"too_long_37", strings.Repeat("a", 37), false},
		{"uppercase", "Invalid_Tag", false},
		{"special_char", "tag!name", false},
		{"space", "tag name", false},
		{"hyphen", "tag-name", false},
		{"too_many_segments", "a_b_c_d_e", false},
		{"leading_underscore_1", "_service_component", true},
		{"leading_underscore_2", "__service_component", false},
		{"trailing_underscore_1", "service_component_", false},
		{"trailing_underscore_2", "service_component__", false},
		{"consecutive_underscore", "service__component", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidTag(tt.tag); got != tt.want {
				t.Errorf("isValidTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestRegisterTag(t *testing.T) {

	assert.Panic(t, func() {
		RegisterTag("1")
	}, "invalid log tag")

	assert.Panic(t, func() {
		RegisterAppTag("", "")
	}, "subType cannot be empty")
}

func TestGetAllTags(t *testing.T) {
	tags := GetAllTags()
	assert.That(t, tags).Equal([]string{
		"_app_def",
		"_biz_def",
		"_com_request_in",
		"_com_request_out",
		"_def",
	})
}

func TestBuildTag(t *testing.T) {
	tests := []struct {
		name     string
		mainType string
		subType  string
		action   string
		want     string
		panicMsg string
	}{
		{
			name:     "empty subtype panic",
			mainType: "app",
			subType:  "",
			action:   "",
			panicMsg: "subType cannot be empty",
		},
		{
			name:     "with action",
			mainType: "app",
			subType:  "startup",
			action:   "init",
			want:     "_app_startup_init",
		},
		{
			name:     "without action",
			mainType: "biz",
			subType:  "payment",
			action:   "",
			want:     "_biz_payment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panicMsg != "" {
				assert.Panic(t, func() {
					BuildTag(tt.mainType, tt.subType, tt.action)
				}, tt.panicMsg)
			} else {
				got := BuildTag(tt.mainType, tt.subType, tt.action)
				assert.String(t, got).Equal(tt.want)
			}
		})
	}
}

func TestRegisterTagValid(t *testing.T) {

	tag := RegisterTag("_test_tag")
	assert.That(t, tag).NotNil()
	assert.String(t, tag.tag).Equal("_test_tag")

	tag2 := RegisterTag("_test_tag")
	assert.That(t, tag).Equal(tag2)
}

func TestRegisterTags(t *testing.T) {

	// Test RegisterAppTag
	tag := RegisterAppTag("web", "start")
	assert.That(t, tag).NotNil()
	assert.String(t, tag.tag).Equal("_app_web_start")

	tag2 := RegisterAppTag("database", "")
	assert.That(t, tag2).NotNil()
	assert.String(t, tag2.tag).Equal("_app_database")

	// Test RegisterBizTag
	tag = RegisterBizTag("payment", "process")
	assert.That(t, tag).NotNil()
	assert.String(t, tag.tag).Equal("_biz_payment_process")

	tag2 = RegisterBizTag("user", "")
	assert.That(t, tag2).NotNil()
	assert.String(t, tag2.tag).Equal("_biz_user")

	// Test RegisterRPCTag
	tag = RegisterRPCTag("grpc", "call")
	assert.That(t, tag).NotNil()
	assert.String(t, tag.tag).Equal("_rpc_grpc_call")

	tag2 = RegisterRPCTag("http", "")
	assert.That(t, tag2).NotNil()
	assert.String(t, tag2.tag).Equal("_rpc_http")
}
