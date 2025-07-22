package utility

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/iancoleman/strcase"
	"github.com/labstack/echo/v4"
)

func contains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

type RESTtoGraphResponse struct {
	Query         string `json:"query"`
	QueryName     string `json:"query_name"`
	OperationName string `json:"operation_name"`
}

func getFieldsInfo(fields map[string]interface{}, _fields []*models.FieldInfo) map[string]interface{} {
	for _, f := range _fields {

		if strings.HasPrefix(f.Identifier, "system_") || (f.Validation != nil && f.Validation.IsPassword) { // skip password field and system_ fields
			continue
		}

		if f.FieldType == "multiline" {
			fields[f.Identifier] = []string{"html"}
		} else if f.FieldType == "media" {
			fields[f.Identifier] = []string{"url"}
		} else if f.FieldType == "geo" {
			fields[f.Identifier] = []string{"lat", "lon"}
		} else if (f.FieldType == "repeated" || f.FieldType == "object") && len(f.SubFieldInfo) > 0 {
			var _nested []*models.FieldInfo
			for _, sf := range f.SubFieldInfo {
				_nested = append(_nested, &models.FieldInfo{
					Identifier:      sf.Identifier,
					Description:     sf.Description,
					InputType:       sf.InputType,
					FieldType:       sf.FieldType,
					Validation:      sf.Validation,
					Serial:          sf.Serial,
					Label:           sf.Label,
					SystemGenerated: sf.SystemGenerated,
					SubFieldInfo:    sf.SubFieldInfo,
				})
			}
			nested := getFieldsInfo(make(map[string]interface{}), _nested)
			fields[f.Identifier] = nested
		} else {
			fields[f.Identifier] = nil
		}
	}
	return fields
}

func getConnectionsInfo(_connections []*models.ConnectionType) map[string]string {
	connections := make(map[string]string)
	for _, c := range _connections {
		connections[c.Model] = c.Relation
	}
	return connections
}

func singleFieldBuilder(key string, value interface{}, filteredSubfields []string) string {
	if value != nil {
		switch reflect.TypeOf(value).Kind() {
		case reflect.Slice:
			vals := value.([]string)
			var filtered []string
			for _, v := range filteredSubfields {
				if contains(vals, v) {
					filtered = append(filtered, v)
				}
			}
			if filtered != nil {
				return fmt.Sprintf(`%s { %s }`, key, strings.Join(filtered, " "))
			}
			return fmt.Sprintf(`%s { %s }`, key, strings.Join(vals, " "))
		case reflect.Map:
			val := value.(map[string]interface{})
			if filteredSubfields != nil {
				filtered := make(map[string]interface{})
				for _, v := range filteredSubfields {
					if filter, ok := val[v]; ok {
						filtered[v] = filter
					}
				}
				v := validFieldBuilder(filtered, filteredSubfields)
				return fmt.Sprintf(`%s { %s }`, key, strings.Join(v, " "))
			}
			v := validFieldBuilder(val, filteredSubfields)
			return fmt.Sprintf(`%s { %s }`, key, strings.Join(v, " "))
		}
	} else {
		return fmt.Sprintf(`%s`, key)
	}
	return ""
}

func validFieldBuilder(fields map[string]interface{}, subFilter []string) []string {
	var validatedFields []string
	for k, v := range fields {
		validatedFields = append(validatedFields, singleFieldBuilder(k, v, subFilter))
	}
	return validatedFields
}

func RESTtoGraphQL(c echo.Context, schema *models.ProjectSchema, rootModel string, rest url.Values, skipQueryDef bool) (*RESTtoGraphResponse, error) {

	var modelType *models.ModelType
	for _, _model := range schema.Models {
		if _model.Name == rootModel {
			modelType = _model
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	validFields := getFieldsInfo(make(map[string]interface{}), modelType.Fields)
	validConnections := getConnectionsInfo(modelType.Connections)
	var validatedFields []string
	if uf, ok := rest["validFields"]; ok {
		var userGivenFields []string
		err := json.Unmarshal([]byte(uf[0]), &userGivenFields)
		if err != nil {
			return nil, err
		}
		for _, f := range userGivenFields {
			// #todo handle rested
			if strings.Contains(f, "(") {
				re := regexp.MustCompile(`\((.*?)\)`)
				sf := re.FindStringSubmatch(f)
				subfields := strings.Split(sf[1], ",")
				split := strings.Split(f, "(")
				field := split[0]
				fmt.Println(subfields)
				if valid, ok := validFields[field]; ok {
					s := singleFieldBuilder(field, valid, subfields)
					validatedFields = append(validatedFields, s)
				}
			} else {
				if valid, ok := validFields[f]; ok {
					s := singleFieldBuilder(f, valid, nil)
					validatedFields = append(validatedFields, s)
				}
			}
		}
	} else {
		b := validFieldBuilder(validFields, nil)
		validatedFields = append(validatedFields, b...)
	}

	var filters []string
	if local, ok := rest["local"]; ok {
		filters = append(filters, fmt.Sprintf(`local: %s`, local[0]))
	}

	var metaData string
	if meta, ok := rest["meta"]; ok {
		isMeta, _ := strconv.ParseBool(meta[0])
		if isMeta {
			metaData = fmt.Sprintf(`
				meta {
				  status
				  updated_at
				  last_modified_by {
					avatar
					id
					name
				  }
				  created_by {
					avatar
					id
					name
				  }
				  created_at
				}
			`)
		}
	}

	var queryModelName string
	var operationName string
	var query string
	var payloads string

	switch c.Request().Method {
	case "GET":
		operationName = fmt.Sprintf(`Get%s`, strings.Title(modelType.Name))
		var whereStrings []string
		if _id, ok := rest["_id"]; ok {
			id := _id[0]
			queryModelName = modelType.Name
			if strings.Contains(id, "-") {
				filters = append(filters, fmt.Sprintf(`_id: "%s"`, id))
			}
		} else {
			queryModelName = MultipleResourceName(modelType.Name)
			if limit, ok := rest["limit"]; ok {
				l, _ := strconv.Atoi(limit[0])
				filters = append(filters, fmt.Sprintf(`limit: %d`, l))
			}
			if page, ok := rest["page"]; ok {
				l, _ := strconv.Atoi(page[0])
				filters = append(filters, fmt.Sprintf(`page: %d`, l))
			}

			if where, ok := rest["query"]; ok {
				var whereFilters map[string]interface{}
				err := json.Unmarshal([]byte(where[0]), &whereFilters)
				if err != nil {
					return nil, err
				}
				for k, v := range whereFilters {
					var key string
					var condition string
					if strings.Contains(k, ":") {
						s := strings.Split(k, ":")
						key = s[0]
						condition = s[1]
					} else {
						key = k
						condition = "eq"
					}
					//format the value
					var value interface{}
					switch v.(type) {
					case string:
						value = fmt.Sprintf(`"%v"`, v)
					case int, float64, bool:
						value = v
					}
					whereStrings = append(whereStrings, fmt.Sprintf(`%s : { %s : %v }`, key, condition, value))
				}
			}
		}

		var filterConditions string
		if len(filters) > 0 {
			filterConditions = strings.Join(filters, ", ")
		}

		if len(whereStrings) > 0 {
			filterConditions += fmt.Sprintf(`, where : { %s }`, strings.Join(whereStrings, ", "))
		}

		if filterConditions != "" {
			filterConditions = fmt.Sprintf(`( %s )`, filterConditions)
		}

		var dataAlies string
		if _, ok := rest["relation"]; ok {
			dataAlies = fmt.Sprintf(`%s : data`, modelType.Name)
		} else {
			dataAlies = fmt.Sprintf(`data`)
		}

		var nestedQuery string
		if relation, ok := rest["relation"]; ok {
			model := SingularResourceName(relation[0])
			delete(rest, "_id") // nested doesn't need id. Must include to avoid loopOnNestedSets
			delete(rest, "relation")
			nestedBuildedQuery, err := RESTtoGraphQL(c, schema, model, rest, true)
			if err != nil {
				return nil, err
			}
			nestedQuery = nestedBuildedQuery.Query
		}

		query = fmt.Sprintf(`
			  %s %s {
				id
				%s {
				  %s
				}
				%s
				%s
			  }
		`, queryModelName, filterConditions, dataAlies, strings.Join(validatedFields, " "), nestedQuery, metaData)
		if !skipQueryDef {
			query = fmt.Sprintf(`query %s { %s }`, operationName, query)
		}
		break
	case "PUT", "POST":
		var req map[string]interface{}
		if c.Request().Method == "PUT" {
			queryModelName = strcase.ToLowerCamel("create " + modelType.Name)
			operationName = fmt.Sprintf(`Create%s`, strings.Title(modelType.Name))
		} else if c.Request().Method == "POST" {
			queryModelName = strcase.ToLowerCamel("update " + modelType.Name)
			operationName = fmt.Sprintf(`Update%s`, strings.Title(modelType.Name))
		}
		if err := c.Bind(&req); err != nil {
			return nil, err
		}
		if len(req) == 0 {
			return nil, errors.New("Request body is required")
		}
		// #todo validate req with validFields
		payload, connect := validPayloadBuilder(req, validFields, validConnections)
		if payload != "" {
			filters = append(filters, fmt.Sprintf(`payload : { %s }`, payload))
		}
		if connect != "" {
			filters = append(filters, fmt.Sprintf(`connect : { %s }`, connect))
		}
		if _id, ok := rest["_id"]; ok {
			filters = append(filters, fmt.Sprintf(`_id: "%s"`, _id[0]))
		}
		if len(filters) > 0 {
			payloads = fmt.Sprintf(`( %s )`, strings.Join(filters, ", "))
		}
		query = fmt.Sprintf(`
			  %s %s {
				id
				data {
				  %s
				}
				%s
			  }
		`, queryModelName, payloads, strings.Join(validatedFields, " "), metaData)
		query = fmt.Sprintf(`mutation %s { %s }`, operationName, query)
		break
	case "DELETE":
		queryModelName = strcase.ToLowerCamel("delete " + modelType.Name)
		operationName = fmt.Sprintf(`Delete%s`, strings.Title(modelType.Name))
		if _id, ok := rest["_id"]; ok {
			filters = append(filters, fmt.Sprintf(`_ids: ["%s"]`, _id[0]))
		}
		if len(filters) > 0 {
			payloads = fmt.Sprintf(`( %s )`, strings.Join(filters, ", "))
		}
		query = fmt.Sprintf(`
			  %s %s {
				response
				%s
			  }
		`, queryModelName, payloads, metaData)
		query = fmt.Sprintf(`mutation %s { %s }`, operationName, query)
		break
	}

	return &RESTtoGraphResponse{
		Query:         query,
		QueryName:     queryModelName,
		OperationName: operationName,
	}, nil
}

func HandleFunctionGraphqlBase(c echo.Context, schema *models.ProjectSchema, functionName string) (*RESTtoGraphResponse, error) {

	//#todo model check is necessary of the request body

	fs := strings.Split(functionName, "/")
	queryModelName := fs[len(fs)-1]
	operationName := fmt.Sprintf(`Call%s`, strings.Title(queryModelName))

	var functionToExecute *models.ApitoFunction
	for _, _fn := range schema.Functions {
		if _fn.Name == queryModelName {
			functionToExecute = _fn
			break
		}
	}

	if functionToExecute == nil {
		return nil, errors.New("function not found")
	}

	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	if len(req) == 0 {
		return nil, errors.New("request body is required")
	}

	// for now the valid fields & validated fields are the same
	// -> req
	var validatedFields []string
	for k, _ := range req {
		validatedFields = append(validatedFields, k)
	}

	var query string
	var payloads string
	// #todo validate req with validFields
	payload, _ := validPayloadBuilder(req, req, nil)
	if payload != "" {
		payloads = fmt.Sprintf(`( payload : { %s } )`, payload)
	}

	var queryBody string
	if functionToExecute.Response != nil && functionToExecute.Response.Model != "JSON" {
		queryBody = fmt.Sprintf(`
			%s {
				id
				data {
				   %s
				}
			}
		`, functionToExecute.Response.Model, strings.Join(validatedFields, " "))
	} else {
		queryBody = fmt.Sprintf(`JSON`)
	}

	query = fmt.Sprintf(`
			  %s %s {
				%s
			  }
		`, queryModelName, payloads, queryBody)
	query = fmt.Sprintf(`mutation %s { %s }`, operationName, query)

	return &RESTtoGraphResponse{
		Query:         query,
		QueryName:     queryModelName,
		OperationName: operationName,
	}, nil
}
