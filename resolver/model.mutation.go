package resolver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) AddModelToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId

	var modelName string
	if val, ok := p.Args["name"].(string); ok {
		modelName = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(val)))
		if modelName == "user" {
			return nil, errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
		} else if modelName == "system" {
			return nil, errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
		} else if modelName == "function" {
			return nil, errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
		}
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	//check if model name starts with number
	var re = regexp.MustCompile(`^\d`)
	matchFound := re.FindAllString(modelName, -1)
	if len(matchFound) > 0 {
		return nil, errors.New("model name can not start with a number! use character instead")
	}

	checkProjectExists, err := s.GraphQLExecutor.GetProjectDriver(ctx).CheckCollectionExists(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	if !checkProjectExists {
		return nil, errors.New("project not found to create a model")
	}

	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	var singleRecord bool
	if val, ok := p.Args["single_record"].(bool); ok {
		singleRecord = val
	}

	// if schema not found then create
	project.Schema, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddModel(p.Context, project, modelName, singleRecord)
	if err != nil {
		return nil, err
	}

	err = s.SystemDriver.UpdateProject(p.Context, project, false)
	if err != nil {
		return nil, err
	}

	return project.Schema.Models, nil
}

func (s *GraphQLServer) UpdateModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	if project == nil {
		return nil, errors.New("project not found to create a model")
	}

	var _type string
	if val, ok := p.Args["type"].(string); ok {
		_type = val
	} else {
		return nil, errors.New("type not found")
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var resp interface{}
	switch _type {
	case "duplicate":
		var newName string
		if val, ok := p.Args["new_name"].(string); ok {
			newName = val
		} else {
			return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
		}
		resp, err = s.duplicateModel(p.Context, project, newName, modelName)
	case "rename":
		var newName string
		if val, ok := p.Args["new_name"].(string); ok {
			newName = val
		} else {
			return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
		}
		resp, err = s.renameModel(p.Context, project, newName, modelName)
	case "convert":
		resp, err = s.convertModel(p.Context, project, modelName)
	case "delete":
		resp, err = s.deleteModel(p.Context, project, modelName)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *GraphQLServer) duplicateModel(ctx context.Context, project *protobuff.Project, newName, modelName string) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var newModelName string

	newModelName = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(newName)))
	if newModelName == "user" {
		return nil, errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
	} else if newModelName == "system" {
		return nil, errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
	} else if newModelName == "function" {
		return nil, errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
	}

	var duplicatedModel *protobuff.ModelType
	var err error

	// if schema not found then create
	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		var modelToDuplicate *protobuff.ModelType
		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToDuplicate = ct
				break
			}
		}

		if modelToDuplicate == nil {
			return nil, errors.New("the model about to be duplicated, not found")
		}

		var found bool
		for _, ct := range project.Schema.Models {
			if ct.Name == newModelName {
				found = true
				break
			}
		}

		if !found {
			duplicatedModel = &protobuff.ModelType{
				Name:            newModelName,
				Fields:          modelToDuplicate.Fields,
				Connections:     modelToDuplicate.Connections,
				HookIds:         modelToDuplicate.HookIds,
				Locals:          modelToDuplicate.Locals,
				RepeatedGroups:  modelToDuplicate.RepeatedGroups,
				SystemGenerated: modelToDuplicate.SystemGenerated,
				HasConnections:  modelToDuplicate.HasConnections,
			}
			if modelToDuplicate.SinglePageUuid != "" { // assign new id
				uid := uuid.New()
				duplicatedModel.SinglePage = true
				duplicatedModel.SinglePageUuid = uid.String()
			}
			project.Schema.Models = append(project.Schema.Models, duplicatedModel)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	err = s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return duplicatedModel, nil
}

func (s *GraphQLServer) renameModel(ctx context.Context, project *protobuff.Project, newName, modelName string) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
	}

	if newName == modelName {
		return nil, errors.New("new model name can not be the same as the old one")
	}

	var newModelName string

	newModelName = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(newName)))
	if newModelName == "user" {
		return nil, errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
	} else if newModelName == "system" {
		return nil, errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
	} else if newModelName == "function" {
		return nil, errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
	}

	var modelToRename *protobuff.ModelType
	var err error

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToRename = ct
				break
			}
		}

		if modelToRename == nil {
			return nil, errors.New("the model about to be renamed, not found")
		}

		if len(modelToRename.Connections) > 0 {
			return nil, errors.New("can not rename model because it has relations with other models")
		}

		// check if the models has documents

		// rename
		modelToRename.Name = newModelName
	}

	err = s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return modelToRename, nil
}

func (s *GraphQLServer) convertModel(ctx context.Context, project *protobuff.Project, modelName string) (interface{}, error) {

	var modelToConvert *protobuff.ModelType

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToConvert = ct
				break
			}
		}

		if modelToConvert == nil {
			return nil, errors.New("the model about to be renamed, not found")
		}

		if len(modelToConvert.Connections) > 0 {
			return nil, errors.New("can not convert model because it has relations with other models")
		}

		// check if the models has documents

		// convert
		if modelToConvert.SinglePage {
			// remove
			modelToConvert.SinglePage = false
			modelToConvert.SinglePageUuid = ""
		} else {
			// assign new id
			uid := uuid.New()
			modelToConvert.SinglePage = true
			modelToConvert.SinglePageUuid = uid.String()
		}
	}

	err := s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return modelToConvert, nil
}

func (s *GraphQLServer) deleteModel(ctx context.Context, project *protobuff.Project, modelName string) (interface{}, error) {

	if modelName == "user" {
		return nil, errors.New("can not delete User Model. If you dont want it then remove User Addons")
	}

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("nothing to Delete")
	} else {
		var index int
		var _model *protobuff.ModelType
		for i, ct := range project.Schema.Models {
			if ct.Name == modelName {
				_model = ct
				index = i
				break
			}
		}

		if _model != nil {
			project.Schema.Models = append(project.Schema.Models[:index], project.Schema.Models[index+1:]...)
			// delete all the data connected to this model
			err := s.GraphQLExecutor.GetProjectDriver(ctx).DeleteDocumentsFromProject(ctx, shared.CommonSystemParams{ProjectId: project.Id, Model: _model})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("could not find model to delete")
		}
	}

	// also remove all its relations

	for _, m := range project.Schema.Models {
		for i, c := range m.Connections {
			if c.Model == modelName {
				m.Connections = append(m.Connections[:i], m.Connections[i+1:]...)
			}
		}
		if len(m.Connections) == 0 {
			m.Connections = nil
		}
	}

	err := s.SystemDriver.UpdateProject(ctx, project, true)
	if err != nil {
		return nil, err
	}

	return project.Schema.Models[len(project.Schema.Models)-1], nil
}

func (s *GraphQLServer) DeleteFieldTypeResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId
	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = strings.TrimSpace(inflection.Singular(val))
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}
	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	param.Model = modelType
	identifier := p.Args["identifier"].(string)

	if isRelation, ok := p.Args["is_relation"].(bool); ok && isRelation {

		// struct the connection type before removing from schema
		var fromConnectionType *protobuff.ConnectionType
		// delete the forward relation
		for i, r := range modelType.Connections {
			if r.Model == identifier {
				fromConnectionType = r
				modelType.Connections = append(modelType.Connections[:i], modelType.Connections[i+1:]...)
				break
			}
		}
		if len(modelType.Connections) == 0 {
			modelType.Connections = nil
		}

		// struct the connection type before removing from schema
		var toConnectionModel *protobuff.ConnectionType

		// delete the backward relation
		for _, ct := range project.Schema.Models {
			if ct.Name == identifier {
				for i, r := range ct.Connections {
					if r.Model == modelName {
						toConnectionModel = r
						ct.Connections = append(ct.Connections[:i], ct.Connections[i+1:]...)
						break
					}
				}
				if len(ct.Connections) == 0 {
					ct.Connections = nil
				}
				break
			}
		}

		// drop it from db
		param.FieldInfo = &protobuff.FieldInfo{Identifier: identifier}
		err := s.GraphQLExecutor.GetProjectDriver(ctx).DropConnections(p.Context, param.ProjectId,
			fromConnectionType,
			toConnectionModel,
		)
		if err != nil {
			return nil, err
		}
	} else {

		var repeatedGroupIdentifier *string
		if val, ok := p.Args["repeated_group_identifier"].(string); ok {
			if val != "_root" { // skip if root
				repeatedGroupIdentifier = &val
			}
		}

		var deletedIndex uint32
		for i, f := range modelType.Fields {
			if f.Identifier == identifier && repeatedGroupIdentifier == nil {
				deletedIndex = f.Serial
				modelType.Fields = append(modelType.Fields[:i], modelType.Fields[i+1:]...)
				// drop it from db
				param.FieldInfo = f
				err := s.GraphQLExecutor.GetProjectDriver(ctx).DropField(p.Context, *param)
				if err != nil {
					return nil, err
				}
				break
			} else if f.SubFieldInfo != nil && repeatedGroupIdentifier != nil && f.Identifier == *repeatedGroupIdentifier {
				for i, sf := range f.SubFieldInfo {
					if sf.Identifier == identifier {
						deletedIndex = sf.Serial
						// drop it from db
						param.FieldInfo = &protobuff.FieldInfo{
							Identifier:      sf.Identifier,
							Description:     sf.Description,
							InputType:       sf.InputType,
							Validation:      sf.Validation,
							Serial:          sf.Serial,
							Label:           sf.Label,
							SystemGenerated: sf.SystemGenerated,
							ParentField:     *repeatedGroupIdentifier,
						}
						if repeatedGroupIdentifier != nil {
							param.FieldInfo.FieldType = f.FieldType // pass parent field type [repeated, object]
						} else {
							param.FieldInfo.FieldType = sf.FieldType
						}
						f.SubFieldInfo = append(f.SubFieldInfo[:i], f.SubFieldInfo[i+1:]...)
						err := s.GraphQLExecutor.GetProjectDriver(ctx).DropField(p.Context, *param)
						if err != nil {
							return nil, err
						}
						break
					}
				}
			}
		}
		// rearrange the serial after to delete
		if deletedIndex > 0 && repeatedGroupIdentifier == nil {
			for _, f := range modelType.Fields {
				if f.Serial > deletedIndex {
					f.Serial = f.Serial - 1
				}
			}
		} else if deletedIndex > 0 && repeatedGroupIdentifier != nil {
			for _, f := range modelType.Fields {
				if f.Identifier == *repeatedGroupIdentifier {
					for _, sf := range f.SubFieldInfo {
						if sf.Serial > deletedIndex {
							sf.Serial = sf.Serial - 1
						}
					}
				}
			}
		}
	}

	err = s.SystemDriver.UpdateProject(ctx, project, true)
	if err != nil {
		return nil, err
	}

	// expire the project cache
	err = s.ExpireGraphQLProjectCache(ctx, projectId)
	if err != nil {
		return nil, err
	}

	return modelType, nil
}

func (s *GraphQLServer) UpsertFieldToModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId
	project, err := s.SystemDriver.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var identifier string
	var label string
	if val, ok := p.Args["field_label"].(string); ok {
		label = strings.TrimSpace(val)
		m := regexp.MustCompile("[^A-Za-z0-9]+")
		identifier = strings.TrimSpace(strings.ToLower(m.ReplaceAllString(label, "_")))
		// check for valid field name. Restrict a few
		if strings.HasPrefix(identifier, "_") {
			return nil, errors.New("field can not begin with _")
		}
		if utility.ArrayContains([]string{"id", "_id"}, identifier) {
			return nil, errors.New(fmt.Sprintf("Field %s is auto generated by the System. No Need to define It", identifier))
		} else if utility.ArrayContains([]string{"status"}, identifier) {
			return nil, errors.New("status field is auto generated and reserved for document publishing status in the API. Choose other name instead")
		} else if strings.HasPrefix(identifier, "sys_") {
			return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
		}

		//check if model name starts with number
		var re = regexp.MustCompile(`^\d`)
		matchFound := re.FindAllString(identifier, -1)
		if len(matchFound) > 0 {
			return nil, errors.New("field name can not start with a number! use character instead")
		}

	} else {
		return nil, errors.New("field Label Is necessary")
	}

	var repeatedGroupIdentifier *string
	if val, ok := p.Args["repeated_group_identifier"].(string); ok {
		if val != "_root" {
			repeatedGroupIdentifier = &val
		}
	}

	var isUpdate bool
	if val, ok := p.Args["is_update"].(bool); ok {
		isUpdate = val
	}

	// now search for fields
	var fieldInfo *protobuff.FieldInfo
	for _, f := range modelType.Fields {
		if f.Identifier == identifier && repeatedGroupIdentifier == nil {
			fieldInfo = f
			break
		} else if repeatedGroupIdentifier != nil && f.Identifier == *repeatedGroupIdentifier && f.SubFieldInfo != nil {
			for _, sf := range f.SubFieldInfo {
				if sf.Identifier == identifier {
					fieldInfo = sf
					break
				}
			}
		}
	}

	if !isUpdate && fieldInfo != nil {
		return nil, errors.New(fmt.Sprintf("A field with identifier '%s' already exits", identifier))
	}

	if fieldInfo == nil {
		fieldInfo = &protobuff.FieldInfo{
			Identifier: identifier,
			Label:      label,
			Serial:     uint32(len(modelType.Fields) + 1),
		}
	}

	if val, ok := p.Args["is_object_field"].(bool); ok {
		fieldInfo.IsObjectField = val
	}

	if val, ok := p.Args["field_type"].(string); ok {
		fieldInfo.FieldType = val
	}

	if val, ok := p.Args["input_type"].(string); ok {
		fieldInfo.InputType = val
	}

	// validate field & input type combination and other validation
	switch fieldInfo.FieldType {
	case "geo":
		if fieldInfo.InputType != "geo" {
			return nil, errors.New("input Type must be Geo if Field Type is Geo")
		}
		break
	case "repeated":
		fieldInfo.SubFieldInfo = []*protobuff.FieldInfo{
			&protobuff.FieldInfo{
				Identifier:   "_id",
				Description:  "An Auto Generated UUIDv4 Unique Identifier",
				InputType:    "string",
				FieldType:    "text",
				SubFieldInfo: nil,
				Validation: &protobuff.Validation{
					Hide:   true,
					Unique: true,
				},
				Serial:                  1,
				Label:                   "ID",
				SystemGenerated:         true,
				RepeatedGroupIdentifier: fieldInfo.Identifier,
			},
		}
	}

	if val, ok := p.Args["serial"].(int); ok {
		fieldInfo.Serial = uint32(val)
	}

	if val, ok := p.Args["field_description"].(string); ok {
		fieldInfo.Description = val
	}

	if val, ok := p.Args["validation"].(map[string]interface{}); ok {
		validation := val
		fieldInfo.Validation = &protobuff.Validation{}
		if v, ok := validation["required"].(bool); ok {
			fieldInfo.Validation.Required = v
		}

		if v, ok := validation["as_title"].(bool); ok {
			fieldInfo.Validation.AsTitle = v
		}

		if v, ok := validation["hide"].(bool); ok {
			fieldInfo.Validation.Hide = v
		}

		if v, ok := validation["is_email"].(bool); ok {
			fieldInfo.Validation.IsEmail = v
		}

		if v, ok := validation["is_gallery"].(bool); ok {
			fieldInfo.Validation.IsGallery = v
		}

		if v, ok := validation["is_url"].(bool); ok {
			fieldInfo.Validation.IsUrl = v
		}

		if v, ok := validation["unique"].(bool); ok {
			fieldInfo.Validation.Unique = v
		}

		if v, ok := validation["is_multi_choice"].(bool); ok {
			fieldInfo.Validation.IsMultiChoice = v
		}

		if v, ok := validation["placeholder"].(string); ok {
			fieldInfo.Validation.Placeholder = v
		}

		if vals, ok := validation["locals"].([]interface{}); ok {
			var elements []string
			for _, v := range vals {
				elements = append(elements, v.(string))
			}
			fieldInfo.Validation.Locals = elements
		}

		if v, ok := validation["list_type"].(string); ok {
			fieldInfo.Validation.ListType = v
		}

		if vals, ok := validation["fixed_list_elements"].([]interface{}); ok {
			var elements []string
			for _, v := range vals {
				elements = append(elements, v.(string))
			}
			fieldInfo.Validation.FixedListElements = elements
		}

	}

	param.Model = modelType
	param.FieldInfo = fieldInfo

	modelType, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddFieldToModel(p.Context, *param, isUpdate, repeatedGroupIdentifier)
	if err != nil {
		return nil, err
	}

	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	if repeatedGroupIdentifier != nil {
		fieldInfo.RepeatedGroupIdentifier = *repeatedGroupIdentifier
	}

	return fieldInfo, nil
}

func (s *GraphQLServer) RearrangeFieldOfModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	for _, ct := range cache.Project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	var oldSerial uint32
	if val, ok := p.Args["old_serial"].(int); ok {
		oldSerial = uint32(val)
	} else {
		return nil, errors.New("old serial is required")
	}

	var newSerial uint32
	if val, ok := p.Args["new_serial"].(int); ok {
		newSerial = uint32(val)
	} else {
		return nil, errors.New("new serial is required")
	}

	var parentField string
	if val, ok := p.Args["parent_field"].(string); ok {
		parentField = val
	} else {
		return nil, errors.New("parent_field is required")
	}

	var fieldName string
	if val, ok := p.Args["field_name"].(string); ok {
		fieldName = val
	} else {
		return nil, errors.New("field_name is required")
	}

	fmt.Println(fieldName)

	// now search for fields
	for _, f := range modelType.Fields {
		if parentField == "_root" { // root field rearrange
			if f.Serial == oldSerial {
				f.Serial = newSerial
			} else {
				if newSerial < oldSerial && (f.Serial >= newSerial && f.Serial < oldSerial) { // bottom up
					f.Serial = f.Serial + 1
				} else if newSerial > oldSerial && (f.Serial > oldSerial && f.Serial <= newSerial) { // top down
					f.Serial = f.Serial - 1
				}
			}
		} else if parentField != "_root" && f.SubFieldInfo != nil { // it's an array/object field rearrange
			if f.Identifier == parentField {
				for _, sf := range f.SubFieldInfo {
					if sf.Serial == oldSerial {
						sf.Serial = newSerial
					} else {
						if newSerial < oldSerial && (sf.Serial >= newSerial && sf.Serial < oldSerial) { // bottom up
							sf.Serial = sf.Serial + 1
						} else if newSerial > oldSerial && (sf.Serial > oldSerial && sf.Serial <= newSerial) { // top down
							sf.Serial = sf.Serial - 1
						}
					}
				}
				sort.Slice(f.SubFieldInfo, func(i, j int) bool {
					return f.SubFieldInfo[i].Serial < f.SubFieldInfo[j].Serial
				})
				break
			}
		}
	}

	// rearrange others
	sort.Slice(modelType.Fields, func(i, j int) bool {
		return modelType.Fields[i].Serial < modelType.Fields[j].Serial
	})

	cache.Param.Model = modelType

	project, err := s.SystemDriver.GetProject(p.Context, cache.Project.Id)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema
	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	return modelType, nil
}

func (s *GraphQLServer) ModelFieldOperationResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	var _type enums.FieldOperation
	if val, ok := p.Args["type"].(enums.FieldOperation); ok {
		_type = val
	}

	var fieldName string
	if val, ok := p.Args["field_name"].(string); ok {
		fieldName = val
	} else {
		return nil, errors.New("field name is required")
	}

	var repeatedGroupIdentifier *string
	if val, ok := p.Args["repeated_group_identifier"].(string); ok {
		if val != "_root" {
			repeatedGroupIdentifier = &val
		}
	}

	var fieldInfo *protobuff.FieldInfo

	switch _type {
	case enums.FieldOperation_Rename:
		var label string
		var newIdentifier string
		if val, ok := p.Args["new_name"].(string); ok {
			name := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(val, " ", "_")))
			// check for valid field name. Restrict a few
			if name == "id" {
				return nil, errors.New("field ID is auto generated by the System. No Need to define It")
			} else if strings.HasPrefix(name, "sys_") {
				return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
			}
			label = val
			newIdentifier = name
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// now search for fields
		for _, f := range modelType.Fields {
			if f.Identifier == fieldName && repeatedGroupIdentifier == nil {
				fieldInfo = f
				// rename this
				fieldInfo.Identifier = newIdentifier
				fieldInfo.Label = label
				break
			} else if repeatedGroupIdentifier != nil && f.Identifier == *repeatedGroupIdentifier && f.SubFieldInfo != nil {
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == fieldName {
						fieldInfo = &protobuff.FieldInfo{
							Identifier:      newIdentifier,
							Description:     sf.Description,
							InputType:       sf.InputType,
							FieldType:       sf.FieldType,
							Validation:      sf.Validation,
							Serial:          sf.Serial,
							Label:           label,
							SystemGenerated: sf.SystemGenerated,
						}
						// rename this
						sf.Identifier = newIdentifier
						sf.Label = label
					}
				}
			}
		}

		if fieldInfo == nil {
			return nil, errors.New("field not found to Request")
		}

		param := *cache.Param
		param.Model = modelType
		param.FieldInfo = fieldInfo

		if fieldName != param.FieldInfo.Identifier { // skip renaming if the same value is given
			err := s.GraphQLExecutor.GetProjectDriver(ctx).RenameField(p.Context, fieldName, repeatedGroupIdentifier, param)
			if err != nil {
				return nil, err
			}
		}
	case enums.FieldOperation_Duplicate:
		var label string
		var newIdentifier string
		if val, ok := p.Args["new_name"].(string); ok {
			name := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(val, " ", "_")))
			// check for valid field name. Restrict a few
			if name == "id" {
				return nil, errors.New("field ID is auto generated by the System. No Need to define It")
			} else if strings.HasPrefix(name, "sys_") {
				return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
			}
			label = val
			newIdentifier = name
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// check for duplicate
		for _, f := range modelType.Fields {
			if f.Identifier == newIdentifier && repeatedGroupIdentifier == nil {
				return nil, errors.New("field with that name already exists")
			} else if repeatedGroupIdentifier != nil && f.Identifier == *repeatedGroupIdentifier && f.SubFieldInfo != nil {
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == fieldName {
						return nil, errors.New("sub field with that name already exists")
					}
				}
			}
		}

		var newFieldInfo protobuff.FieldInfo
		// now search for fields
		for _, f := range modelType.Fields {
			if f.Identifier == fieldName && repeatedGroupIdentifier == nil {
				newFieldInfo = *f
				// add the new info
				newFieldInfo.Identifier = newIdentifier
				newFieldInfo.Label = label
				break
			} else if repeatedGroupIdentifier != nil && f.Identifier == *repeatedGroupIdentifier && f.SubFieldInfo != nil {
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == fieldName {
						newFieldInfo = protobuff.FieldInfo{
							Identifier:      newIdentifier,
							Description:     sf.Description,
							InputType:       sf.InputType,
							FieldType:       sf.FieldType,
							Validation:      sf.Validation,
							Serial:          sf.Serial,
							Label:           label,
							SystemGenerated: sf.SystemGenerated,
						}
					}
				}
			}
		}

		if newFieldInfo.Identifier == "" {
			return nil, errors.New("could not duplicate field. Invalid Request")
		}

		// add the new field data
		newFieldInfo.Serial = uint32(len(modelType.Fields) + 1) // overwrite the serial
		modelType.Fields = append(modelType.Fields, &newFieldInfo)

		param := *cache.Param
		param.Model = modelType
		param.FieldInfo = &newFieldInfo

		modelType, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddFieldToModel(p.Context, param, true, repeatedGroupIdentifier)
		if err != nil {
			return nil, err
		}

		if repeatedGroupIdentifier != nil {
			fieldInfo.RepeatedGroupIdentifier = *repeatedGroupIdentifier
		}

		// for response
		fieldInfo = &newFieldInfo

	}

	project, err = s.SystemDriver.GetProject(p.Context, project.Id)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema
	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	// expire the cache
	err = s.ExpireGraphQLFieldCache(ctx, project.Id, modelType.Name)
	if err != nil {
		return nil, err
	}

	return fieldInfo, nil
}

func (s *GraphQLServer) UpsertModelDataFnFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p

	project, err := s.SystemDriver.GetProject(ctx, param.ProjectId)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = strings.TrimSpace(inflection.Singular(val))
	} else {
		return nil, errors.New("model Name is Necessary")
	}

	if strings.Contains(modelName, " ") {
		return nil, errors.New("space is not allowed in Model Name")
	}

	var modelType *protobuff.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New("model Not Found")
	}

	param.Model = modelType

	collectionName := fmt.Sprintf("p_%s", param.ProjectId)

	var _status string
	if val, ok := p.Args["status"].(string); ok {
		_status = val
	}

	var isFaker bool
	if val, ok := p.Args["faker"].(bool); ok {
		isFaker = val
	}

	var doc *shared.DefaultDocumentStructure

	if val, ok := p.Args["_id"]; ok {

		var isSinglePageData bool
		if val, ok := p.Args["single_page_data"].(bool); ok {
			isSinglePageData = val
		}

		param.DocumentId = val.(string)
		param.ResolveParams = &p
		param.Model = modelType
		param.SinglePageData = isSinglePageData

		raw, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleRawDocumentFromProject(p.Context, *param)
		if err != nil {
			return nil, err
		}
		doc = raw.(*shared.DefaultDocumentStructure)

		// got the doc but doc doesn't belong to specific model
		if doc.Type != modelName {
			return nil, errors.New(fmt.Sprintf("Id does not belongs to %s", modelName))
		}

		if len(modelType.Connections) > 0 {
			if disconnects, ok := p.Args["disconnect"].(map[string]interface{}); ok && len(disconnects) > 0 {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(p.Context, cache.Project.Schema, param.DocumentId, collectionName, disconnects, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = s.GraphQLExecutor.GetProjectDriver(ctx).DisconnectBuilder(p.Context, *param)
				if err != nil {
					return nil, err
				}
			}

			if connectionIds, ok := p.Args["connect"].(map[string]interface{}); ok && len(connectionIds) > 0 {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(p.Context, cache.Project.Schema, param.DocumentId, collectionName, connectionIds, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = s.GraphQLExecutor.GetProjectDriver(ctx).ConnectBuilder(p.Context, *param)
				if err != nil {
					return nil, err
				}
			}
		}

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			// local support
			local := "en"
			if val, ok := input["local"].(string); ok {
				local = val
			}

			//#todo need image param validation
			modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(p.Context, param, false, local, modelType.Fields, inputPayload, doc.Data)
			if err != nil {
				return nil, err
			}
			doc.Data = modifiedPayload
		}

		// replacing the doc might case the local field to disappear.. don't replace the old doc
		// fixed it later !!
		err = s.GraphQLExecutor.GetProjectDriver(ctx).UpdateDocumentOfProject(p.Context, *param, doc, true)
		if err != nil {
			return nil, err
		}

	} else {

		//#todo replace these operation with transaction

		id := uuid.New()
		uid := id.String()

		doc = &shared.DefaultDocumentStructure{
			Id:   uid,
			Key:  uid,
			Type: modelName,
			Meta: &protobuff.MetaField{
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
				CreatedBy: &protobuff.SystemUser{
					Id: param.UserId,
				},
				LastModifiedBy: &protobuff.SystemUser{
					Id: param.UserId,
				},
				Status: _status,
			},
		}

		local := "en"
		var inputPayload map[string]interface{}
		var numberOfRecords int

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			input := param.ResolveParams.Args
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val

			}
			// local support
			if val, ok := input["local"].(string); ok {
				local = val
			}

			if val, ok := inputPayload["number_of_records"].(float64); ok {
				numberOfRecords = int(val)
			}
		}

		if isFaker {

			if numberOfRecords > 100 {
				return nil, errors.New("The limit to Generate Fake data is between 1 - 100")
			}

			wg := sync.WaitGroup{}
			wg.Add(numberOfRecords)

			for i := 0; i < numberOfRecords; i++ {
				modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(p.Context, param, isFaker, local, modelType.Fields, inputPayload, make(map[string]interface{}))
				if err != nil {
					return nil, err
				}
				doc.Data = modifiedPayload

				id := uuid.New()
				doc.Key = id.String()
				doc.Id = doc.Key

				_, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(p.Context, param.ProjectId, modelName, doc)
				if err != nil {
					return nil, err
				}

			}

		} else {

			//#todo need image param validation
			modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(p.Context, param, isFaker, local, modelType.Fields, inputPayload, make(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			doc.Data = modifiedPayload

			_, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(p.Context, param.ProjectId, modelName, doc)
			if err != nil {
				return nil, err
			}

			// for new document also check for connect disconnect
			if len(modelType.Connections) > 0 {
				if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
					v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(p.Context, cache.Project.Schema, uid, collectionName, connections, modelType)
					if err != nil {
						// if relation error at first then remove the document
						param.DocumentId = doc.Id
						err = s.GraphQLExecutor.GetProjectDriver(ctx).DeleteDocumentFromProject(p.Context, *param)
						return nil, err
					}
					param.ConDisParam = v
					err = s.GraphQLExecutor.GetProjectDriver(ctx).ConnectBuilder(p.Context, *param)
					if err != nil {
						// if relation error at first then remove the document
						param.DocumentId = doc.Id
						err = s.GraphQLExecutor.GetProjectDriver(ctx).DeleteDocumentFromProject(p.Context, *param)
						return nil, err
					}
				}
			}
		}
	}

	// add the meta
	docWithMeta, err := s.SystemDriver.AddSystemUserMetaInfo(p.Context, doc)
	if err != nil {
		return nil, err
	}

	return docWithMeta, nil
}

func (s *GraphQLServer) DeleteModelDataFnFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p

	projectId := param.ProjectId

	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		modelName = strings.TrimSpace(inflection.Singular(val))
	} else {
		return nil, errors.New("Model Name is Necessary")
	}

	var modelType *protobuff.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New("Model Not Found")
	}

	var docId string
	if val, ok := p.Args["_id"]; ok {
		docId = val.(string)
		param.DocumentId = docId
		param.ResolveParams = &p
		param.Model = modelType
		exists, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(p.Context, *param)
		if err != nil {
			return nil, err
		}

		if exists != nil {
			err = s.GraphQLExecutor.GetProjectDriver(ctx).DeleteDocumentFromProject(p.Context, *param)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("doc not found to delete")
		}
	} else {
		return nil, errors.New("_id is required for delete")
	}

	return map[string]interface{}{
		"id": docId,
	}, nil
}
