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

package pathidl

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Segment
		hasErr   bool
	}{
		{
			name:   "empty path",
			input:  "",
			hasErr: true,
		},
		{
			name:   "root path",
			input:  "/",
			hasErr: true,
		},
		{
			name:  "single static segment",
			input: "/users",
			expected: []Segment{
				{Type: Static, Value: "users"},
			},
		},
		{
			name:  "multiple static segments",
			input: "/api/v1/users",
			expected: []Segment{
				{Type: Static, Value: "api"},
				{Type: Static, Value: "v1"},
				{Type: Static, Value: "users"},
			},
		},
		{
			name:  "colon param",
			input: "/users/:id",
			expected: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "id"},
			},
		},
		{
			name:  "braced param",
			input: "/users/{id}",
			expected: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "id"},
			},
		},
		{
			name:  "colon wildcard",
			input: "/files/:path*",
			expected: []Segment{
				{Type: Static, Value: "files"},
				{Type: Wildcard, Value: "path"},
			},
		},
		{
			name:  "braced wildcard",
			input: "/files/{path...}",
			expected: []Segment{
				{Type: Static, Value: "files"},
				{Type: Wildcard, Value: "path"},
			},
		},
		{
			name:  "mixed params",
			input: "/api/v1/users/{userId}/posts/:postId",
			expected: []Segment{
				{Type: Static, Value: "api"},
				{Type: Static, Value: "v1"},
				{Type: Static, Value: "users"},
				{Type: Param, Value: "userId"},
				{Type: Static, Value: "posts"},
				{Type: Param, Value: "postId"},
			},
		},
		{
			name:  "complex path with multiple param types",
			input: "/org/{orgId}/repos/:repoId/branches/{branch...}",
			expected: []Segment{
				{Type: Static, Value: "org"},
				{Type: Param, Value: "orgId"},
				{Type: Static, Value: "repos"},
				{Type: Param, Value: "repoId"},
				{Type: Static, Value: "branches"},
				{Type: Wildcard, Value: "branch"},
			},
		},
		{
			name:  "param at beginning",
			input: "/:id/profile",
			expected: []Segment{
				{Type: Param, Value: "id"},
				{Type: Static, Value: "profile"},
			},
		},
		{
			name:  "consecutive parameters",
			input: "/users/:userId/posts/:postId/comments/:commentId",
			expected: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "userId"},
				{Type: Static, Value: "posts"},
				{Type: Param, Value: "postId"},
				{Type: Static, Value: "comments"},
				{Type: Param, Value: "commentId"},
			},
		},
		{
			name:  "braced param with dots",
			input: "/users/{user.id}",
			expected: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "user.id"},
			},
			hasErr: true,
		},
		{
			name:     "leading and trailing spaces",
			input:    "  /users/:id  ",
			expected: []Segment{{Type: Static, Value: "users"}, {Type: Param, Value: "id"}},
		},
		{
			name:   "invalid param starting with number",
			input:  "/users/:123id",
			hasErr: true,
		},
		{
			name:   "invalid braced param starting with number",
			input:  "/users/{123id}",
			hasErr: true,
		},
		{
			name:   "unmatched brace",
			input:  "/users/{id",
			hasErr: true,
		},
		{
			name:   "extra wildcard character",
			input:  "/users/:id**",
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			if tt.hasErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    []Segment
		expected string
		style    SegmentStyle
	}{
		{
			name: "static path with colon style",
			input: []Segment{
				{Type: Static, Value: "users"},
				{Type: Static, Value: "profile"},
			},
			expected: "/users/profile",
			style:    Colon,
		},
		{
			name: "param with colon style",
			input: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "id"},
			},
			expected: "/users/:id",
			style:    Colon,
		},
		{
			name: "wildcard with colon style",
			input: []Segment{
				{Type: Static, Value: "files"},
				{Type: Wildcard, Value: "path"},
			},
			expected: "/files/:path*",
			style:    Colon,
		},
		{
			name: "param with brace style",
			input: []Segment{
				{Type: Static, Value: "users"},
				{Type: Param, Value: "id"},
			},
			expected: "/users/{id}",
			style:    Brace,
		},
		{
			name: "wildcard with brace style",
			input: []Segment{
				{Type: Static, Value: "files"},
				{Type: Wildcard, Value: "path"},
			},
			expected: "/files/{path...}",
			style:    Brace,
		},
		{
			name: "complex path with colon style",
			input: []Segment{
				{
					Type:  Static,
					Value: "api",
				},
				{Type: Static, Value: "v1"},
				{Type: Static, Value: "users"},
				{Type: Param, Value: "userId"},
				{Type: Static, Value: "posts"},
				{Type: Param, Value: "postId"},
			},
			expected: "/api/v1/users/:userId/posts/:postId",
			style:    Colon,
		},
		{
			name: "complex path with brace style",
			input: []Segment{
				{Type: Static, Value: "api"},
				{Type: Static, Value: "v1"},
				{Type: Static, Value: "users"},
				{Type: Param, Value: "userId"},
				{Type: Static, Value: "posts"},
				{Type: Param, Value: "postId"},
			},
			expected: "/api/v1/users/{userId}/posts/{postId}",
			style:    Brace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Format(tt.input, tt.style)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
