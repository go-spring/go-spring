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

package SpringRedoc

import (
	"html/template"

	"github.com/go-spring/spring-web"
	"github.com/swaggo/http-swagger"
)

func init() {
	SpringWeb.RegisterSwaggerHandler(func(mapping SpringWeb.RootRouter, doc string) {
		hSwagger := httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))

		// 注册 swagger-ui 和 doc.json 接口
		mapping.GetMapping("/swagger/*", func(webCtx SpringWeb.WebContext) {
			if webCtx.PathParam("*") == "doc.json" {
				webCtx.Blob(SpringWeb.MIMEApplicationJSONCharsetUTF8, []byte(doc))
			} else {
				hSwagger(webCtx.ResponseWriter(), webCtx.Request())
			}
		})

		// 注册 redoc 接口
		mapping.GetMapping("/redoc", ReDoc)
	})
}

// ReDoc redoc 响应函数
func ReDoc(ctx SpringWeb.WebContext) {

	index, err := template.New("redoc.html").Parse(redocTempl)
	if err != nil {
		panic(err)
	}

	// 不确定 Execute 是否线程安全，官方文档表示也许是线程安全的，谁知道呢
	_ = index.Execute(ctx.ResponseWriter(), map[string]interface{}{
		"URL": "/swagger/doc.json",
	})
}

const redocTempl = `
<!DOCTYPE html>
<html>
  <head>
    <title>ReDoc</title>
    <!-- needed for adaptive design -->
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">

    <!--
    ReDoc doesn't change outer page styles
    -->
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <redoc spec-url='{{.URL}}'></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@next/bundles/redoc.standalone.js"> </script>
  </body>
</html>
`
