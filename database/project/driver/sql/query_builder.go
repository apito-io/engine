package sql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	qutility "github.com/apito-io/engine/database/utility"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
)

// formatSQLMetaTimestamp normalizes meta.created_at / meta.updated_at (and similar) from SQL row scans.
// SQLite and libsql commonly return DATE/DATETIME as string; PostgreSQL returns time.Time.
func formatSQLMetaTimestamp(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339), nil
	case *time.Time:
		if t == nil {
			return "", nil
		}
		return t.UTC().Format(time.RFC3339), nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", nil
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		return s, nil
	case []byte:
		return formatSQLMetaTimestamp(string(t))
	default:
		return "", fmt.Errorf("unexpected type for meta timestamp: %T", v)
	}
}

type QueryBuilderParam struct {
	CollectionName string
	RelationName   string
	Args           map[string]interface{}
	ParentModel    string
	ModelType      *models.ModelType
}

var FilterSuffix = map[string]string{
	"eq":     "==",
	"ne":     "!=",
	"lt":     "<",
	"lte":    "<=",
	"gt":     ">",
	"gte":    ">=",
	"in":     "IN",
	"not_in": "NOT IN",
}

// filterSQLComparator maps GraphQL filter suffixes to SQL operators for the parameterized query path.
func filterSQLComparator(suffix string) (string, bool) {
	switch suffix {
	case "eq":
		return "=", true
	case "ne":
		return "!=", true
	case "lt":
		return "<", true
	case "lte":
		return "<=", true
	case "gt":
		return ">", true
	case "gte", "gtr":
		return ">=", true
	case "in":
		return "IN", true
	case "not_in":
		return "NOT IN", true
	default:
		return "", false
	}
}

func SelectBuilder(mv string, local string, modelType *models.ModelType, returnCount bool) []string {

	var returnType []string
	if returnCount {
		return []string{"count(x.id)"}
	}

	var dataJson []string
	for _, f := range modelType.Fields {
		phys := PhysicalSQLColumnForSystemRelationField(f.Identifier)
		if f.Validation != nil && local != "en" && utility.ArrayContains(f.Validation.Locals, local) {
			dataJson = append(dataJson, fmt.Sprintf(`x.%s_%s AS %s`, phys, local, f.Identifier))
		} else {
			dataJson = append(dataJson, fmt.Sprintf(`x.%s AS %s`, phys, f.Identifier))
		}
	}
	// Avoid `SELECT x.id AS id, , y.created_at...` when the model has no user fields yet.
	returnType = append(returnType, "x.id AS id")
	if len(dataJson) > 0 {
		returnType = append(returnType, strings.Join(dataJson, ", "))
	}

	metaQuery := fmt.Sprintf(`%s.created_at AS sys_created_at, %s.updated_at AS sys_updated_at, %s.created_by AS sys_created_by, %s.updated_by AS sys_updated_by, %s.status as sys_status`, mv, mv, mv, mv, mv)
	returnType = append(returnType, metaQuery)

	return returnType
}

func graphqlArgInt(m map[string]interface{}, key string, def int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

// graphqlArgBool coerces GraphQL / JSON variable shapes into a boolean (used for intersect).
func graphqlArgBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case *bool:
		if t == nil {
			return false
		}
		return *t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case json.Number:
		i, err := t.Int64()
		return err == nil && i != 0
	default:
		return false
	}
}

func LimitBuilder(param *graphql.ResolveParams) (int, int) {
	arg := param.Args

	limit := graphqlArgInt(arg, "limit", 10)
	start := graphqlArgInt(arg, "start", 0)
	page := graphqlArgInt(arg, "page", 1)

	if page > 1 {
		offset := limit * (page - 1)
		return limit, offset
	}
	return limit, start
}

func getFieldType(val interface{}) reflect.Kind {
	return reflect.TypeOf(val).Kind()
}

func ConditionBuilder(variable string, args map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) (map[string][]string, error) {

	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = qutility.GetUserFieldType(field)
	}

	conditions := make(map[string][]string)

	if w, ok := args["where"]; args["where"] != nil && ok && len(w.(map[string]interface{})) > 0 {
		where := w.(map[string]interface{})
		for field, filterObj := range where {

			switch field { // default is x.data
			case "role":
				variable = "x"
			}

			// if AND / OR found
			switch field {
			case "AND":
				conditions["AND"], _ = FilterBuilder(variable, filterObj.(map[string]interface{}), modelType, sqlArgs)
			case "OR":
				conditions["OR"], _ = FilterBuilder(variable, filterObj.(map[string]interface{}), modelType, sqlArgs)
			default:
				conditions["AND"], _ = FilterBuilder(variable, where, modelType, sqlArgs)
			}
		}
	}

	return conditions, nil
}

func FilterBuilder(variable string, where map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) ([]string, error) {

	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = qutility.GetUserFieldType(field)
	}

	var conditions []string

	for field, filterObj := range where {

		sqlField := field
		// SQL row filters use table alias `x`; Arango-style paths use `x.data` — keep document keys intact there.
		if variable == "x" {
			sqlField = PhysicalSQLColumnForSystemRelationField(field)
		}
		fieldName := fmt.Sprintf("%s.%s", variable, sqlField)

		var actualValue interface{}
		for suffix, value := range filterObj.(map[string]interface{}) {

			actualValue = value

			switch suffix {
			case "contains":
				if sqlArgs != nil {
					conditions = append(conditions, fmt.Sprintf(`%s LIKE ?`, fieldName))
					*sqlArgs = append(*sqlArgs, fmt.Sprintf("%%%s%%", value.(string)))
				} else {
					conditions = append(conditions, fmt.Sprintf(`%s LIKE '%%%s%%'`, fieldName, value.(string)))
				}
			case "eq", "ne", "lt", "lte", "gt", "gtr", "in", "not_in":
				if sqlArgs != nil {
					op, ok := filterSQLComparator(suffix)
					if !ok {
						continue
					}
					switch suffix {
					case "in", "not_in":
						arr, ok := value.([]interface{})
						if !ok || len(arr) == 0 {
							if suffix == "in" {
								conditions = append(conditions, "0=1")
							} else {
								conditions = append(conditions, "1=1")
							}
							break
						}
						placeholders := strings.TrimSuffix(strings.Repeat("?,", len(arr)), ",")
						conditions = append(conditions, fmt.Sprintf(`%s %s (%s)`, fieldName, op, placeholders))
						for _, v := range arr {
							*sqlArgs = append(*sqlArgs, v)
						}
					default:
						conditions = append(conditions, fmt.Sprintf(`%s %s ?`, fieldName, op))
						*sqlArgs = append(*sqlArgs, value)
					}
				} else {
					switch value := value.(type) {
					case int, float64, bool:
						conditions = append(conditions, fmt.Sprintf(`%s %s %v`, fieldName, FilterSuffix[suffix], value))
					case string:
						conditions = append(conditions, fmt.Sprintf(`%s %s '%v'`, fieldName, FilterSuffix[suffix], value))
					case []interface{}:
						var vals []string
						for _, v := range value {
							switch v.(type) {
							case int, float64:
								vals = append(vals, fmt.Sprintf("%v", v))
							case string:
								vals = append(vals, fmt.Sprintf("'%v'", v))
							}
						}
						final := fmt.Sprintf("[%s]", strings.Join(vals, ","))
						conditions = append(conditions, fmt.Sprintf(`COUNT(%s[* FILTER CONTAINS(%s, CURRENT)])`, fieldName, final))
					}
				}
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

func CommonDocTransformation(model *models.ModelType, local string, result map[string]interface{}, classification *FieldClassification) (*types.DefaultDocumentStructure, error) {

	doc := types.DefaultDocumentStructure{
		Type: model.Name,
		Meta: &types.MetaField{},
	}

	if val, ok := result["id"].(string); ok {
		doc.ID = string(val)
		doc.Key = doc.ID
	} else {
		return nil, errors.New("id is required for any document to fetch")
	}

	data := map[string]interface{}{}

	for k, v := range result {
		switch k {
		case "doc_id":
			continue
		case "sys_key":
			continue
		case "id":
			continue
		case "sys_status":
			doc.Meta.Status = v.(string)
		case "sys_created_at":
			s, err := formatSQLMetaTimestamp(v)
			if err != nil {
				return nil, err
			}
			doc.Meta.CreatedAt = s
		case "sys_updated_at":
			s, err := formatSQLMetaTimestamp(v)
			if err != nil {
				return nil, err
			}
			doc.Meta.UpdatedAt = s
		case "sys_updated_by":
			id := v.(string)
			doc.Meta.LastModifiedBy = &types.SystemUser{
				ID: string(id),
			}
		case "sys_created_by":
			id := v.(string)
			doc.Meta.CreatedBy = &types.SystemUser{
				ID: string(id),
			}
		default:
			if utility.ArrayContains(classification.MultilineFields, k) {
				var html string
				if val, ok := v.(string); ok {
					html = val
				}
				// Use the new markdown processor
				processed := utility.ProcessMultilineField(map[string]interface{}{
					"html": html,
				})
				data[k] = processed
			} else if utility.ArrayContains(classification.DoubleFields, k) {
				if val, ok := v.([]byte); ok {
					f, _ := strconv.ParseFloat(string(val), 64)
					data[k] = f
				}
			} else if utility.ArrayContains(classification.PictureField, k) {
				if val, ok := v.([]byte); ok {
					var pic map[string]interface{}
					err := json.Unmarshal(val, &pic)
					if err != nil {
						return nil, err
					}
					data[k] = pic
				}
			} else if utility.ArrayContains(classification.GalleryField, k) {
				if val, ok := v.([]byte); ok {
					var gallery []map[string]interface{}
					err := json.Unmarshal(val, &gallery)
					if err != nil {
						return nil, err
					}
					data[k] = gallery
				}
			} else if utility.ArrayContains(classification.ListFields, k) {
				if val, ok := v.([]byte); ok {
					var lists []interface{}
					err := json.Unmarshal(val, &lists)
					if err != nil {
						return nil, err
					}
					data[k] = lists
				}
			} else if subfields, ok := classification.RepeatedFields[k]; ok && len(classification.RepeatedFields) > 0 {

				var repeated []map[string]interface{}
				if val, ok := v.([]byte); ok {
					err := json.Unmarshal(val, &repeated)
					if err != nil {
						return nil, err
					}
				}
				if local == "en" {
					data[k] = repeated
				} else {
					for _, subItem := range repeated {
						for _, f := range subfields {
							if f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) {
								if localContentFound, ok := subItem[fmt.Sprintf(`%s_%s`, f.Identifier, local)]; ok {
									subItem[f.Identifier] = localContentFound
								}
								break
							}
						}
					}
					data[k] = repeated
				}
			} else {
				data[k] = v
			}
		}
	}
	doc.Data = data
	return &doc, nil
}

func MediaDocTransformation(docType string, result map[string]interface{}) (*models.FileDetails, error) {

	doc := models.FileDetails{
		Type: docType,
	}

	if val, ok := result["id"].([]byte); ok {
		doc.ID = string(val)
		doc.XKey = doc.ID
	} else {
		return nil, nil
	}

	if s, err := formatSQLMetaTimestamp(result["created_at"]); err != nil {
		return nil, err
	} else if s != "" {
		doc.CreatedAt = s
	}

	if val, ok := result["model"].(string); ok {
		if doc.UploadParam == nil {
			doc.UploadParam = &models.UploadParams{}
		}
		doc.UploadParam.ModelName = val
	}

	if val, ok := result["s3_key"].(string); ok {
		doc.S3Key = val
	}

	if val, ok := result["media_type"].(string); ok {
		doc.ContentType = val
	}

	if val, ok := result["file_extension"].(string); ok {
		doc.FileExtension = val
	}

	if val, ok := result["file_name"].(string); ok {
		doc.FileName = val
	}

	if val, ok := result["size"].(int32); ok {
		doc.Size = int64(val)
	}

	if val, ok := result["url"].(string); ok {
		doc.URL = val
	}

	return &doc, nil
}

func RootConnectionResolverQueryBuilder(cfg *models.Config, param *models.CommonSystemParams) (string, error) {

	projectId := param.ProjectID
	_args := param.ResolveParams.Args

	filters, err := ConditionBuilder("x.data", _args, param.Model, nil)
	if err != nil {
		return "", err
	}
	if err := mergeQueryFilterHookAQL(cfg, param, filters, "x"); err != nil {
		return "", err
	}

	var mergedFilter []string
	for condition, _ := range filters {
		mergedFilter = append(mergedFilter, strings.Join(filters[condition], condition))
	}

	model := param.Model.Name

	var queries []string
	queries = append(queries, fmt.Sprintf("FOR x in `p_%s`", projectId))
	if len(filters) > 0 {
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s' AND %s`, model, strings.Join(mergedFilter, " AND "))) // #todo need fix
	} else {
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s'`, model))
	}
	queries = append(queries, fmt.Sprintf(`COLLECT WITH COUNT INTO total`))
	queries = append(queries, fmt.Sprintf(`return total`))

	return strings.Join(queries, " "), nil
}

func RootResolverQueryBuilder(cfg *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {

	var modelName string
	if param.Model == nil {
		modelName = utility.SingularResourceName(param.ResolveParams.Info.FieldName)
	} else {
		modelName = utility.SingularResourceName(param.Model.Name)
	}

	var leftJoins []string

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := SelectBuilder("y", local, param.Model, returnCount)

	var connection map[string]interface{}
	if val, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok {
		connection = val
	}

	intersect := false
	if param.ResolveParams != nil && param.ResolveParams.Args != nil {
		intersect = graphqlArgBool(param.ResolveParams.Args, "intersect")
	}

	var connPreds []string
	var connArgs []interface{}

	if len(connection) > 0 {
		var relationType string
		if val, ok := connection["relation_type"].(string); ok && val != "" {
			relationType = val
		}

		var connectionType string
		if val, ok := connection["connection_type"].(string); ok && val != "" {
			connectionType = val
		} else {
			return "", nil, errors.New("connection type is required if passing connection object")
		}

		var fromModel string
		var toModel string
		switch connectionType {
		case "forward":
			fromModel = connection["to_model"].(string)
			toModel = connection["model"].(string)
		case "backward":
			fromModel = connection["model"].(string)
			toModel = connection["to_model"].(string)
		default:
			return "", nil, errors.New("invalid connection type")
		}

		switch relationType {
		case "has_many":
			anchorID, okAnchor := sqlConnectionAnchorID(connection)
			if !okAnchor {
				return "", nil, errors.New("connection _id is required when filtering by relation")
			}
			// Console sends relation_type=has_many for "parent has many children"; SQL may use either a
			// pivot (true M:N) or a FK on the child table (e.g. Work has_one Person → person_id on work).
			usePivot := len(param.ProjectSchemaModels) == 0 || sqlConnectionIsTrueManyToMany(param.ProjectSchemaModels, fromModel, toModel)
			if usePivot {
				pivotTable := fmt.Sprintf(`%s_%s`, utility.SingularResourceName(fromModel), utility.SingularResourceName(toModel))
				// Pivot columns must follow the *listed* model (x) and *anchor* document (connection._id),
				// not connection fromModel/toModel order (backward vs forward swaps which side is listed).
				if param.Model == nil {
					return "", nil, errors.New("model is required for pivot relation filter")
				}
				listedPivotCol := utility.SingularResourceName(param.Model.Name) + "_id"
				anchorModelName := sqlConnectionAnchorModelName(connectionType, fromModel, toModel)
				if anchorModelName == "" {
					return "", nil, errors.New("invalid connection type")
				}
				anchorPivotCol := utility.SingularResourceName(anchorModelName) + "_id"
				if intersect {
					// Same as Arango: listed rows whose id is NOT IN pivot ids linked to the anchor.
					if !returnCount {
						returnType = SelectBuilder("y", local, param.Model, false)
					}
					connPreds = append(connPreds, fmt.Sprintf("`x`.`id` NOT IN (SELECT `p`.`%s` FROM `%s` AS `p` WHERE `p`.`%s` = ?)", listedPivotCol, pivotTable, anchorPivotCol))
					connArgs = append(connArgs, anchorID)
				} else {
					leftJoins = append(leftJoins, fmt.Sprintf(`left join `+"`%s`"+` as z on z.%s = x.id`, pivotTable, listedPivotCol))
					// Keep default returnType (full `x` row + meta): pivot only filters; overwriting
					// returnType would omit `id` and break CommonDocTransformation.
					connPreds = append(connPreds, fmt.Sprintf("`z`.`%s` = ?", anchorPivotCol))
					connArgs = append(connArgs, anchorID)
				}
			} else {
				xfk := utility.SingularResourceName(fromModel) + "_id"
				if intersect {
					connPreds = append(connPreds, fmt.Sprintf("(`x`.`%s` IS NULL OR `x`.`%s` != ?)", xfk, xfk))
					connArgs = append(connArgs, anchorID)
				} else {
					connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", xfk))
					connArgs = append(connArgs, anchorID)
				}
			}
		case "has_one":
			anchorID, okAnchor := sqlConnectionAnchorID(connection)
			if !okAnchor {
				return "", nil, errors.New("connection _id is required when filtering by relation")
			}
			anchorM := sqlConnectionAnchorModelName(connectionType, fromModel, toModel)
			if anchorM == "" {
				return "", nil, errors.New("invalid connection type")
			}
			listedMT := param.Model
			if listedMT == nil {
				return "", nil, errors.New("model is required for connection filter")
			}
			if len(param.ProjectSchemaModels) > 0 {
				anchorMT := findSchemaModel(param.ProjectSchemaModels, anchorM)
				if col, ok := fkPhysicalColumnOnModelToTarget(listedMT, anchorM); ok {
					connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", col))
					connArgs = append(connArgs, anchorID)
				} else if anchorMT != nil {
					if col, ok := fkPhysicalColumnOnModelToTarget(anchorMT, listedMT.Name); ok {
						anchTbl := utility.SingularResourceName(anchorM)
						connPreds = append(connPreds, fmt.Sprintf("EXISTS (SELECT 1 FROM `%s` AS anch WHERE anch.id = ? AND anch.`%s` = `x`.`id`)", anchTbl, col))
						connArgs = append(connArgs, anchorID)
					} else {
						return "", nil, fmt.Errorf("could not resolve SQL FK for has_one connection (listed=%s anchor=%s)", listedMT.Name, anchorM)
					}
				} else {
					return "", nil, fmt.Errorf("unknown anchor model %q for has_one connection", anchorM)
				}
			} else {
				col := utility.SingularResourceName(anchorM) + "_id"
				connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", col))
				connArgs = append(connArgs, anchorID)
			}
		}
	}

	limit, offset := LimitBuilder(param.ResolveParams)

	tableName := utility.SingularResourceName(param.Model.Name)

	if !returnCount {
		leftJoins = append(leftJoins, `left join meta as y on y.doc_id = x.id`)
	}

	var queries []string
	queries = append(queries, fmt.Sprintf("SELECT %s FROM `%s` as x %s",
		strings.Join(returnType, ", "),
		tableName,
		strings.Join(leftJoins, "\n"),
	))

	var sqlArgs []interface{}
	filters, err := ConditionBuilder("x", param.ResolveParams.Args, param.Model, &sqlArgs)
	if err != nil {
		return "", nil, err
	}

	// filter based on roles
	if permission, ok := utility.LookupAPIPermission(param.Role, modelName); ok {
		switch permission.Read {
		case "own":
			if !returnCount {
				filters["AND"] = append(filters["AND"], `y.created_by = ?`)
				sqlArgs = append(sqlArgs, param.UserID)
			}
		}
	}
	if err := mergeQueryFilterHookSQL(cfg, param, filters, "x", &sqlArgs); err != nil {
		return "", nil, err
	}
	for i := range connPreds {
		filters["AND"] = append(filters["AND"], connPreds[i])
		sqlArgs = append(sqlArgs, connArgs[i])
	}

	var mergedFilter []string
	for condition, _ := range filters {
		mergedFilter = append(mergedFilter, fmt.Sprintf(`(%s)`, strings.Join(filters[condition], fmt.Sprintf(` %s `, condition))))
	}

	if len(mergedFilter) > 0 {
		queries = append(queries, fmt.Sprintf(`WHERE %s`, strings.Join(mergedFilter, " AND ")))
	}

	// default sort
	if !returnCount { // limit & Order if not counting
		queries = append(queries, `ORDER BY y.created_at DESC, x.id DESC`)
		queries = append(queries, fmt.Sprintf(`LIMIT %d OFFSET %d`, limit, offset))
	}

	query := strings.Join(queries, " ")

	return query, sqlArgs, nil
}

func BuildCombinedRelationQuery(cfg *models.Config, relationType string, parentModel string, arg *models.CommonSystemParams) (string, []interface{}, *string, error) {

	var local string
	if val, ok := arg.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	var filterArgs []interface{}
	filters, err := ConditionBuilder("x", arg.ResolveParams.Args, arg.Model, &filterArgs)
	if err != nil {
		return "", nil, nil, err
	}
	if err := mergeQueryFilterHookSQL(cfg, arg, filters, "x", &filterArgs); err != nil {
		return "", nil, nil, err
	}

	var mergedFilter []string
	for condition, _ := range filters {
		mergedFilter = append(mergedFilter, fmt.Sprintf(`(%s)`, strings.Join(filters[condition], fmt.Sprintf(` %s `, condition))))
	}

	var relationshipDirection string
	for _, m := range arg.Model.Connections {
		if m.Model == parentModel && m.Type == "backward" {
			relationshipDirection = "to"
		} else if m.Model == parentModel && m.Type == "forward" {
			relationshipDirection = "from"
		}
	}

	if relationshipDirection == "" {
		return "", nil, nil, errors.New("could not decide form/to relations")
	}

	relationInput := map[string]interface{}{}
	if len(arg.ResolveParams.Args) > 0 {
		relationInput = arg.ResolveParams.Args
	} else {
		relationInput = map[string]interface{}{
			"from_model":    parentModel,
			"to_model":      arg.Model.Name,
			"relation_type": relationType,
		}
	}

	keys := arg.DocumentIDs
	if len(keys) == 0 {
		return "", nil, nil, errors.New("BuildCombinedRelationQuery: document ids required")
	}
	keyPH := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	keyArgs := make([]interface{}, len(keys))
	for i, k := range keys {
		keyArgs[i] = k
	}

	selectThing := SelectBuilder("z", local, arg.Model, arg.OnlyReturnCount)
	selectList := strings.Join(selectThing, ", ")

	relationTo := relationInput["to_model"].(string)
	tableName := utility.SingularResourceName(relationTo)

	var query string
	var whereCondition string
	var pivotTable string
	switch relationInput["relation_type"] {
	case "has_many":
		var manyToManyRelation bool
		for _, c := range arg.Model.Connections {
			if c.Model == parentModel {
				if c.Relation == "has_many" {
					manyToManyRelation = true
					break
				}
			}
		}

		var keyField string
		if manyToManyRelation {

			if len(mergedFilter) > 0 {
				whereCondition = fmt.Sprintf(`y.%s_id IN (%s) AND %s`, parentModel, keyPH, strings.Join(mergedFilter, " AND "))
			} else {
				whereCondition = fmt.Sprintf(`y.%s_id IN (%s)`, parentModel, keyPH)
			}

			keyField = fmt.Sprintf(`y.%s_id`, parentModel)

			switch relationshipDirection {
			case "to":
				pivotTable = fmt.Sprintf(`%s_%s`, utility.SingularResourceName(relationInput["from_model"].(string)), utility.SingularResourceName(relationTo))
			case "from":
				pivotTable = fmt.Sprintf(`%s_%s`, utility.SingularResourceName(relationTo), utility.SingularResourceName(relationInput["from_model"].(string)))
			}
			query = fmt.Sprintf(`SELECT %s as key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.id = y.%s_id 
				LEFT JOIN meta AS z ON z.doc_id = x.id
				WHERE %s`, keyField, selectList, pivotTable, tableName, relationTo, whereCondition)
		} else {

			if len(mergedFilter) > 0 {
				whereCondition = fmt.Sprintf(`y.id IN (%s) AND %s`, keyPH, strings.Join(mergedFilter, " AND "))
			} else {
				whereCondition = fmt.Sprintf(`y.id IN (%s)`, keyPH)
			}

			keyField = fmt.Sprintf(`y.id`)

			switch relationshipDirection {
			case "to":
				pivotTable = utility.SingularResourceName(relationInput["from_model"].(string))
			case "from":
				pivotTable = utility.SingularResourceName(relationTo)
			}
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.%s_id = y.id 
				LEFT JOIN meta AS z ON z.doc_id = x.id
				WHERE %s`, keyField, selectList, pivotTable, tableName, relationInput["from_model"].(string), whereCondition)
		}
	case "has_one":
		if len(mergedFilter) > 0 {
			whereCondition = fmt.Sprintf(`y.id IN (%s) AND %s`, keyPH, strings.Join(mergedFilter, " AND "))
		} else {
			whereCondition = fmt.Sprintf(`y.id IN (%s)`, keyPH)
		}

		keyField := `y.id`

		switch relationshipDirection {
		case "to":
			pivotTable = utility.SingularResourceName(relationInput["from_model"].(string))
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.%s_id = y.id 
				LEFT JOIN meta AS z ON z.doc_id = x.id
				WHERE %s LIMIT 1`, keyField, selectList, pivotTable, tableName, relationInput["from_model"].(string), whereCondition)
		case "from":
			pivotTable = utility.SingularResourceName(relationInput["from_model"].(string))
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM  `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.id = y.%s_id 
				LEFT JOIN meta AS z ON z.doc_id = x.id
				WHERE %s LIMIT 1`, keyField, selectList, pivotTable, tableName, relationTo, whereCondition)
		}
	default:
		return "", nil, nil, fmt.Errorf("unsupported relation_type %v", relationInput["relation_type"])
	}

	rt := relationInput["relation_type"].(string)
	outArgs := append(keyArgs, filterArgs...)
	return query, outArgs, &rt, nil
}

func RootResolverMediaQueryBuilder(param *graphql.ResolveParams) (string, error) {

	limit, offset := LimitBuilder(param)
	var queries []string
	queries = append(queries, `SELECT * FROM media AS x`)

	if val, ok := param.Args["model"].(string); ok {
		if val != "" {
			queries = append(queries, fmt.Sprintf(`WHERE x.model = '%s'`, val))
		}
	} else if val, ok := param.Args["search"]; ok {
		queries = append(queries, fmt.Sprintf(`WHERE x.file_name LIKE '%%%s%%'`, val))
	}

	// default sort
	queries = append(queries, `ORDER BY x.created_at DESC`)
	queries = append(queries, fmt.Sprintf(`LIMIT %d OFFSET %d`, limit, offset))

	return strings.Join(queries, " "), nil
}

func BuildCombinedMetaQuery(keys dataloader.Keys, param *QueryBuilderParam) ([]byte, error) {

	queries := make(map[string]string)
	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		metaUserIDs := []string{meta.CreatedBy.ID, meta.LastModifiedBy.ID}
		queries[key.String()] = fmt.Sprintf(`(FOR u IN users FILTER u._key in ['%s'] return u)`, strings.Join(metaUserIDs, `','`))
	}

	query, err := json.Marshal(queries)
	if err != nil {
		return nil, err
	}

	first := bytes.ReplaceAll(query, []byte(`:"(`), []byte(`:(`))
	query = bytes.ReplaceAll(first, []byte(`u)"`), []byte(`u)`))

	return bytes.Join([][]byte{[]byte(`return`), query}, []byte(" ")), nil
}
