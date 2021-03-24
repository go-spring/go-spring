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

package SpringSwagger

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-openapi/spec"
	"github.com/go-spring/spring-core/web"
)

// Swagger 封装 spec.Swagger 对象，提供流式调用
type Swagger struct {
	spec.Swagger
}

// NewSwagger swagger 的构造函数
func NewSwagger() *Swagger {
	return &Swagger{
		Swagger: spec.Swagger{
			SwaggerProps: spec.SwaggerProps{
				Swagger: "2.0",
				Info: &spec.Info{
					InfoProps: spec.InfoProps{
						Contact: &spec.ContactInfo{},
						License: &spec.License{},
					},
				},
				Paths: &spec.Paths{
					Paths: make(map[string]spec.PathItem),
				},
				Definitions:         make(map[string]spec.Schema),
				SecurityDefinitions: map[string]*spec.SecurityScheme{},
			},
		},
	}
}

// ReadDoc 获取应用的 Swagger 描述内容
func (s *Swagger) ReadDoc() string {
	if b, err := s.MarshalJSON(); err == nil {
		return string(b)
	} else {
		panic(err)
	}
}

// WithID 设置应用 ID
func (s *Swagger) WithID(id string) *Swagger {
	s.ID = id
	return s
}

// WithConsumes 设置消费协议
func (s *Swagger) WithConsumes(consumes ...string) *Swagger {
	s.Consumes = consumes
	return s
}

// WithProduces 设置生产协议
func (s *Swagger) WithProduces(produces ...string) *Swagger {
	s.Produces = produces
	return s
}

// WithSchemes 设置服务协议
func (s *Swagger) WithSchemes(schemes ...string) *Swagger {
	s.Schemes = schemes
	return s
}

// WithDescription 设置服务描述
func (s *Swagger) WithDescription(description string) *Swagger {
	s.Info.Description = description
	return s
}

// WithTitle 设置服务名称
func (s *Swagger) WithTitle(title string) *Swagger {
	s.Info.Title = title
	return s
}

// WithTermsOfService 设置服务条款地址
func (s *Swagger) WithTermsOfService(termsOfService string) *Swagger {
	s.Info.TermsOfService = termsOfService
	return s
}

// WithContact 设置作者的名字、主页地址、邮箱
func (s *Swagger) WithContact(name string, url string, email string) *Swagger {
	c := new(spec.ContactInfo)
	c.Name = name
	c.URL = url
	c.Email = email
	s.Info.Contact = c
	return s
}

// WithLicense 设置开源协议的名称、地址
func (s *Swagger) WithLicense(name string, url string) *Swagger {
	l := new(spec.License)
	l.Name = name
	l.URL = url
	s.Info.License = l
	return s
}

// WithVersion 设置 API 版本号
func (s *Swagger) WithVersion(version string) *Swagger {
	s.Info.Version = version
	return s
}

// WithHost 设置可用服务器地址
func (s *Swagger) WithHost(host string) *Swagger {
	s.Host = host
	return s
}

// WithBasePath 设置 API 路径的前缀
func (s *Swagger) WithBasePath(basePath string) *Swagger {
	s.BasePath = basePath
	return s
}

// WithTags 添加标签
func (s *Swagger) WithTags(tags ...spec.Tag) *Swagger {
	s.Swagger.Tags = tags
	return s
}

// AddPath 添加一个路由
func (s *Swagger) AddPath(path string, method uint32, op web.Operation) {

	path = strings.TrimPrefix(path, s.BasePath)
	path = strings.TrimRight(path, "/")
	pathItem, ok := s.Paths.Paths[path]

	if !ok {
		pathItem = spec.PathItem{PathItemProps: spec.PathItemProps{}}
	}

	//for _, m := range web.GetMethod(method) {
	//	switch m {
	//	case http.MethodGet:
	//		pathItem.Get = op.Operation
	//	case http.MethodPost:
	//		pathItem.Post = op.Operation
	//	case http.MethodPut:
	//		pathItem.Put = op.Operation
	//	case http.MethodDelete:
	//		pathItem.Delete = op.Operation
	//	case http.MethodOptions:
	//		pathItem.Options = op.Operation
	//	case http.MethodHead:
	//		pathItem.Head = op.Operation
	//	case http.MethodPatch:
	//		pathItem.Patch = op.Operation
	//	}
	//}

	s.Paths.Paths[path] = pathItem
}

// AddDefinition 添加一个定义
func (s *Swagger) AddDefinition(name string, schema *spec.Schema) *Swagger {
	s.Definitions[name] = *schema
	return s
}

type DefinitionField struct {
	Description string
	Example     interface{}
	Enums       []interface{}
}

// BindDefinitions 绑定一个定义
func (s *Swagger) BindDefinitions(i ...interface{}) *Swagger {
	m := map[string]DefinitionField{}
	for _, v := range i {
		s.BindDefinitionWithTags(v, m)
	}
	return s
}

// BindDefinitionWithTags 绑定一个定义
func (s *Swagger) BindDefinitionWithTags(i interface{}, attachFields map[string]DefinitionField) *Swagger {

	it := reflect.TypeOf(i)
	if it.Kind() == reflect.Ptr {
		it = it.Elem()
	}

	objSchema := new(spec.Schema).Typed("object", "")
	for i := 0; i < it.NumField(); i++ { // TODO json 和 xml 分开解析
		f := it.Field(i)

		// 处理 XML 标签
		var xmlTag []string
		if tag, ok := f.Tag.Lookup("xml"); ok {
			xmlTag = strings.Split(tag, ",")
			if f.Type == reflect.TypeOf(xml.Name{}) {
				objSchema.WithXMLName(xmlTag[0])
				continue
			}
		}

		propName := f.Name

		// 处理 JSON 标签
		var jsonTag []string
		if tag, ok := f.Tag.Lookup("json"); ok {
			jsonTag = strings.Split(tag, ",")
			propName = jsonTag[0]
		}

		var propSchema *spec.Schema
		switch k := f.Type.Kind(); k {
		case reflect.Bool:
			propSchema = spec.BoolProperty()
		case reflect.Int8:
			propSchema = spec.Int8Property()
		case reflect.Int16:
			propSchema = spec.Int16Property()
		case reflect.Int32:
			propSchema = spec.Int32Property()
		case reflect.Int64:
			propSchema = spec.Int64Property()
		case reflect.String:
			propSchema = spec.StringProperty()
		case reflect.Struct:
			if f.Type == reflect.TypeOf(time.Time{}) {
				propSchema = spec.DateTimeProperty()
			} else {
				panic(fmt.Errorf("unsupported swagger type %s", f.Type))
			}
		case reflect.Ptr:
			if et := f.Type.Elem(); et.Kind() == reflect.Struct {
				propSchema = spec.RefSchema("#/definitions/" + et.Name())
			} else {
				panic(fmt.Errorf("unsupported swagger type %s", f.Type))
			}
		case reflect.Slice:
			{
				et := f.Type.Elem()

				var items *spec.Schema
				switch k := et.Kind(); k {
				case reflect.Bool:
					items = spec.BoolProperty()
				case reflect.Int8:
					items = spec.Int8Property()
				case reflect.Int16:
					items = spec.Int16Property()
				case reflect.Int32:
					items = spec.Int32Property()
				case reflect.Int64:
					items = spec.Int64Property()
				case reflect.String:
					items = spec.StringProperty()
				case reflect.Struct:
					items = spec.RefSchema("#/definitions/" + et.Name())
				default:
					panic(fmt.Errorf("unsupported swagger type %s", f.Type))
				}

				if len(xmlTag) > 0 {
					items.WithXMLName(xmlTag[0])
				}

				propSchema = spec.ArrayProperty(items)
			}
		default:
			panic(fmt.Errorf("unsupported swagger type %s", f.Type))
		}

		if len(xmlTag) > 1 {
			for _, v := range xmlTag {
				if v == "wrapped" {
					propSchema.AsWrappedXML()
					break
				}
			}
		}

		required := true

		for _, v := range jsonTag {
			if v == "omitempty" {
				required = false
				break
			}
		}

		if required {
			objSchema.AddRequired(propName)
		}

		if attachField, ok := attachFields[propName]; ok {
			if len(attachField.Enums) > 0 {
				propSchema.WithEnum(attachField.Enums...)
			}
			if attachField.Description != "" {
				propSchema.WithDescription(attachField.Description)
			}
			if attachField.Example != "" {
				propSchema.WithExample(attachField.Example)
			}
		}

		objSchema.SetProperty(propName, *propSchema)
	}

	s.Definitions[it.Name()] = *objSchema
	return s
}

// AddBasicSecurityDefinition 添加 Basic 方式认证
func (s *Swagger) AddBasicSecurityDefinition() *Swagger {
	s.Swagger.SecurityDefinitions["BasicAuth"] = spec.BasicAuth()
	return s
}

// AddApiKeySecurityDefinition 添加 ApiKey 方式认证
func (s *Swagger) AddApiKeySecurityDefinition(name string, in string) *Swagger {
	if name == "" {
		name = "ApiKeyAuth"
	}
	s.Swagger.SecurityDefinitions[name] = spec.APIKeyAuth(name, in)
	return s
}

// AddOauth2ApplicationSecurityDefinition 添加 OAuth2 Application 方式认证
func (s *Swagger) AddOauth2ApplicationSecurityDefinition(name string, tokenUrl string, scopes map[string]string) *Swagger {
	if name == "" {
		name = "OAuth2Application"
	}
	securityScheme := spec.OAuth2Application(tokenUrl)
	return s.securitySchemeWithScopes(name, securityScheme, scopes)
}

// AddOauth2ImplicitSecurityDefinition 添加 OAuth2 Implicit 方式认证
func (s *Swagger) AddOauth2ImplicitSecurityDefinition(name string, authorizationUrl string, scopes map[string]string) *Swagger {
	if name == "" {
		name = "OAuth2Implicit"
	}
	securityScheme := spec.OAuth2Implicit(authorizationUrl)
	return s.securitySchemeWithScopes(name, securityScheme, scopes)
}

// AddOauth2PasswordSecurityDefinition 添加 OAuth2 Password 方式认证
func (s *Swagger) AddOauth2PasswordSecurityDefinition(name string, tokenUrl string, scopes map[string]string) *Swagger {
	if name == "" {
		name = "OAuth2Password"
	}
	securityScheme := spec.OAuth2Password(tokenUrl)
	return s.securitySchemeWithScopes(name, securityScheme, scopes)
}

// AddOauth2AccessCodeSecurityDefinition 添加 OAuth2 AccessCode 方式认证
func (s *Swagger) AddOauth2AccessCodeSecurityDefinition(name string, authorizationUrl string, tokenUrl string, scopes map[string]string) *Swagger {
	if name == "" {
		name = "OAuth2AccessCode"
	}
	securityScheme := spec.OAuth2AccessToken(authorizationUrl, tokenUrl)
	return s.securitySchemeWithScopes(name, securityScheme, scopes)
}

func (s *Swagger) securitySchemeWithScopes(name string, scheme *spec.SecurityScheme, scopes map[string]string) *Swagger {
	securityScheme := scheme
	for scope, description := range scopes {
		securityScheme.AddScope(scope, description)
	}
	s.Swagger.SecurityDefinitions[name] = securityScheme
	return s
}

// WithExternalDocs
func (s *Swagger) WithExternalDocs(externalDocs *spec.ExternalDocumentation) *Swagger {
	s.Swagger.ExternalDocs = externalDocs
	return s
}
