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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/go-openapi/spec"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/spring-swag"
	"github.com/go-spring/spring-swag/swagger"
)

func init() {
	web.RegisterSwaggerHandler(func(r web.Router, doc string) {})
}

type ApiResponse struct {
	Code    int32  `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

type Category struct {
	XMLName xml.Name `xml:"Category"`
	Id      int64    `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
}

type Order struct {
	XMLName  xml.Name  `xml:"Order"`
	Id       int64     `json:"id,omitempty"`
	PetId    int64     `json:"petId,omitempty"`
	Quantity int32     `json:"quantity,omitempty"`
	ShipDate time.Time `json:"shipDate,omitempty"`
	Status   string    `json:"status,omitempty"` // Order Status
	Complete bool      `json:"complete,omitempty"`
}

type Tag struct {
	XMLName xml.Name `xml:"Tag"`
	Id      int64    `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
}

type Pet struct {
	XMLName   xml.Name  `xml:"Pet"`
	Id        int64     `json:"id,omitempty"`
	Category  *Category `json:"category,omitempty"`
	Name      string    `json:"name"`
	PhotoUrls []string  `json:"photoUrls" xml:"photoUrls>photoUrl"`
	Tags      []Tag     `json:"tags,omitempty" xml:"tags>tag"`
	Status    string    `json:"status,omitempty"` // pet status in the store
}

type User struct {
	XMLName    xml.Name `xml:"User"`
	Id         int64    `json:"id,omitempty"`
	Username   string   `json:"username,omitempty"`
	FirstName  string   `json:"firstName,omitempty"`
	LastName   string   `json:"lastName,omitempty"`
	Email      string   `json:"email,omitempty"`
	Password   string   `json:"password,omitempty"`
	Phone      string   `json:"phone,omitempty"`
	UserStatus int32    `json:"userStatus,omitempty"` // User Status
}

type UserController struct {
}

func (c *UserController) CreateUser(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) CreateUsersWithArrayInput(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) CreateUsersWithListInput(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) GetUserByName(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) DeleteUser(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) LoginUser(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) LogoutUser(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *UserController) UpdateUser(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

type OrderController struct {
}

func (c *OrderController) DeleteOrder(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *OrderController) GetInventory(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *OrderController) GetOrderById(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *OrderController) PlaceOrder(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

type PetController struct {
}

func (c *PetController) AddPet(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) DeletePet(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) FindPetsByStatus(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) FindPetsByTags(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) GetPetById(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) UpdatePet(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) UpdatePetWithForm(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func (c *PetController) UploadFile(ctx web.Context) {
	ctx.SetContentType("application/json; charset=UTF-8")
	ctx.NoContent(http.StatusOK)
}

func main() {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration/>
	`
	err := log.RefreshBuffer(config, ".xml")
	util.Panic(err).When(err != nil)

	c := SpringEcho.New(web.ServerConfig{Port: 8080, BasePath: "/v2"})
	rootSW := swagger.Doc(c).
		WithDescription("This is a sample server Petstore server.  You can find out more about Swagger at [http://swagger.io](http://swagger.io) or on [irc.freenode.net, #swagger](http://swagger.io/irc/).  For this sample, you can use the api key `special-key` to test the authorization filters.").
		WithVersion("1.0.5").
		WithTitle("Swagger Petstore").
		WithTermsOfService("http://swagger.io/terms/").
		WithContact("", "", "apiteam@swagger.io").
		WithLicense("Apache 2.0", "http://www.apache.org/licenses/LICENSE-2.0.html").
		WithHost("petstore.swagger.io").
		WithBasePath("/v2").
		WithTags(
			spec.NewTag("pet", "Everything about your Pets", &spec.ExternalDocumentation{
				Description: "Find out more",
				URL:         "http://swagger.io",
			}),
			spec.NewTag("store", "Access to Petstore orders", nil),
			spec.NewTag("user", "Operations about user", &spec.ExternalDocumentation{
				Description: "Find out more about our store",
				URL:         "http://swagger.io",
			}),
		).
		WithSchemes("https", "http").
		WithExternalDocs(&spec.ExternalDocumentation{
			Description: "Find out more about Swagger",
			URL:         "http://swagger.io",
		}).
		BindDefinitions(new(ApiResponse), new(Tag), new(Category)).
		BindDefinitionWithTags(new(Order), map[string]SpringSwagger.DefinitionField{
			"status": {
				Enums:       []interface{}{"placed", "approved", "delivered"},
				Description: "Order Status",
			},
		}).
		BindDefinitionWithTags(new(User), map[string]SpringSwagger.DefinitionField{
			"userStatus": {
				Description: "User Status",
			},
		}).
		BindDefinitionWithTags(new(Pet), map[string]SpringSwagger.DefinitionField{
			"name": {
				Example: "doggie",
			},
			"status": {
				Description: "pet status in the store",
				Enums:       []interface{}{"available", "pending", "sold"},
			},
		}).
		AddApiKeySecurityDefinition("api_key", "header").
		AddOauth2ImplicitSecurityDefinition("petstore_auth",
			"https://petstore.swagger.io/oauth/authorize",
			map[string]string{
				"read:pets":  "read your pets",
				"write:pets": "modify pets in your account",
			})

	pet := new(PetController)
	{
		r := c.PostMapping("/pet/", pet.AddPet)
		swagger.Path(r).
			WithID("addPet").
			WithTags("pet").
			WithSummary("Add a new pet to the store").
			WithConsumes("application/json", "application/xml").
			WithProduces("application/json", "application/xml").
			BindParam(new(Pet), "Pet object that needs to be added to the store").
			RespondsWith(http.StatusMethodNotAllowed, SpringSwagger.NewResponse("Invalid input")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.PutMapping("/pet/", pet.UpdatePet)
		swagger.Path(r).
			WithID("updatePet").
			WithTags("pet").
			WithSummary("Update an existing pet").
			WithConsumes("application/json", "application/xml").
			WithProduces("application/json", "application/xml").
			BindParam(new(Pet), "Pet object that needs to be added to the store").
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid ID supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("Pet not found")).
			RespondsWith(http.StatusMethodNotAllowed, SpringSwagger.NewResponse("Validation exception")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.GetMapping("/pet/{petId}", pet.GetPetById)
		swagger.Path(r).
			WithID("getPetById").
			WithTags("pet").
			WithDescription("Returns a single pet").
			WithSummary("Find pet by ID").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("petId", "integer", "int64").WithDescription("ID of pet to return")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse(new(Pet), "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid ID supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("Pet not found")).
			SecuredWith("api_key", []string{}...)

		r = c.PostMapping("/pet/{petId}", pet.UpdatePetWithForm)
		swagger.Path(r).
			WithID("updatePetWithForm").
			WithTags("pet").
			WithSummary("Updates a pet in the store with form data").
			WithConsumes("application/x-www-form-urlencoded").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("petId", "integer", "int64").WithDescription("ID of pet that needs to be updated")).
			AddParam(spec.FormDataParam("name").Typed("string", "").WithDescription("Updated name of the pet")).
			AddParam(spec.FormDataParam("status").Typed("string", "").WithDescription("Updated status of the pet")).
			RespondsWith(http.StatusMethodNotAllowed, SpringSwagger.NewResponse("Invalid input")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.DeleteMapping("/pet/{petId}", pet.DeletePet)
		swagger.Path(r).
			WithID("deletePet").
			WithTags("pet").
			WithSummary("Deletes a pet").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.HeaderParam("api_key", "string", "").AsOptional()).
			AddParam(SpringSwagger.PathParam("petId", "integer", "int64").WithDescription("Pet id to delete")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid ID supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("Pet not found")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.PostMapping("/pet/{petId}/uploadImage", pet.UploadFile)
		swagger.Path(r).
			WithID("uploadFile").
			WithTags("pet").
			WithSummary("uploads an image").
			WithConsumes("multipart/form-data").
			WithProduces("application/json").
			AddParam(SpringSwagger.PathParam("petId", "integer", "int64").WithDescription("ID of pet to update")).
			AddParam(spec.FormDataParam("additionalMetadata").Typed("string", "").WithDescription("Additional data to pass to server")).
			AddParam(spec.FileParam("file").WithDescription("file to upload")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse(new(ApiResponse), "successful operation")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.GetMapping("/pet/findByStatus", pet.FindPetsByStatus)
		swagger.Path(r).
			WithID("findPetsByStatus").
			WithTags("pet").
			WithDescription("Multiple status values can be provided with comma separated strings").
			WithSummary("Finds Pets by status").
			WithProduces("application/json", "application/xml").
			AddParam(spec.QueryParam("status").
				AsRequired().
				WithDescription("Status values that need to be considered for filter").
				CollectionOf(spec.NewItems().Typed("string", "").WithDefault("available").WithEnum("available", "pending", "sold"), "multi")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse([]Pet{}, "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid status value")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")

		r = c.GetMapping("/pet/findByTags", pet.FindPetsByTags)
		swagger.Path(r).
			WithID("findPetsByTags").
			Deprecate().
			WithTags("pet").
			WithDescription("Multiple tags can be provided with comma separated strings. Use tag1, tag2, tag3 for testing.").
			WithSummary("Finds Pets by tags").
			WithProduces("application/json", "application/xml").
			AddParam(spec.QueryParam("tags").
				AsRequired().
				WithDescription("Tags to filter by").
				CollectionOf(spec.NewItems().Typed("string", ""), "multi")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse([]Pet{}, "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid tag value")).
			SecuredWith("petstore_auth", "write:pets", "read:pets")
	}

	order := new(OrderController)
	{
		r := c.PostMapping("/store/order", order.PlaceOrder)
		swagger.Path(r).
			WithID("placeOrder").
			WithTags("store").
			WithDescription("").
			WithSummary("Place an order for a pet").
			WithConsumes("application/json").
			WithProduces("application/json", "application/xml").
			BindParam(Order{}, "order placed for purchasing the pet").
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse(Order{}, "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid Order"))

		r = c.GetMapping("/store/order/{orderId}", order.GetOrderById)
		swagger.Path(r).
			WithID("getOrderById").
			WithTags("store").
			WithDescription("For valid response try integer IDs with value \u003e= 1 and \u003c= 10. Other values will generated exceptions").
			WithSummary("Find purchase order by ID").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("orderId", "integer", "int64").WithMaximum(10, false).WithMinimum(1, false).WithDescription("ID of pet that needs to be fetched")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse(Order{}, "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid ID supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("Order not found"))

		r = c.DeleteMapping("/store/order/{orderId}", order.DeleteOrder)
		swagger.Path(r).
			WithID("deleteOrder").
			WithTags("store").
			WithDescription("For valid response try integer IDs with positive integer value. Negative or non-integer values will generate API errors").
			WithSummary("Delete purchase order by ID").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("orderId", "integer", "int64").WithMinimum(1, false).WithDescription("ID of the order that needs to be deleted")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid ID supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("Order not found"))

		r = c.GetMapping("/store/inventory", order.GetInventory)
		swagger.Path(r).
			WithID("getInventory").
			WithTags("store").
			WithDescription("Returns a map of status codes to quantities").
			WithSummary("Returns pet inventories by status").
			WithProduces("application/json").
			RespondsWith(http.StatusOK, SpringSwagger.NewResponse("successful operation").WithSchema(spec.MapProperty(spec.Int32Property()))).
			SecuredWith("api_key", []string{}...)
	}

	user := new(UserController)
	{

		r := c.PostMapping("/user/", user.CreateUser)
		swagger.Path(r).
			WithID("createUser").
			WithTags("user").
			WithDescription("This can only be done by the logged in user.").
			WithSummary("Create user").
			WithConsumes("application/json").
			WithProduces("application/json", "application/xml").
			BindParam(User{}, "Created user object").
			WithDefaultResponse(spec.NewResponse().WithDescription("successful operation"))

		r = c.GetMapping("/user/{username}", user.GetUserByName)
		swagger.Path(r).
			WithID("getUserByName").
			WithTags("user").
			WithDescription("").
			WithSummary("Get user by user name").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("username", "string", "").WithDescription("The name that needs to be fetched. Use user1 for testing. ")).
			RespondsWith(http.StatusOK, SpringSwagger.NewBindResponse(User{}, "successful operation")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid username supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("User not found"))

		r = c.PutMapping("/user/{username}", user.UpdateUser)
		swagger.Path(r).
			WithID("updateUser").
			WithTags("user").
			WithDescription("This can only be done by the logged in user.").
			WithSummary("Updated user").
			WithConsumes("application/json").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("username", "string", "").WithDescription("name that need to be updated")).
			BindParam(User{}, "Updated user object").
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid user supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("User not found"))

		r = c.DeleteMapping("/user/{username}", user.DeleteUser)
		swagger.Path(r).
			WithID("deleteUser").
			WithTags("user").
			WithDescription("This can only be done by the logged in user.").
			WithSummary("Delete user").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.PathParam("username", "string", "").WithDescription("The name that needs to be deleted")).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid username supplied")).
			RespondsWith(http.StatusNotFound, SpringSwagger.NewResponse("User not found"))

		r = c.GetMapping("/user/login", user.LoginUser)
		swagger.Path(r).
			WithID("loginUser").
			WithTags("user").
			WithDescription("").
			WithSummary("Logs user into the system").
			WithProduces("application/json", "application/xml").
			AddParam(spec.QueryParam("username").
				AsRequired().
				WithDescription("The user name for login").
				Typed("string", "")).
			AddParam(spec.QueryParam("password").
				AsRequired().
				WithDescription("The password for login in clear text").
				Typed("string", "")).
			RespondsWith(http.StatusOK, SpringSwagger.NewResponse("successful operation").
				AddHeader("X-Expires-After", spec.ResponseHeader().
					WithDescription("date in UTC when token expires").
					Typed("string", "date-time")).
				AddHeader("X-Rate-Limit", spec.ResponseHeader().
					WithDescription("calls per hour allowed by the user").
					Typed("integer", "int32")).
				WithSchema(spec.StringProperty())).
			RespondsWith(http.StatusBadRequest, SpringSwagger.NewResponse("Invalid username/password supplied"))

		r = c.GetMapping("/user/logout", user.LogoutUser)
		swagger.Path(r).
			WithID("logoutUser").
			WithTags("user").
			WithDescription("").
			WithSummary("Logs out current logged in user session").
			WithProduces("application/json", "application/xml").
			WithDefaultResponse(SpringSwagger.NewResponse("successful operation"))

		r = c.PostMapping("/user/createWithArray", user.CreateUsersWithArrayInput)
		swagger.Path(r).
			WithID("createUsersWithArrayInput").
			WithTags("user").
			WithDescription("").
			WithSummary("Creates list of users with given input array").
			WithConsumes("application/json").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.BodyParam("body", spec.ArrayProperty(spec.RefSchema("#/definitions/User"))).
				AsRequired().
				WithDescription("List of user object")).
			WithDefaultResponse(SpringSwagger.NewResponse("successful operation"))

		r = c.PostMapping("/user/createWithList", user.CreateUsersWithListInput)
		swagger.Path(r).
			WithID("createUsersWithListInput").
			WithTags("user").
			WithDescription("").
			WithSummary("Creates list of users with given input array").
			WithConsumes("application/json").
			WithProduces("application/json", "application/xml").
			AddParam(SpringSwagger.BodyParam("body", spec.ArrayProperty(spec.RefSchema("#/definitions/User"))).
				AsRequired().
				WithDescription("List of user object")).
			WithDefaultResponse(SpringSwagger.NewResponse("successful operation"))
	}

	go func() {
		c.Start()
	}()

	time.Sleep(500 * time.Millisecond)

	var m1 map[string]interface{}
	_ = json.Unmarshal([]byte(petstore), &m1)

	var m2 map[string]interface{}
	doc := rootSW.ReadDoc()
	_ = json.Unmarshal([]byte(doc), &m2)

	setDef(m1)
	setDef(m2)

	DiffMap("", m1, m2)

	// TODO 验证 XML Wrapper 语法
	fmt.Println(cast.ToString(m1))
	fmt.Println(cast.ToString(m2))
}

var petstore = `{"swagger":"2.0","info":{"description":"This is a sample server Petstore server.  You can find out more about Swagger at [http://swagger.io](http://swagger.io) or on [irc.freenode.net, #swagger](http://swagger.io/irc/).  For this sample, you can use the api key ` + "`special-key`" + ` to test the authorization filters.","version":"1.0.5","title":"Swagger Petstore","termsOfService":"http://swagger.io/terms/","contact":{"email":"apiteam@swagger.io"},"license":{"name":"Apache 2.0","url":"http://www.apache.org/licenses/LICENSE-2.0.html"}},"host":"petstore.swagger.io","basePath":"/v2","tags":[{"name":"pet","description":"Everything about your Pets","externalDocs":{"description":"Find out more","url":"http://swagger.io"}},{"name":"store","description":"Access to Petstore orders"},{"name":"user","description":"Operations about user","externalDocs":{"description":"Find out more about our store","url":"http://swagger.io"}}],"schemes":["https","http"],"paths":{"/pet/{petId}/uploadImage":{"post":{"tags":["pet"],"summary":"uploads an image","description":"","operationId":"uploadFile","consumes":["multipart/form-data"],"produces":["application/json"],"parameters":[{"name":"petId","in":"path","description":"ID of pet to update","required":true,"type":"integer","format":"int64"},{"name":"additionalMetadata","in":"formData","description":"Additional data to pass to server","required":false,"type":"string"},{"name":"file","in":"formData","description":"file to upload","required":false,"type":"file"}],"responses":{"200":{"description":"successful operation","schema":{"$ref":"#/definitions/ApiResponse"}}},"security":[{"petstore_auth":["write:pets","read:pets"]}]}},"/pet":{"post":{"tags":["pet"],"summary":"Add a new pet to the store","description":"","operationId":"addPet","consumes":["application/json","application/xml"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"Pet object that needs to be added to the store","required":true,"schema":{"$ref":"#/definitions/Pet"}}],"responses":{"405":{"description":"Invalid input"}},"security":[{"petstore_auth":["write:pets","read:pets"]}]},"put":{"tags":["pet"],"summary":"Update an existing pet","description":"","operationId":"updatePet","consumes":["application/json","application/xml"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"Pet object that needs to be added to the store","required":true,"schema":{"$ref":"#/definitions/Pet"}}],"responses":{"400":{"description":"Invalid ID supplied"},"404":{"description":"Pet not found"},"405":{"description":"Validation exception"}},"security":[{"petstore_auth":["write:pets","read:pets"]}]}},"/pet/findByStatus":{"get":{"tags":["pet"],"summary":"Finds Pets by status","description":"Multiple status values can be provided with comma separated strings","operationId":"findPetsByStatus","produces":["application/json","application/xml"],"parameters":[{"name":"status","in":"query","description":"Status values that need to be considered for filter","required":true,"type":"array","items":{"type":"string","enum":["available","pending","sold"],"default":"available"},"collectionFormat":"multi"}],"responses":{"200":{"description":"successful operation","schema":{"type":"array","items":{"$ref":"#/definitions/Pet"}}},"400":{"description":"Invalid status value"}},"security":[{"petstore_auth":["write:pets","read:pets"]}]}},"/pet/findByTags":{"get":{"tags":["pet"],"summary":"Finds Pets by tags","description":"Multiple tags can be provided with comma separated strings. Use tag1, tag2, tag3 for testing.","operationId":"findPetsByTags","produces":["application/json","application/xml"],"parameters":[{"name":"tags","in":"query","description":"Tags to filter by","required":true,"type":"array","items":{"type":"string"},"collectionFormat":"multi"}],"responses":{"200":{"description":"successful operation","schema":{"type":"array","items":{"$ref":"#/definitions/Pet"}}},"400":{"description":"Invalid tag value"}},"security":[{"petstore_auth":["write:pets","read:pets"]}],"deprecated":true}},"/pet/{petId}":{"get":{"tags":["pet"],"summary":"Find pet by ID","description":"Returns a single pet","operationId":"getPetById","produces":["application/json","application/xml"],"parameters":[{"name":"petId","in":"path","description":"ID of pet to return","required":true,"type":"integer","format":"int64"}],"responses":{"200":{"description":"successful operation","schema":{"$ref":"#/definitions/Pet"}},"400":{"description":"Invalid ID supplied"},"404":{"description":"Pet not found"}},"security":[{"api_key":[]}]},"post":{"tags":["pet"],"summary":"Updates a pet in the store with form data","description":"","operationId":"updatePetWithForm","consumes":["application/x-www-form-urlencoded"],"produces":["application/json","application/xml"],"parameters":[{"name":"petId","in":"path","description":"ID of pet that needs to be updated","required":true,"type":"integer","format":"int64"},{"name":"name","in":"formData","description":"Updated name of the pet","required":false,"type":"string"},{"name":"status","in":"formData","description":"Updated status of the pet","required":false,"type":"string"}],"responses":{"405":{"description":"Invalid input"}},"security":[{"petstore_auth":["write:pets","read:pets"]}]},"delete":{"tags":["pet"],"summary":"Deletes a pet","description":"","operationId":"deletePet","produces":["application/json","application/xml"],"parameters":[{"name":"api_key","in":"header","required":false,"type":"string"},{"name":"petId","in":"path","description":"Pet id to delete","required":true,"type":"integer","format":"int64"}],"responses":{"400":{"description":"Invalid ID supplied"},"404":{"description":"Pet not found"}},"security":[{"petstore_auth":["write:pets","read:pets"]}]}},"/store/order":{"post":{"tags":["store"],"summary":"Place an order for a pet","description":"","operationId":"placeOrder","consumes":["application/json"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"order placed for purchasing the pet","required":true,"schema":{"$ref":"#/definitions/Order"}}],"responses":{"200":{"description":"successful operation","schema":{"$ref":"#/definitions/Order"}},"400":{"description":"Invalid Order"}}}},"/store/order/{orderId}":{"get":{"tags":["store"],"summary":"Find purchase order by ID","description":"For valid response try integer IDs with value >= 1 and <= 10. Other values will generated exceptions","operationId":"getOrderById","produces":["application/json","application/xml"],"parameters":[{"name":"orderId","in":"path","description":"ID of pet that needs to be fetched","required":true,"type":"integer","maximum":10,"minimum":1,"format":"int64"}],"responses":{"200":{"description":"successful operation","schema":{"$ref":"#/definitions/Order"}},"400":{"description":"Invalid ID supplied"},"404":{"description":"Order not found"}}},"delete":{"tags":["store"],"summary":"Delete purchase order by ID","description":"For valid response try integer IDs with positive integer value. Negative or non-integer values will generate API errors","operationId":"deleteOrder","produces":["application/json","application/xml"],"parameters":[{"name":"orderId","in":"path","description":"ID of the order that needs to be deleted","required":true,"type":"integer","minimum":1,"format":"int64"}],"responses":{"400":{"description":"Invalid ID supplied"},"404":{"description":"Order not found"}}}},"/store/inventory":{"get":{"tags":["store"],"summary":"Returns pet inventories by status","description":"Returns a map of status codes to quantities","operationId":"getInventory","produces":["application/json"],"parameters":[],"responses":{"200":{"description":"successful operation","schema":{"type":"object","additionalProperties":{"type":"integer","format":"int32"}}}},"security":[{"api_key":[]}]}},"/user/createWithArray":{"post":{"tags":["user"],"summary":"Creates list of users with given input array","description":"","operationId":"createUsersWithArrayInput","consumes":["application/json"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"List of user object","required":true,"schema":{"type":"array","items":{"$ref":"#/definitions/User"}}}],"responses":{"default":{"description":"successful operation"}}}},"/user/createWithList":{"post":{"tags":["user"],"summary":"Creates list of users with given input array","description":"","operationId":"createUsersWithListInput","consumes":["application/json"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"List of user object","required":true,"schema":{"type":"array","items":{"$ref":"#/definitions/User"}}}],"responses":{"default":{"description":"successful operation"}}}},"/user/{username}":{"get":{"tags":["user"],"summary":"Get user by user name","description":"","operationId":"getUserByName","produces":["application/json","application/xml"],"parameters":[{"name":"username","in":"path","description":"The name that needs to be fetched. Use user1 for testing. ","required":true,"type":"string"}],"responses":{"200":{"description":"successful operation","schema":{"$ref":"#/definitions/User"}},"400":{"description":"Invalid username supplied"},"404":{"description":"User not found"}}},"put":{"tags":["user"],"summary":"Updated user","description":"This can only be done by the logged in user.","operationId":"updateUser","consumes":["application/json"],"produces":["application/json","application/xml"],"parameters":[{"name":"username","in":"path","description":"name that need to be updated","required":true,"type":"string"},{"in":"body","name":"body","description":"Updated user object","required":true,"schema":{"$ref":"#/definitions/User"}}],"responses":{"400":{"description":"Invalid user supplied"},"404":{"description":"User not found"}}},"delete":{"tags":["user"],"summary":"Delete user","description":"This can only be done by the logged in user.","operationId":"deleteUser","produces":["application/json","application/xml"],"parameters":[{"name":"username","in":"path","description":"The name that needs to be deleted","required":true,"type":"string"}],"responses":{"400":{"description":"Invalid username supplied"},"404":{"description":"User not found"}}}},"/user/login":{"get":{"tags":["user"],"summary":"Logs user into the system","description":"","operationId":"loginUser","produces":["application/json","application/xml"],"parameters":[{"name":"username","in":"query","description":"The user name for login","required":true,"type":"string"},{"name":"password","in":"query","description":"The password for login in clear text","required":true,"type":"string"}],"responses":{"200":{"description":"successful operation","headers":{"X-Expires-After":{"type":"string","format":"date-time","description":"date in UTC when token expires"},"X-Rate-Limit":{"type":"integer","format":"int32","description":"calls per hour allowed by the user"}},"schema":{"type":"string"}},"400":{"description":"Invalid username/password supplied"}}}},"/user/logout":{"get":{"tags":["user"],"summary":"Logs out current logged in user session","description":"","operationId":"logoutUser","produces":["application/json","application/xml"],"parameters":[],"responses":{"default":{"description":"successful operation"}}}},"/user":{"post":{"tags":["user"],"summary":"Create user","description":"This can only be done by the logged in user.","operationId":"createUser","consumes":["application/json"],"produces":["application/json","application/xml"],"parameters":[{"in":"body","name":"body","description":"Created user object","required":true,"schema":{"$ref":"#/definitions/User"}}],"responses":{"default":{"description":"successful operation"}}}}},"securityDefinitions":{"api_key":{"type":"apiKey","name":"api_key","in":"header"},"petstore_auth":{"type":"oauth2","authorizationUrl":"https://petstore.swagger.io/oauth/authorize","flow":"implicit","scopes":{"read:pets":"read your pets","write:pets":"modify pets in your account"}}},"definitions":{"ApiResponse":{"type":"object","properties":{"code":{"type":"integer","format":"int32"},"type":{"type":"string"},"message":{"type":"string"}}},"Category":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"name":{"type":"string"}},"xml":{"name":"Category"}},"Pet":{"type":"object","required":["name","photoUrls"],"properties":{"id":{"type":"integer","format":"int64"},"category":{"$ref":"#/definitions/Category"},"name":{"type":"string","example":"doggie"},"photoUrls":{"type":"array","xml":{"wrapped":true},"items":{"type":"string","xml":{"name":"photoUrl"}}},"tags":{"type":"array","xml":{"wrapped":true},"items":{"xml":{"name":"tag"},"$ref":"#/definitions/Tag"}},"status":{"type":"string","description":"pet status in the store","enum":["available","pending","sold"]}},"xml":{"name":"Pet"}},"Tag":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"name":{"type":"string"}},"xml":{"name":"Tag"}},"Order":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"petId":{"type":"integer","format":"int64"},"quantity":{"type":"integer","format":"int32"},"shipDate":{"type":"string","format":"date-time"},"status":{"type":"string","description":"Order Status","enum":["placed","approved","delivered"]},"complete":{"type":"boolean"}},"xml":{"name":"Order"}},"User":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"username":{"type":"string"},"firstName":{"type":"string"},"lastName":{"type":"string"},"email":{"type":"string"},"password":{"type":"string"},"phone":{"type":"string"},"userStatus":{"type":"integer","format":"int32","description":"User Status"}},"xml":{"name":"User"}}},"externalDocs":{"description":"Find out more about Swagger","url":"http://swagger.io"}}`

func setDef(m map[string]interface{}) {
	for k, v := range m {
		switch v0 := v.(type) {
		case bool:
			if !v0 {
				delete(m, k)
			}
		case string:
			if v0 == "" {
				delete(m, k)
			}
		case map[string]interface{}:
			setDef(v0)
		default:
			if reflect.TypeOf(v0).Kind() == reflect.Slice {
				if rv := reflect.ValueOf(v0); rv.Len() == 0 {
					delete(m, k)
				} else {
					for i := 0; i < rv.Len(); i++ {
						iv := rv.Index(i).Interface()
						if mv, ok := iv.(map[string]interface{}); ok {
							setDef(mv)
						}
					}
				}
			}
		}
	}
}

// Diff 比较任意两个变量的内容是否相同
func Diff(path string, v1 interface{}, v2 interface{}) {

	m1, ok1 := v1.(map[string]interface{})
	m2, ok2 := v2.(map[string]interface{})
	if ok1 || ok2 {
		DiffMap(path, m1, m2)
		return
	}

	ma1, ok1 := v1.([]map[string]interface{})
	ma2, ok2 := v2.([]map[string]interface{})
	if ok1 || ok2 {
		DiffMapArray(path, ma1, ma2)
		return
	}

	a1, ok1 := v1.([]interface{})
	a2, ok2 := v2.([]interface{})
	if ok1 || ok2 {
		DiffArray(path, a1, a2)
		return
	}

	s1, ok1 := DefaultString(v1)
	s2, ok2 := DefaultString(v2)
	if (ok1 && ok2) && (s1 == s2) {
		return
	}

	b1, ok1 := DefaultBool(v1)
	b2, ok2 := DefaultBool(v2)
	if (ok1 && ok2) && (b1 == b2) {
		return
	}

	if !reflect.DeepEqual(v1, v2) {
		fmt.Printf("%s %v -> %v\n", path, v1, v2)
	}
}

// DiffMap 比较两个 map 的内容是否相同
func DiffMap(path string, v1 map[string]interface{}, v2 map[string]interface{}) {
	if len(v1) > len(v2) {
		for k := range v1 {
			Diff(path+"/"+k, v1[k], v2[k])
		}
	} else {
		for k := range v2 {
			Diff(path+"/"+k, v1[k], v2[k])
		}
	}
}

// DiffMapArray 比较两个 map 数组的内容是否相同，顺序也必须相同
func DiffMapArray(path string, v1 []map[string]interface{}, v2 []map[string]interface{}) {
	if len(v1) != len(v2) {
		fmt.Printf("%s %v -> %v\n", path, v1, v2)
	}
	for i := range v1 {
		DiffMap(path, v1[i], v2[i])
	}
}

// DiffArray 比较两个数组的内容是否相同，顺序也必须相同
func DiffArray(path string, v1 []interface{}, v2 []interface{}) {
	if len(v1) != len(v2) {
		fmt.Printf("%s %v -> %v\n", path, v1, v2)
	}
	for i := range v1 {
		Diff(path, v1[i], v2[i])
	}
}

// DefaultBool 将 nil 转换成 false 布尔值
func DefaultBool(v interface{}) (bool, bool) {
	if v == nil {
		return false, true
	}
	s, ok := v.(bool)
	return s, ok
}

// DefaultString 将 nil 转换成空字符串
func DefaultString(v interface{}) (string, bool) {
	if v == nil {
		return "", true
	}
	s, ok := v.(string)
	return s, ok
}
