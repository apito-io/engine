package utility

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/oas"
	"github.com/jinzhu/inflection"
)

func ModelSchemaBuilder(modelType *protobuff.ModelType, update bool) *oas.Schema {
	schema := oas.Props{}
	var required []string
	for _, f := range modelType.Fields {
		if f.Validation != nil && (f.Validation.IsPassword || f.Validation.Hide) && !update { // skip password field
			continue
		}
		switch f.InputType {
		case "string":
			schema[f.Identifier] = oas.String()
			break
		case "int":
			schema[f.Identifier] = oas.Integer()
			break
		case "double":
			schema[f.Identifier] = oas.Float()
			break
		case "geo":
			schema[f.Identifier] = oas.ObjectOf(oas.Props{
				"lat": oas.Float(),
				"lon": oas.Float(),
			})
			break
		case "repeated":
			var fields []*protobuff.FieldInfo
			for _, f := range f.SubFieldInfo {
				fields = append(fields, &protobuff.FieldInfo{
					Identifier:      f.Identifier,
					Description:     f.Description,
					InputType:       f.InputType,
					FieldType:       f.FieldType,
					Validation:      f.Validation,
					Serial:          f.Serial,
					Label:           f.Label,
					SystemGenerated: f.SystemGenerated,
				})
			}
			if update {
				fields = append(fields, &protobuff.FieldInfo{
					Identifier:  "_id",
					Description: "Id of the nested item",
					InputType:   "string",
					FieldType:   "text",
					Validation:  &protobuff.Validation{Required: true, Unique: true},
					//Serial:                  0,
					Label:                   "ID",
					SystemGenerated:         true,
					RepeatedGroupIdentifier: f.Identifier,
				})
			}
			fakeModel := &protobuff.ModelType{Fields: fields}
			schema[f.Identifier] = oas.ItemsOf(ModelSchemaBuilder(fakeModel, update))
		case "bool":
			schema[f.Identifier] = oas.Boolean()
			break
		}
		if f.Validation != nil && f.Validation.Required {
			required = append(required, f.Identifier)
		}
	}

	if len(modelType.Connections) > 0 {
		connections := make(oas.Props)
		for _, c := range modelType.Connections {
			switch c.Relation {
			case "has_many":
				connections[c.Model+"_ids"] = oas.ItemsOf(oas.String())
				break
			case "has_one":
				connections[c.Model+"_id"] = oas.String()
				break
			}
		}
		schema["_connections"] = oas.ObjectOf(connections)
	}

	return oas.ObjectOf(schema, required...)
}

func OpenApiSpecGenerator(env string, loader *shared.ApplicationCache) (interface{}, error) {

	project := loader.Project

	openapi := oas.NewOpenAPI()
	openapi.Version = "1.0.0"
	openapi.Title = project.Name
	openapi.Description = project.Description
	/*	openapi.License = &oas.License{
		LicenseObject: oas.LicenseObject{
			Name: "MIT",
		},
	}*/

	switch env {
	case "local":
		openapi.AddServer(oas.NewServer("http://localhost:5050/secured/rest/" + project.Id))
	default:
		openapi.AddServer(oas.NewServer("https://api.apito.io/secured/rest/" + project.Id))
	}

	openapi.AddSecurityScheme("api_key", oas.NewAPIKeySecurityScheme("Authorization", "Go to <strong>Settings > API Secrets</strong> and Generate an api key to be used in Authorize. Value should be `Bearer <token>`", "header"))

	if project.Schema == nil {
		return openapi, nil
	}

	for _, model := range project.Schema.Models {

		singularName := strings.Title(model.Name)
		pluralName := strings.Title(inflection.Plural(singularName))

		openapi.AddTag(oas.NewTag(pluralName))

		openapi.AddSchema(singularName, ModelSchemaBuilder(model, false))
		openapi.AddSchema(pluralName, oas.ItemsOf(openapi.RefSchema(singularName)))

		openapi.AddSchema("Error", oas.ObjectOf(oas.Props{
			"code":    oas.Integer(),
			"message": oas.String(),
		}, "code", "message"))

		// list
		{
			op := oas.NewOperation(fmt.Sprintf(`list%s`, pluralName))
			op.Summary = fmt.Sprintf(`List all %s`, pluralName)
			op.Tags = []string{pluralName}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			queryField := oas.QueryParameter("query", oas.String(), false).
				WithDesc(`{"title:contains": "fahim", "quantify" : 20} The Query that you want to run.`)
			/*fieldFields := oas.QueryParameter("fields", oas.String(), false).
			WithDesc(`["name", "title"] The Fields that you want to see in the Response JSON. Leve this blank if you want to see all fields`)*/
			pageFields := oas.QueryParameter("page", oas.Integer(), false).
				WithDesc(`Pagination Support`)
			limitFields := oas.QueryParameter("limit", oas.Integer(), false).
				WithDesc(`Limit the number of resources`)
			localFields := oas.QueryParameter("local", oas.String(), false).
				WithDesc(`Query Data with Locals`)
			metaFields := oas.QueryParameter("meta", oas.Boolean(), false).
				WithDesc(`Whether not to include meta in the result`)

			op.AddParameter(queryField)
			//op.AddParameter(fieldFields)
			op.AddParameter(pageFields)
			op.AddParameter(localFields)
			op.AddParameter(limitFields)
			op.AddParameter(metaFields)

			{
				resp := oas.NewResponse(fmt.Sprintf(`An paged array of %s`, pluralName))
				/*				s := oas.String()
								s.Description = "A link to the next page of responses"
								resp.AddHeader("x-next", oas.NewHeaderWithSchema(s))*/
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(pluralName)))
				op.AddResponse(http.StatusOK, resp)
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))

				op.SetDefaultResponse(resp)
			}

			openapi.AddOperation(oas.GET, fmt.Sprintf(`/%s`, inflection.Plural(model.Name)), op)
		}

		// single data
		{
			op := oas.NewOperation(fmt.Sprintf(`get%s`, singularName))
			op.Summary = fmt.Sprintf(`Get Single %s`, singularName)
			op.Tags = []string{pluralName}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			idPath := oas.PathParameter("id", oas.String()).
				WithDesc(`The ID of the document that you want to fetch`)
			op.AddParameter(idPath)

			{
				resp := oas.NewResponse(fmt.Sprintf(`An Object of %s`, singularName))
				/*				s := oas.String()
								s.Description = "A link to the next page of responses"
								resp.AddHeader("x-next", oas.NewHeaderWithSchema(s))*/
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName)))
				op.AddResponse(http.StatusOK, resp)
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))

				op.SetDefaultResponse(resp)
			}

			openapi.AddOperation(oas.GET, fmt.Sprintf(`/%s/{id}`, model.Name), op)
		}

		// create
		{
			op := oas.NewOperation(fmt.Sprintf(`create%s`, singularName))
			op.Summary = fmt.Sprintf(`Creates a %s`, singularName)
			op.Tags = []string{pluralName}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			bodyField := oas.NewRequestBody(fmt.Sprintf(`The JSON of %s that you want to create`, singularName), true)
			bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName)))
			op.SetRequestBody(bodyField)

			// 201
			{
				resp := oas.NewResponse(fmt.Sprintf(`An Object of %s`, singularName))
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName)))
				op.AddResponse(http.StatusCreated, resp)
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
				op.SetDefaultResponse(resp)
			}
			openapi.AddOperation(oas.PUT, fmt.Sprintf("/%s", inflection.Plural(model.Name)), op)
		}

		// update
		{
			op := oas.NewOperation(fmt.Sprintf(`update%s`, singularName))
			op.Summary = fmt.Sprintf(`Updates a %s`, singularName)
			op.Tags = []string{pluralName}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			// special build
			openapi.AddSchema(singularName+"Update", ModelSchemaBuilder(model, true))

			idField := oas.QueryParameter("_id", oas.String(), true).
				WithDesc(`The ID of the Document that you want to Update`)
			bodyField := oas.NewRequestBody(fmt.Sprintf(`The JSON of %s that you want to create`, singularName), true)
			bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName+"Update")))
			op.AddParameter(idField)
			op.SetRequestBody(bodyField)

			// 200
			{
				resp := oas.NewResponse(fmt.Sprintf(`An Object of %s`, singularName))
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName)))
				op.AddResponse(http.StatusOK, resp)
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
				op.SetDefaultResponse(resp)
			}
			openapi.AddOperation(oas.POST, fmt.Sprintf("/%s", inflection.Plural(model.Name)), op)
		}

		// delete
		{
			op := oas.NewOperation(fmt.Sprintf(`delete%s`, singularName))
			op.Summary = fmt.Sprintf(`Deletes a %s`, singularName)
			op.Tags = []string{pluralName}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			queryField := oas.QueryParameter("_id", oas.String(), false).
				WithDesc(`The ID of the Doc`)
			op.AddParameter(queryField)

			// 200
			{
				resp := oas.NewResponse(fmt.Sprintf(`An Object of %s`, singularName))
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(singularName)))
				op.AddResponse(http.StatusOK, resp)
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
				op.SetDefaultResponse(resp)
			}
			openapi.AddOperation(oas.DELETE, fmt.Sprintf("/%s", inflection.Plural(model.Name)), op)
		}

	}

	for _, model := range project.Schema.Models {

		singularName := strings.Title(model.Name)
		pluralName := strings.Title(inflection.Plural(singularName))

		// check if this model has relations
		if len(model.Connections) > 0 {
			for _, c := range model.Connections {

				var schemaName string
				var connectedModel string
				if c.Relation == "has_many" {
					connectedModel = strings.Title(inflection.Plural(c.Model))
					schemaName = fmt.Sprintf("%s-%s-response", model.Name, strings.ToLower(inflection.Plural(connectedModel)))
					openapi.AddSchema(schemaName, oas.ObjectOf(oas.Props{
						model.Name: openapi.RefSchema(singularName),
						strings.ToLower(inflection.Plural(connectedModel)): oas.ItemsOf(openapi.RefSchema(connectedModel)),
					}))
				} else {
					connectedModel = strings.Title(c.Model)
					schemaName = fmt.Sprintf("%s-%s-response", model.Name, strings.ToLower(connectedModel))
					openapi.AddSchema(schemaName, oas.ObjectOf(oas.Props{
						model.Name:                      openapi.RefSchema(singularName),
						strings.ToLower(connectedModel): openapi.RefSchema(connectedModel),
					}))
				}

				// list relations
				{
					op := oas.NewOperation(fmt.Sprintf(`list%s`, connectedModel))
					op.Summary = fmt.Sprintf(`List all %s`, connectedModel)
					op.Tags = []string{pluralName}
					op.AddSecurityRequirement(&oas.SecurityRequirement{
						"api_key": []string{},
					})

					idPath := oas.PathParameter("id", oas.String()).
						WithDesc(`The ID of the document that you want to fetch`)
					op.AddParameter(idPath)

					queryField := oas.QueryParameter("query", oas.String(), false).
						WithDesc(`{"title:contains": "fahim", "quantify" : 20} The Query that you want to run.`)
					/*fieldFields := oas.QueryParameter("fields", oas.String(), false).
					WithDesc(`["name", "title"] The Fields that you want to see in the Response JSON. Leve this blank if you want to see all fields`)*/
					pageFields := oas.QueryParameter("page", oas.Integer(), false).
						WithDesc(`Pagination Support`)
					limitFields := oas.QueryParameter("limit", oas.Integer(), false).
						WithDesc(`Limit the number of resources`)
					localFields := oas.QueryParameter("local", oas.String(), false).
						WithDesc(`Query Data with Locals`)
					metaFields := oas.QueryParameter("meta", oas.Boolean(), false).
						WithDesc(`Whether not to include meta in the result`)

					op.AddParameter(queryField)
					//op.AddParameter(fieldFields)
					op.AddParameter(pageFields)
					op.AddParameter(localFields)
					op.AddParameter(limitFields)
					op.AddParameter(metaFields)

					{
						resp := oas.NewResponse(fmt.Sprintf(`An paged array of %s`, connectedModel))
						/*				s := oas.String()
										s.Description = "A link to the next page of responses"
										resp.AddHeader("x-next", oas.NewHeaderWithSchema(s))*/
						resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(schemaName)))
						op.AddResponse(http.StatusOK, resp)
					}

					{
						resp := oas.NewResponse("unexpected error")
						resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))

						op.SetDefaultResponse(resp)
					}

					openapi.AddOperation(oas.GET, fmt.Sprintf(`/%s/{id}/%s`, model.Name, strings.ToLower(connectedModel)), op)
				}
			}
		}
	}

	for _, function := range project.Schema.Functions {

		openapi.AddTag(oas.NewTag("Functions"))
		openapi.AddSchema("Error", oas.ObjectOf(oas.Props{
			"code":    oas.Integer(),
			"message": oas.String(),
		}, "code", "message"))

		// post
		{
			request := strings.Title(function.Request.Model)
			response := strings.Title(function.Response.Model)

			op := oas.NewOperation(fmt.Sprintf(`%s`, function.Name))
			op.Summary = fmt.Sprintf(`%s`, function.Description)
			op.Tags = []string{"Functions"}
			op.AddSecurityRequirement(&oas.SecurityRequirement{
				"api_key": []string{},
			})

			if request == "JSON" {
				bodyField := oas.NewRequestBody(fmt.Sprintf(`Model %s as Request Object`, request), true)
				bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(oas.ObjectOf(oas.Props{
					"_replace_or_remove_this":        oas.String(),
					"_with_any_json_object_you_like": oas.String(),
				})))
				op.SetRequestBody(bodyField)

				// 200
				{
					resp := oas.NewResponse(fmt.Sprintf(`Model %s as Response Object`, response))
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(oas.ObjectOf(oas.Props{
						"_you_can_return_any":  oas.String(),
						"_valid_JSON_response": oas.String(),
					})))
					op.AddResponse(http.StatusOK, resp)
				}
			} else {
				bodyField := oas.NewRequestBody(fmt.Sprintf(`Model %s as Request Object`, request), true)
				bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(request)))
				op.SetRequestBody(bodyField)

				// 200
				{
					resp := oas.NewResponse(fmt.Sprintf(`Model %s as Response Object`, response))
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema(response)))
					op.AddResponse(http.StatusOK, resp)
				}
			}

			{
				resp := oas.NewResponse("unexpected error")
				resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
				op.SetDefaultResponse(resp)
			}
			openapi.AddOperation(oas.POST, fmt.Sprintf("/system/function/%s", inflection.Plural(function.Name)), op)
		}
	}

	// Deprecated now that Auth is separated by Extension
	/*if project.Extensions != nil {
		auth := project.Extensions["auth"]
		if auth != nil && auth.Enable {
			openapi.AddTag(oas.NewTag("Authentication"))
			openapi.AddSchema("auth", oas.ObjectOf(oas.Props{
				auth.SubType: oas.String(),
				"secret":     oas.String(),
			}))

			openapi.AddSchema("authResponse", oas.ObjectOf(oas.Props{
				"id_token":      oas.String(),
				"refresh_token": oas.String(),
			}))

			// login
			{
				op := oas.NewOperation(`userLogin`)
				op.Summary = fmt.Sprintf(`Login API for a Apito Project User`)
				op.Tags = []string{"Authentication"}
				op.AddSecurityRequirement(&oas.SecurityRequirement{
					"api_key": []string{},
				})

				bodyField := oas.NewRequestBody(fmt.Sprintf(`The JSON of the login Details`), true)
				bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("auth")))
				op.SetRequestBody(bodyField)

				// 200
				{
					resp := oas.NewResponse(fmt.Sprintf(`An Object of user`))
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("authResponse")))
					op.AddResponse(http.StatusOK, resp)
				}

				{
					resp := oas.NewResponse("unexpected error")
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
					op.SetDefaultResponse(resp)
				}
				openapi.AddOperation(oas.POST, fmt.Sprintf("/system/auth/login"), op)
			}

			// register
			{
				op := oas.NewOperation(`userRegister`)
				op.Summary = fmt.Sprintf(`Register API for a Apito Project User`)
				op.Tags = []string{"Authentication"}
				op.AddSecurityRequirement(&oas.SecurityRequirement{
					"api_key": []string{},
				})

				bodyField := oas.NewRequestBody(fmt.Sprintf(`The JSON of the User Details`), true)
				bodyField.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("auth")))
				op.SetRequestBody(bodyField)

				// 200
				{
					resp := oas.NewResponse(fmt.Sprintf(`An Object of user Initial Credential`))
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("authResponse")))
					op.AddResponse(http.StatusOK, resp)
				}

				{
					resp := oas.NewResponse("unexpected error")
					resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
					op.SetDefaultResponse(resp)
				}
				openapi.AddOperation(oas.POST, fmt.Sprintf("/system/auth/register"), op)
			}

		}
	}*/

	openapi.AddTag(oas.NewTag("System Media Upload"))
	openapi.AddSchema("fileUpload", oas.ObjectOf(oas.Props{
		"file":       oas.Binary(),
		"model":      oas.String().WithDesc("(Optional) Use this if you want to this image to be under your particular model"),
		"id":         oas.String().WithDesc("(Optional) The Document Id of which this image will belong to. If using this parameter then you must fill `model` & `field_name` field"),
		"field_name": oas.String().WithDesc("(Optional) Under model which field this image will belong to"),
	}))

	// common media uplaod rest API
	{
		op := oas.NewOperation(`upload`)
		op.Summary = fmt.Sprintf(`Uploads a File to your current project`)
		op.Tags = []string{"System Media Upload"}
		op.AddSecurityRequirement(&oas.SecurityRequirement{
			"api_key": []string{},
		})

		bodyField := oas.NewRequestBody(fmt.Sprintf("The FILE that you want to upload. Required field is file. `model`, `id`, `field_name` those 3 parameter is optional but if you use them, then all 3 has to be used together"), true)
		bodyField.AddContent("multipart/form-data", oas.NewMediaTypeWithSchema(openapi.RefSchema("fileUpload")))
		op.SetRequestBody(bodyField)

		// 200
		{
			resp := oas.NewResponse(fmt.Sprintf(`An Object of user`))
			resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("user")))
			op.AddResponse(http.StatusOK, resp)
		}

		{
			resp := oas.NewResponse("unexpected error")
			resp.AddContent("application/json", oas.NewMediaTypeWithSchema(openapi.RefSchema("Error")))
			op.SetDefaultResponse(resp)
		}
		openapi.AddOperation(oas.POST, fmt.Sprintf("/system/media/upload"), op)
	}

	return openapi, nil
}
