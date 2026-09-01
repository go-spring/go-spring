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

// Command mint prints a signed HS256 bearer token for the reference app, so
// check.sh (or a curl by hand) can authenticate against the order service. It
// signs with the same shared secret the order service verifies against
// (internal/authsecret), standing in for an identity provider the sample does
// not run.
//
// Usage:
//
//	go run ./cmd/mint [subject] [role...]   # defaults: subject "alice", role "user"
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"fullstack/internal/authsecret"
)

func main() {
	subject := "alice"
	roles := []string{"user"}
	if len(os.Args) > 1 {
		subject = os.Args[1]
	}
	if len(os.Args) > 2 {
		roles = os.Args[2:]
	}

	claims := jwt.MapClaims{
		"sub":   subject,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(authsecret.Secret))
	if err != nil {
		fmt.Fprintln(os.Stderr, "mint:", err)
		os.Exit(1)
	}
	fmt.Print(signed)
}
