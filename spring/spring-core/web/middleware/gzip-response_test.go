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

package middleware

import (
	"fmt"
	"github.com/go-spring/spring-core/web"
	"net/http"
	"testing"
)

func TestShouldCompress(t *testing.T) {
	fmt.Println(`
       yaml example-

               web:
                 server:
                    compression:
                      level: 1
                      enable: true

	  main.go import-
		_ "github.com/go-spring/spring-core/web/middleware"
`, "")

	req, _ := http.NewRequest("GET", "http://t.dev:8080/user", nil)
	req.Header.Set(web.HeaderAcceptEncoding, "gzip")
	req.Header.Set(web.HeaderAccept, "application/json")
	b := shouldCompress(req)
	fmt.Println("is should compress: ", b)

	req2, _ := http.NewRequest("GET", "http://t.dev:8080/user.html", nil)
	req2.Header.Set(web.HeaderAcceptEncoding, "gzip")
	req2.Header.Set(web.HeaderAccept, "text/html")
	b2 := shouldCompress(req2)
	fmt.Println("is should compress: ", b2)

	req3, _ := http.NewRequest("GET", "http://t.dev:8080/user.jpeg", nil)
	req3.Header.Set(web.HeaderAcceptEncoding, "gzip")
	b3 := shouldCompress(req3)
	fmt.Println("is should compress: ", b3)
}
