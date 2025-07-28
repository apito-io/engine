package firestore

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"cloud.google.com/go/firestore"

	"github.com/apito-io/engine/database/utility"
	"github.com/apito-io/engine/models"
	utility2 "github.com/apito-io/engine/utility"
	"github.com/tailor-inc/graphql"
)

func LocalBuilder(variable string, args map[string]interface{}, modelType *models.ModelType) firestore.Query {
	// return MERGE(x, { data :  {'first_name' : x.data.first_name_bn, 'last_name' : x.data.last_name} })
	var returnType firestore.Query
	if val, ok := args["local"]; ok {
		local := val.(string) // #todo will be enum in future

		var dataJson []string
		for _, f := range modelType.Fields {
			if f.Validation != nil && local != "en" && utility2.ArrayContains(f.Validation.Locals, local) {
				dataJson = append(dataJson, fmt.Sprintf(`'%s' : x.data.%s_%s`, f.Identifier, f.Identifier, local))
			} else {
				dataJson = append(dataJson, fmt.Sprintf(`'%s' : x.data.%s`, f.Identifier, f.Identifier))
			}
		}
		//returnType = fmt.Sprintf(`MERGE(x, { data : {%s} })`, strings.Join(dataJson, ", "))
	} else {
		//returnType = fmt.Sprintf(`x`)
	}
	return returnType
}

func LimitBuilder(param *graphql.ResolveParams) firestore.Query {
	var q firestore.Query
	arg := param.Args

	limit := 10
	if val, ok := arg["limit"]; ok {
		limit = val.(int)
	}

	start := 0
	if val, ok := arg["start"]; ok {
		start = val.(int)
	}

	page := 1
	if val, ok := arg["page"]; ok {
		page = val.(int)
	}

	if page > 1 {
		offset := limit * (page - 1)
		return q.OrderBy("meta.created_at", firestore.Asc).StartAfter(offset).Limit(limit)
	}

	return q.OrderBy("meta.created_at", firestore.Asc).StartAfter(start).Limit(limit)
}

var FilterSuffix = map[string]string{
	"eq":     "==",
	"ne":     "!=",
	"lt":     "<",
	"lte":    "<=",
	"gt":     ">",
	"gte":    ">=",
	"in":     "in",
	"not_in": "not-in",
}

func getFieldType(val interface{}) reflect.Kind {
	return reflect.TypeOf(val).Kind()
}

func FilterBuilder(variable string, where map[string]interface{}, modelType *models.ModelType) ([]firestore.Query, error) {

	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = utility.GetUserFieldType(field)
	}

	var conditions []firestore.Query

	for field, filterObj := range where {

		fieldName := fmt.Sprintf("%s.%s", variable, field)

		var actualValue interface{}
		for suffix, value := range filterObj.(map[string]interface{}) {
			actualValue = value
			var q firestore.Query
			switch suffix {
			case "contains":
				return nil, errors.New("contains is not yet supported byt Firestore Itself")
			case "eq", "ne", "lt", "lte", "gt", "gtr", "in", "not_in":
				switch value.(type) {
				case int, float64, bool:
					conditions = append(conditions, q.Where(fieldName, FilterSuffix[suffix], value.(string)))
					break
				case string:
					conditions = append(conditions, q.Where(fieldName, FilterSuffix[suffix], value.(string)))
					break
				case []interface{}:
					var vals []string
					for _, v := range value.([]interface{}) {
						switch v.(type) {
						case int, float64:
							vals = append(vals, fmt.Sprintf("%v", v))
						case string:
							vals = append(vals, fmt.Sprintf("'%v'", v))
						}
					}
					conditions = append(conditions, q.Where(fieldName, FilterSuffix[suffix], vals))
				}
				break
			}
		}

		//validate the field & type
		if kind, ok := userDefinedFieldNames[field]; ok {
			k := getFieldType(actualValue)
			if kind != k {
				return nil, errors.New(fmt.Sprintf("Invalid Value for %s in Query. Type mismatched", field))
			}
		} else {
			return nil, errors.New(fmt.Sprintf("Invalid Field Name %s in Query", field))
		}

	}

	return conditions, nil
}

func ConditionBuilder(variable string, args map[string]interface{}, modelType *models.ModelType) (map[string][]firestore.Query, error) {

	var err error
	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = utility.GetUserFieldType(field)
	}

	conditions := make(map[string][]firestore.Query)

	if w, ok := args["where"]; ok {
		where := w.(map[string]interface{})
		for field, filterObj := range where {

			switch field { // default is x.data
			case "role":
				variable = "x"
			case "tenant":
				variable = "x"
			}

			// if AND / OR found
			switch field {
			case "AND":
				conditions["AND"], err = FilterBuilder(variable, filterObj.(map[string]interface{}), modelType)
				if err != nil {
					return nil, err
				}
			case "OR":
				conditions["OR"], err = FilterBuilder(variable, filterObj.(map[string]interface{}), modelType)
				if err != nil {
					return nil, err
				}
			default:
				conditions["AND"], err = FilterBuilder(variable, where, modelType)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return conditions, nil
}

func RootResolverQueryBuilder(param *models.CommonSystemParams, previewMode bool) ([]firestore.Query, error) {

	var modelName string
	if param.Model == nil {
		modelName = utility2.SingularResourceName(param.ResolveParams.Info.FieldName)
	} else {
		modelName = utility2.SingularResourceName(param.Model.Name)
	}

	var finalQuery []firestore.Query

	fields := []string{"id: x.id", "status : x.meta.status"}

	var previewField *string
	var previewIcon *string
	if previewMode { // preview mode is on so just fetch the preview data
		for _, f := range param.Model.Fields {
			if f.Validation != nil && previewField == nil {
				previewField = &f.Identifier
			} else if f.FieldType == "media" && f.Validation != nil && !f.Validation.IsGallery && previewIcon == nil {
				previewIcon = &f.Identifier
			}
		}

		if previewField == nil {
			// search for anything with string type
			for _, f := range param.Model.Fields {
				if f.FieldType == "text" || f.FieldType == "multiline" { // assign any text field as title
					if f.FieldType == "multiline" {
						html := fmt.Sprintf(`%s.html`, f.Identifier)
						previewField = &html
					} else {
						previewField = &f.Identifier
					}
					break
				}
			}
		}
	}

	var returnType string
	if previewMode {
		if previewIcon != nil {
			fields = append(fields, fmt.Sprintf(`icon : x.data.%s.url`, *previewIcon))
		}
		if previewField == nil {
			fields = append(fields, `title : x.id`) // default show the id
		} else {
			fields = append(fields, fmt.Sprintf(`title : x.data.%s`, *previewField))
		}
		returnType = fmt.Sprintf(`{ %s }`, strings.Join(fields, " , "))
	} else {
		finalQuery = append(finalQuery, LocalBuilder("x", param.ResolveParams.Args, param.Model))
	}

	finalQuery = append(finalQuery, LimitBuilder(param.ResolveParams))

	var queries []string
	queries = append(queries, fmt.Sprintf("FOR x in `p_%s`", param.ProjectID))

	filters, err := ConditionBuilder("x.data", param.ResolveParams.Args, param.Model)
	if err != nil {
		return nil, err
	}

	// filter based on roles
	if permission, ok := param.Role.APIPermissions[modelName]; ok && param.Role.APIPermissions != nil {
		switch permission.Read {
		case "own":
			var q firestore.Query
			q.Where("meta.created_by.id", "=", param.UserID)
			filters["AND"] = []firestore.Query{q}
			break
		case "tenant":
			var q firestore.Query
			q.Where("tenant_id", "=", param.TenantID)
			filters["AND"] = []firestore.Query{q}
			break
		}
	}

	var mergedFilter firestore.Query
	for condition, filter := range filters {
		if condition != "OR" {
			for _, f := range filter {
				mergedFilter = f
			}
		}
	}

	finalQuery = append(finalQuery, mergedFilter)

	if len(filters) > 0 {
		//queries = append(queries, fmt.Sprintf(`Filter x.type == '%s' AND %s`, modelName, strings.Join(mergedFilter, " AND ")))
	} else if previewMode && param.ResolveParams.Args["search"] != nil {
		arg := strings.ToLower(param.ResolveParams.Args["search"].(string))
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s' AND LOWER(x.data.%s) LIKE '%%%s%%'`, modelName, *previewField, arg))
	} else {
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s'`, modelName))
	}

	// default sort
	queries = append(queries, fmt.Sprintf(`SORT DATE_TIMESTAMP(x.meta.created_at) DESC`))
	queries = append(queries, fmt.Sprintf(`return %s`, returnType))

	return finalQuery, nil
}
