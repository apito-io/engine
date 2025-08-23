package helper

import (
	"fmt"
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/vektah/gqlparser/v2/ast"
	"reflect"
	"strings"
)

func AssignEmptyContent(fieldDetails *models.FieldInfo) interface{} {
	switch fieldDetails.InputType {
	case "string":
		return ""
	default:
		return ""
	}
}

func SelectionToFieldBuilder(selections []ast.Selection) []*models.FieldInfo {
	var fields []*models.FieldInfo
	for _, s := range selections {
		f := &models.FieldInfo{
			Identifier:      "Field",
			InputType:       "string",
			FieldType:       "text",
			SystemGenerated: true,
		}
		if _s := s.(*ast.Field).SelectionSet; _s != nil {
			f.SubFieldInfo = SelectionToFieldBuilder(_s)
		}
		fields = append(fields, f)
	}
	return fields
}

func FieldToSelectionBuilder(fields []*models.FieldInfo) ast.SelectionSet {

	actor := ast.SelectionSet{
		&ast.Field{
			Alias: "id",
			Name:  "id",
		},
		&ast.Field{
			Alias: "first_name",
			Name:  "first_name",
		},
		&ast.Field{
			Alias: "role",
			Name:  "role",
		},
		&ast.Field{
			Alias: "email",
			Name:  "email",
		},
		&ast.Field{
			Alias: "avatar",
			Name:  "avatar",
		},
	}

	return ast.SelectionSet{
		&ast.Field{
			Alias: "id",
			Name:  "id",
		},
		&ast.Field{
			Alias:        "data",
			Name:         "data",
			SelectionSet: _fieldToSelectionBuilder(fields),
		},
		&ast.Field{
			Alias: "meta",
			Name:  "meta",
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Alias: "status",
					Name:  "status",
				},
				&ast.Field{
					Alias: "created_at",
					Name:  "created_at",
				},
				&ast.Field{
					Alias: "updated_at",
					Name:  "updated_at",
				},
				&ast.Field{
					Alias:        "created_by",
					Name:         "created_by",
					SelectionSet: actor,
				},
				&ast.Field{
					Alias:        "last_modified_by",
					Name:         "last_modified_by",
					SelectionSet: actor,
				},
			},
		},
	}
}

func _fieldToSelectionBuilder(fields []*models.FieldInfo) ast.SelectionSet {
	var sections []ast.Selection
	for _, f := range fields {
		s := &ast.Field{
			/*Kind: "Field",
			Name: &ast.Name{
				Kind:  "Name",
				Value: f.Identifier,
			},*/
			Alias:            f.Identifier,
			Name:             f.Identifier,
			ObjectDefinition: &ast.Definition{Kind: "Field"},
		}
		// inject for the exception
		switch f.FieldType {
		case _const.MediaField:
			f.SubFieldInfo = []*models.FieldInfo{
				{Identifier: "url", FieldType: "text", InputType: "string"},
				{Identifier: "id", FieldType: "text", InputType: "string"},
				{Identifier: "file_name", FieldType: "text", InputType: "string"},
			}
			break
		case _const.MultilineField:
			f.SubFieldInfo = []*models.FieldInfo{
				{Identifier: "html", FieldType: "text", InputType: "string"},
				{Identifier: "markdown", FieldType: "text", InputType: "string"},
				{Identifier: "text", FieldType: "text", InputType: "string"},
			}
			break
		case _const.GeoField:
			f.SubFieldInfo = []*models.FieldInfo{
				{Identifier: "coordinates", FieldType: "text", InputType: "double"},
				{Identifier: "lat", FieldType: "text", InputType: "string"},
				{Identifier: "lon", FieldType: "text", InputType: "string"},
				{Identifier: "type", FieldType: "text", InputType: "string"},
			}
			break
		}
		if f.SubFieldInfo != nil {
			//Kind:       "SelectionSet",
			s.SelectionSet = _fieldToSelectionBuilder(f.SubFieldInfo)
		}
		sections = append(sections, s)
	}
	return sections
}

func ReturnAggregateBuilder(_var string, set ast.SelectionSet) map[string]*models.FieldDetails {
	fieldInfos := make(map[string]*models.FieldDetails)

	for _, ss := range set {
		x := ss.(*ast.Field)
		if x.Name == "__typename" { // skip all the __typename included by client
			continue
		}
		if (x.Name == "aggregate" || x.Name == "groupBy") && x.SelectionSet != nil { // inject the selections if empty, used for `listSingleModelData` query
			for _, f := range x.SelectionSet {
				xx := f.(*ast.Field)
				fieldInfos[xx.Name] = &models.FieldDetails{
					//Identifier: fmt.Sprintf("`%s`.`%s`", _var, xx.Name),
					Identifier: xx.Name,
					Kind:       reflect.Float64,
					FieldType:  "number",
				}
			}
		}
	}
	return fieldInfos
}

func ReturnBuilder(_var string, local string, _fields *models.FieldDetails, set ast.SelectionSet) map[string]*models.FieldDetails {

	fieldInfos := make(map[string]*models.FieldDetails)
	for _, f := range _fields.SubFields {
		fd := &models.FieldDetails{
			FieldType:  f.FieldType,
			SubFields:  f.SubFieldInfo,
			Validation: f.Validation,
		}
		if f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) || f.Identifier == "data" {
			fd.Local = local
		}

		// type and id should be included by default in any return query
		if _fields.FieldType == "" && utility.ArrayContains([]string{"id", "type", "_key"}, f.Identifier) {
			fd.Value = fmt.Sprintf(`%s.%s`, _var, f.Identifier)
		}

		// include the html query by default
		if _fields.FieldType == "multiline" && f.Identifier == "html" {
			fd.Value = fmt.Sprintf(`%s.html`, _var)
		}

		if _fields.FieldType == "object" && _fields.SubFields == nil {
			continue
		}

		fieldInfos[f.Identifier] = fd
	}

	for _, ss := range set {

		x := ss.(*ast.Field)

		if x.Name == "__typename" { // skip all the __typename included by client
			continue
		}

		if x.Name == "data" && x.SelectionSet == nil { // inject the selections if empty, used for `listSingleModelData` query
			var dataFields []*models.FieldInfo
			for _, f := range _fields.SubFields {
				if f.Identifier == "data" {
					dataFields = f.SubFieldInfo
					break
				}
			}
			//Kind:       "SelectionSet",
			x.SelectionSet = ast.SelectionSet{
				&ast.Field{
					Alias:            "",
					Name:             "",
					Arguments:        nil,
					Directives:       nil,
					SelectionSet:     FieldToSelectionBuilder(dataFields),
					Position:         nil,
					Comment:          nil,
					Definition:       nil,
					ObjectDefinition: &ast.Definition{Kind: "SelectionSet"},
				},
			}
		}

		var fieldDetails *models.FieldDetails
		if val, ok := fieldInfos[x.Name]; ok && val != nil {
			fieldDetails = val
			// inject system subfields for special cases
			switch val.FieldType {
			case _const.MediaField:
				val.Kind = reflect.Map
				val.SubFields = []*models.FieldInfo{
					{Identifier: "url", FieldType: "text", InputType: "string"},
					{Identifier: "id", FieldType: "text", InputType: "string"},
					{Identifier: "file_name", FieldType: "text", InputType: "string"},
				}
				break
			case _const.MultilineField:
				val.Kind = reflect.Map
				val.SubFields = []*models.FieldInfo{
					{Identifier: "html", FieldType: "text", InputType: "string"},
					{Identifier: "markdown", FieldType: "text", InputType: "string"},
					{Identifier: "text", FieldType: "text", InputType: "string"},
				}
				break
			case _const.GeoField:
				val.Kind = reflect.Pointer
				val.SubFields = []*models.FieldInfo{
					{Identifier: "coordinates", FieldType: "text", InputType: "double"},
					{Identifier: "lat", FieldType: "text", InputType: "string"},
					{Identifier: "lon", FieldType: "text", InputType: "string"},
					{Identifier: "type", FieldType: "text", InputType: "string"},
				}
				break
			}
		} else {
			continue
		}

		// object fields does not have any default subfields so ignore it if it
		// does not have any subfields. if not skipped then it will cause panic
		if fieldDetails.FieldType == "object" && fieldDetails.SubFields == nil {
			continue
		}

		if x.SelectionSet != nil {
			var name string
			//if fieldDetails.Local != "" && fieldDetails.Local != "en" {
			//	name = fmt.Sprintf(`%s.%s_%s`, _var, x.Name, fieldDetails.Local)
			//} else {
			name = fmt.Sprintf(`%s.%s`, _var, x.Name)
			//}
			for _, f := range _fields.SubFields {
				if f.Identifier == x.Name && f.SubFieldInfo != nil {
					_fields.SubFields = f.SubFieldInfo
					break
				}
			}
			generated := ReturnBuilder(name, local, fieldDetails, x.SelectionSet)
			fieldInfos[x.Name].Value = generated
		} else {
			var name string
			if fieldDetails.Local != "" && fieldDetails.Local != "en" {
				name = fmt.Sprintf(`%s.%s_%s`, _var, x.Name, fieldDetails.Local)
			} else {
				name = fmt.Sprintf(`%s.%s`, _var, x.Name)
			}
			fieldInfos[x.Name].Value = name
		}
	}
	return fieldInfos
}

func MapApitoFieldType2(fieldInfo *models.FieldInfo) *models.FieldDetails {
	switch fieldInfo.InputType {
	case _const.StringInput:
		switch fieldInfo.FieldType {
		case _const.DateField:
			return &models.FieldDetails{
				Identifier: fieldInfo.Identifier,
				Kind:       reflect.Interface, // date is a special case, it could be single or a range of dates
				FieldType:  fieldInfo.FieldType,
				SubFields:  nil,
			}
		case _const.ListField:
			if !fieldInfo.Validation.IsMultiChoice && len(fieldInfo.Validation.FixedListElements) > 0 { // dropdown
				return &models.FieldDetails{
					Identifier: fieldInfo.Identifier,
					Kind:       reflect.String, // input could be string
					FieldType:  fieldInfo.FieldType,
					Validation: fieldInfo.Validation, // exception case for dropdown where value can be string and a list
					SubFields:  nil,
				}
			} else {
				return &models.FieldDetails{
					Identifier: fieldInfo.Identifier,
					Kind:       reflect.Slice, // input could be a list
					FieldType:  fieldInfo.FieldType,
					SubFields:  nil,
				}
			}
		case _const.MultilineField, _const.MediaField:
			return &models.FieldDetails{
				Kind:      reflect.String,
				FieldType: fieldInfo.FieldType,
				SubFields: nil,
			}
		default:
			return &models.FieldDetails{
				Identifier: fieldInfo.Identifier,
				Kind:       reflect.Interface, // input could be a list or a string || old solution was reflect.String
				FieldType:  fieldInfo.FieldType,
				SubFields:  nil,
			}
		}
	case _const.IntInput:
		return &models.FieldDetails{
			Identifier: fieldInfo.Identifier,
			Kind:       reflect.Interface, // input could be a list or a string || old solution was reflect.Int
			FieldType:  fieldInfo.FieldType,
			SubFields:  nil,
		}
	case _const.DoubleInput:
		return &models.FieldDetails{
			Identifier: fieldInfo.Identifier,
			Kind:       reflect.Interface, // input could be a list or a string || old solution was reflect.Float64
			FieldType:  fieldInfo.FieldType,
			SubFields:  nil,
		}
	case _const.BoolInput:
		return &models.FieldDetails{
			Identifier: fieldInfo.Identifier,
			Kind:       reflect.Bool,
			FieldType:  fieldInfo.FieldType,
			SubFields:  nil,
		}
	case _const.GeoField:
		return &models.FieldDetails{
			Identifier: fieldInfo.Identifier,
			Kind:       reflect.Map,
			FieldType:  fieldInfo.FieldType,
			SubFields:  nil,
		}
	}

	return &models.FieldDetails{
		Identifier: fieldInfo.Identifier,
		Kind:       reflect.Interface, // default case
		FieldType:  fieldInfo.FieldType,
		SubFields:  fieldInfo.SubFieldInfo,
	}
}

/*
	func returnAQLObjectBuilderBk(_var string, _pv string, nestedMedia bool, _map map[string]*FieldDetails) ([]string, string, error) {
		var vals []string
		for k, v := range _map {
			if v.Value != nil {
				switch v.FieldType {
				case "repeated":
					if !contains([]string{"data", "meta", "created_by", "last_modified_by"}, k) { // skip for data object
						_nestedVar := utility.RandomVariableGenerator(4)
						_returns, _pvr, err := returnAQLObjectBuilderBk(_nestedVar, _pv, false, v.Value.(map[string]*FieldDetails))
						if err != nil {
							return nil, "", err
						}
						vals = append(vals, fmt.Sprintf(`"%s" : ( FOR %s in NOT_NULL(%s) ? %s : [] RETURN { %s } )`, k, _nestedVar, _pvr, _pvr, strings.Join(_returns, ", ")))
					} else {
						_returns, _, err := returnAQLObjectBuilderBk(_var, _pv, false, v.Value.(map[string]*FieldDetails))
						if err != nil {
							return nil, "", err
						}
						vals = append(vals, fmt.Sprintf(`"%s" : { %s }`, k, strings.Join(_returns, ", ")))
					}
					break
				case "media", "multiline", "geo":
					if v.Validation != nil && v.Validation.IsGallery { // multiple media is an array
						_nestedVar := utility.RandomVariableGenerator(4)
						_returns, _pvr, err := returnAQLObjectBuilderBk(_nestedVar, _pv, false, v.Value.(map[string]*FieldDetails))
						if err != nil {
							return nil, "", err
						}
						vals = append(vals, fmt.Sprintf(`"%s" : ( FOR %s in NOT_NULL(%s) ? %s : [] RETURN { %s } )`, k, _nestedVar, _pvr, _pvr, strings.Join(_returns, ", ")))
					} else {
						_returns, _pvr, err := returnAQLObjectBuilderBk(_var, _pv, true, v.Value.(map[string]*FieldDetails))
						if err != nil {
							return nil, "", err
						}
						_pv = _pvr
						vals = append(vals, fmt.Sprintf(`"%s" : { %s }`, k, strings.Join(_returns, ", ")))
					}
					break
				default:
					if _var == "" {
						vals = append(vals, fmt.Sprintf(`"%s" : %s`, k, v.Value.(string)))
					} else if nestedMedia { // media in repeated field
						if val, ok := v.Value.(string); ok {
							splits := strings.Split(val, ".")
							end := strings.Join(splits[len(splits)-2:len(splits)], ".")
							vals = append(vals, fmt.Sprintf(`"%s" : %s.%s`, k, _var, end))
							start := strings.Join(splits[:len(splits)-2], ".")
							_pv = start
						}
					} else {
						if val, ok := v.Value.(string); ok {
							splitAt := strings.LastIndex(val, ".")
							vals = append(vals, fmt.Sprintf(`"%s" : %s.%s`, k, _var, val[splitAt+1:len(val)]))
							_pv = val[:splitAt]
						}
					}
				}
			}
		}
		return vals, _pv, nil
	}
*/
func ReturnAQLObjectBuilder(_var string, isArray bool, isParentArray bool, _map map[string]*models.FieldDetails) ([]string, error) {
	var vals []string
	for k, v := range _map {
		// skip if value is nil
		if v.Value == nil {
			continue
		}
		switch v.FieldType {
		case _const.RepeatedField:
			if !utility.ArrayContains([]string{"data", "meta", "created_by", "last_modified_by"}, k) { // skip for data object
				_nestedVar := utility.RandomVariableGenerator(4)
				_returns, err := ReturnAQLObjectBuilder(_nestedVar, true, isArray, v.Value.(map[string]*models.FieldDetails))
				if err != nil {
					return nil, err
				}
				_array := fmt.Sprintf("%s.`%s`", _var, k)
				q := fmt.Sprintf(`%s : ( FOR %s in NOT_NULL(%s) ? %s : [] RETURN { %s } )`, k, _nestedVar, _array, _array, strings.Join(_returns, ", "))
				vals = append(vals, q)
			} else {
				_returns, err := ReturnAQLObjectBuilder(fmt.Sprintf(`%s.%s`, _var, k), false, false, v.Value.(map[string]*models.FieldDetails))
				if err != nil {
					return nil, err
				}
				q := fmt.Sprintf(`%s : { %s }`, k, strings.Join(_returns, ", "))
				vals = append(vals, q)
			}
		case _const.ObjectField:
			if !utility.ArrayContains([]string{"data", "meta", "created_by", "last_modified_by"}, k) { // skip for data object
				// turn var.address in to `var`.`address`
				varSplit := strings.Split(_var, ".")
				_sv := ""
				for _, _v := range varSplit {
					if strings.HasPrefix(_v, "`") { // then dont do it again
						_sv += fmt.Sprintf("%s.", _v)
					} else {
						_sv += fmt.Sprintf("`%s`.", _v)
					}
				}
				_sv = strings.TrimSuffix(_sv, ".")
				// loop
				_nestedVar := fmt.Sprintf("%s.`%s`", _sv, k)
				_m := v.Value.(map[string]*models.FieldDetails)
				_returns, err := ReturnAQLObjectBuilder(_nestedVar, true, isArray, _m)
				if err != nil {
					return nil, err
				}
				q := fmt.Sprintf(`%s : { %s } `, k, strings.Join(_returns, ", "))
				vals = append(vals, q)
			} else {
				_nestedVar := fmt.Sprintf("%s.%s", _var, k)
				_m := v.Value.(map[string]*models.FieldDetails)
				_returns, err := ReturnAQLObjectBuilder(_nestedVar, false, false, _m)
				if err != nil {
					return nil, err
				}
				q := fmt.Sprintf(`%s : { %s }`, k, strings.Join(_returns, ", "))
				vals = append(vals, q)
			}
		case _const.GeoField:
			_nestedVar := fmt.Sprintf("%s.`%s`", _var, k)
			vals = append(vals, fmt.Sprintf(`%s : %s`, k, _nestedVar))
		case _const.MediaField:
			if v.Validation != nil && v.Validation.IsGallery { // multiple media is an array
				/*_nestedVar := utility.RandomVariableGenerator(4)
				_returns, err := ReturnAQLObjectBuilder(_nestedVar, true, isArray, v.Value.(map[string]*models.FieldDetails))
				if err != nil {
					return nil, err
				}
				_array := fmt.Sprintf("%s.`%s`", _var, k)
				q := fmt.Sprintf(`"%s" : ( FOR %s in NOT_NULL(%s) ? %s : [] RETURN { %s } )`, k, _nestedVar, _array, _array, strings.Join(_returns, ", "))
				*/
				// turn var.address in to `var`.`address`
				varSplit := strings.Split(_var, ".")
				_sv := fmt.Sprintf("%s", strings.Join(varSplit, "`.`"))
				q := fmt.Sprintf("%s : `%s`.`%s`", k, _sv, k)
				vals = append(vals, q)
			} else {
				var newVar string
				if isArray {
					newVar = _var
				} else {
					newVar = fmt.Sprintf("%s.%s", _var, k)
				}
				_m := v.Value.(map[string]*models.FieldDetails)
				_returns, err := ReturnAQLObjectBuilder(newVar, false, isArray, _m)
				if err != nil {
					return nil, err
				}
				vals = append(vals, fmt.Sprintf(`%s : { %s }`, k, strings.Join(_returns, ", ")))
			}
			break
		case _const.MultilineField:
			var newVar string
			if isArray {
				newVar = _var
			} else {
				newVar = fmt.Sprintf("%s.`%s`", _var, k)
			}
			_m := v.Value.(map[string]*models.FieldDetails)
			_returns, err := ReturnAQLObjectBuilder(newVar, false, isArray, _m)
			if err != nil {
				return nil, err
			}
			vals = append(vals, fmt.Sprintf(`%s : { %s }`, k, strings.Join(_returns, ", ")))
			break
		default:
			if val, ok := v.Value.(string); ok && v.Value != nil {
				var value string
				varSplit := strings.Split(val, ".")
				if isArray { // its not nested
					//_varSplit := strings.Split(_var, ".")
					end := fmt.Sprintf("`%s`", strings.Join(varSplit[len(varSplit)-1:len(varSplit)], "`.`"))
					//start := strings.Join(_varSplit[:len(_varSplit)-1], ".")
					value = fmt.Sprintf("%s.%s", _var, end)
				} else if !isArray && isParentArray { // its not nested
					//_varSplit := strings.Split(_var, ".")
					end := fmt.Sprintf("`%s`", strings.Join(varSplit[len(varSplit)-2:len(varSplit)], "`.`"))
					//start := strings.Join(_varSplit[:len(_varSplit)-1], ".")
					value = fmt.Sprintf("%s.%s", _var, end)
				} else {
					end := fmt.Sprintf("`%s`", strings.Join(varSplit[len(varSplit)-1:len(varSplit)], "`.`"))
					start := fmt.Sprintf("`%s`", strings.Join(varSplit[:len(varSplit)-1], "`.`"))
					value = fmt.Sprintf("%s.%s", start, end)
				}
				q := fmt.Sprintf(`%s : %s`, k, value)
				vals = append(vals, q)
			}
		}
	}
	/*if len(vals) > 0 {
		// inject id field for gqlgen support
		vals = append(vals, fmt.Sprintf(`"id" : %s.id`, strings.TrimSuffix(_var, ".data")))
	}*/
	return vals, nil
}
