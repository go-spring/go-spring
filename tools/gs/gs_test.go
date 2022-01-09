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

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestReplaceVersion(t *testing.T) {
	src, _ := ioutil.TempFile(os.TempDir(), ".go.mod")
	_, _ = src.WriteString(`module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-spring/spring-base v1.1.0-beta.0.20211001035852-bfba805daa15 // indirect
	github.com/go-spring/spring-core v1.0.6-0.20211001040940-f4fed6e6c943
	github.com/go-spring/starter-echo v1.1.0-alpha.0.20211002014844-f5432e77cd0f // indirect
	github.com/go-spring/starter-go-redis v1.1.0-alpha.0.20211002011402-f6f9d978d487
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)`)
	_ = src.Close()
	err := replaceModVersion(src.Name(), "v1.0.0")
	if err != nil {
		t.Fail()
	}
	b, _ := ioutil.ReadFile(src.Name())

	expect := `module github.com/go-spring/examples/spring-boot-demo

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-spring/spring-base v1.0.0 // indirect
	github.com/go-spring/spring-core v1.0.0
	github.com/go-spring/starter-echo v1.0.0 // indirect
	github.com/go-spring/starter-go-redis v1.0.0
)

//replace (
//	github.com/go-spring/spring-core => ../../spring/spring-core
//)
`

	if !bytes.Equal(b, []byte(expect)) {
		t.Fail()
		return
	}
	fmt.Println("test success")
}
