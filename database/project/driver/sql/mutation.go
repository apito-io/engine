package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (S *SQLDriver) DeleteProject(ctx context.Context, projectId string) error {
	sql := fmt.Sprintf(`drop schema %s;`, projectId)
	err := S.Gorm.Exec(sql).Error
	if err != nil {
		return err
	}
	return nil
}

func (S *SQLDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {

	tableName := utility.SingularResourceName(param.Model.Name)
	sql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s;", tableName, param.FieldInfo.Identifier)
	err := S.Gorm.Exec(sql).Error
	//return nil
	if err != nil {
		return err
	}
	return nil
}

func (a *SQLDriver) RemoveAuthAddOns(ctx context.Context, project *models.Project, option map[string]interface{}) error {
	return nil
}

func (S *SQLDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	return nil
}

type Meta2 struct {
	ID    string `gorm:"primaryKey;column:id;not null"`
	DocID string `gorm:"column:doc_id;not null"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:current_date"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:current_date"`

	CreatedBy string `gorm:"column:created_by;not null"`

	UpdatedBy string `gorm:"column:updated_by;not null"`
	Status    string `gorm:"column:status;not null"`
}

func (S *SQLDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) error {

	projectId := param.ProjectID

	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		// create database can not be executed inside transaction so it's outside the transaction
		if err := S.Gorm.Exec(fmt.Sprintf("CREATE DATABASE `%s`", projectId)).Error; err != nil {
			return err
		}

		// reinit the GORM connection
		S.DriverCredential.Database = projectId
		db, err := GetSQLDriver(S.DriverCredential)
		if err != nil {
			return err
		}

		err = db.Gorm.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`
			CREATE TABLE public.meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE,
				updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(36)
			);`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`
			CREATE TABLE public.media(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				model VARCHAR(125),
				media_type VARCHAR(65),
				file_extension VARCHAR(65),
				file_name TEXT,
				size INTEGER,
				s3_key TEXT,
				url TEXT,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE
			);`).Error; err != nil {
				return err
			}
			// return nil will commit the whole transaction
			return nil
		})
		if err != nil {
			return err
		}
	case _const.MySQLDriver, _const.MariaDBDriver:
		err := S.Gorm.Transaction(func(tx *gorm.DB) error {
			// do some database operations in the transaction (use 'tx' from this point, not 'db')
			if err := tx.Exec(fmt.Sprintf("CREATE DATABASE %s", projectId)).Error; err != nil {
				// return any error will rollback
				return err
			}

			var tenantField string
			if param.ProjectType == models.ProjectType_SaaS {
				tenantField = fmt.Sprintf("tenant_id VARCHAR(36) NOT NULL, ADD CONSTRAINT %s_tenant_id_fk FOREIGN KEY (tenant_id) REFERENCES %s(id), ADD CONSTRAINT %s_tenant_composite_key UNIQUE(id, tenant_id);", utility.SingularResourceName(param.Model.Name), param.TenantModel, utility.SingularResourceName(param.Model.Name))
			}

			if err := tx.Exec(fmt.Sprintf(`
			CREATE TABLE %s.meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				%s
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT (CURRENT_DATE),
				updated_at DATE NOT NULL DEFAULT (CURRENT_DATE),
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(35)
			);`, projectId, tenantField)).Error; err != nil {
				return err
			}

			if err := tx.Exec(fmt.Sprintf(`
			CREATE TABLE %s.media(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				%s
				model VARCHAR(125),
				media_type VARCHAR(65),
				file_extension VARCHAR(65),
				file_name TEXT,
				size INTEGER,
				s3_key TEXT,
				url TEXT,
				created_at DATE NOT NULL DEFAULT (CURRENT_DATE)
			);`, projectId, tenantField)).Error; err != nil {
				return err
			}
			// return nil will commit the whole transaction
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *SQLDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	// check if table exists
	query := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'", param.ProjectID, param.Model.Name)
	var count int64
	err := a.Gorm.Raw(query).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (S *SQLDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {

	// if schema not found then create
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			Models: []*models.ModelType{
				model,
			},
		}
	} else {
		var found bool
		for _, ct := range project.Schema.Models {
			if ct.Name == model.Name {
				found = true
				break
			}
		}

		if !found {
			project.Schema.Models = append(project.Schema.Models, model)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	type Fahim struct {
		Name string
	}

	tableName := utility.SingularResourceName(model.Name)

	var createTableQuery string
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		createTableQuery = fmt.Sprintf("`%s`( id VARCHAR(36) NOT NULL PRIMARY KEY )", tableName)
	case _const.MySQLDriver, _const.MariaDBDriver:
		createTableQuery = fmt.Sprintf("`%s`( id VARCHAR(36) NOT NULL PRIMARY KEY )", tableName)
	}

	//Then execute your query for creating table
	query := fmt.Sprintf("CREATE TABLE %s;", createTableQuery)
	err := S.Gorm.Exec(query).Error
	if err != nil {
		return nil, err
	}

	// if project type is saas then add tenant id column
	if project.ProjectType == models.ProjectType_SaaS {
		query = fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN tenant_id VARCHAR(36) NOT NULL, ADD CONSTRAINT %s_tenant_id_fk FOREIGN KEY (tenant_id) REFERENCES %s(id), ADD CONSTRAINT %s_tenant_composite_key UNIQUE(id, tenant_id);",
			tableName,
			utility.SingularResourceName(model.Name),
			project.TenantModelName,
			utility.SingularResourceName(model.Name))
		err = S.Gorm.Exec(query).Error
		if err != nil {
			return nil, err
		}
	}

	return project.Schema, nil
}

// AddRelationFields creates a relation field (has one or has many) between models.
func (s *SQLDriver) AddRelationFields_AUTOGEN(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	// Create pivot table for has_many relations or foreign key for has_one
	fromTable := utility.SingularResourceName(from.Model)
	toTable := utility.SingularResourceName(to.Model)

	if from.Relation == "has_many" {
		// Create pivot table
		pivotTable := fmt.Sprintf("%s_%s", fromTable, toTable)
		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(36) PRIMARY KEY,
				%s_id VARCHAR(36),
				%s_id VARCHAR(36),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (%s_id) REFERENCES %s(id),
				FOREIGN KEY (%s_id) REFERENCES %s(id)
			)
		`, pivotTable, fromTable, toTable, fromTable, fromTable, toTable, toTable)

		err := s.Gorm.Exec(query).Error
		return err
	} else if from.Relation == "has_one" {
		// Add foreign key column
		query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s_id VARCHAR(36)", toTable, fromTable)
		err := s.Gorm.Exec(query).Error
		if err != nil {
			// Column might already exist, ignore error
			return nil
		}

		// Add foreign key constraint
		constraintQuery := fmt.Sprintf(
			"ALTER TABLE `%s` ADD CONSTRAINT fk_%s_%s FOREIGN KEY (%s_id) REFERENCES %s(id)",
			toTable, toTable, fromTable, fromTable, fromTable)
		s.Gorm.Exec(constraintQuery) // Ignore errors for constraints
	}

	return nil
}

func (S *SQLDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {

	toTableName := utility.SingularResourceName(from.Model)
	fromTableName := utility.SingularResourceName(to.Model)
	toFieldName := to.Model
	fromFieldName := from.Model

	// from connection
	switch from.Relation {
	case "has_one":
		switch to.Relation {
		case "has_one":
			// First drop the foreign key constraint
			fkName := fmt.Sprintf("fk_%s_%s", toTableName, toFieldName)
			err := S.Gorm.Migrator().DropConstraint(toTableName, fkName)
			if err != nil {
				return err
			}

			// Then drop the column
			err = S.Gorm.Migrator().DropColumn(toTableName, fmt.Sprintf("%s_id", toFieldName))
			if err != nil {
				return err
			}
		case "has_many":
			//same for one to one & one to many
			query := fmt.Sprintf(`ALTER TABLE `+"`%s`"+` ADD %s_id VARCHAR(36) , 
			ADD CONSTRAINT fk_%s_%s_id FOREIGN KEY (%s_id) references `+"`%s`"+` (id) 
			ON DELETE CASCADE;`, fromTableName, fromFieldName, fromTableName, fromFieldName, fromFieldName, toTableName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		}
	case "has_many":
		switch to.Relation {
		case "has_many":
			//same for one to one & one to many
			query := fmt.Sprintf(`CREATE TABLE `+"`%s_%s`"+`(
				%s_id VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				%s_id VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				PRIMARY KEY (%s_id, %s_id)
			);`, fromTableName, toTableName,
				toFieldName, fromTableName,
				fromFieldName, toTableName,
				toFieldName, fromFieldName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		case "has_one":
			//same for one to one & one to many
			query := fmt.Sprintf("ALTER TABLE `%s` ADD %s_id VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE;", toTableName, toFieldName, fromTableName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteRelationDocuments drops pivot tables, relation keys, or collection tables and all documents within them.
func (s *SQLDriver) DeleteRelationDocuments_AUTOGEN(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	fromTable := utility.SingularResourceName(from.Model)
	toTable := utility.SingularResourceName(to.Model)

	if from.Relation == "has_many" {
		// Drop pivot table
		pivotTable := fmt.Sprintf("%s_%s", fromTable, toTable)
		query := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", pivotTable)
		err := s.Gorm.Exec(query).Error
		return err
	} else if from.Relation == "has_one" {
		// Remove foreign key column
		query := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s_id", toTable, fromTable)
		err := s.Gorm.Exec(query).Error
		return err
	}

	return nil
}

func (S *SQLDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {

	toTableName := utility.SingularResourceName(from.Model)
	fromTableName := utility.SingularResourceName(to.Model)
	toFieldName := to.Model
	fromFieldName := from.Model

	// from connection
	switch from.Relation {
	case "has_one":
		switch to.Relation {
		case "has_one":
			// First drop the foreign key constraint
			fkName := fmt.Sprintf("fk_%s_%s", toTableName, toFieldName)
			err := S.Gorm.Migrator().DropConstraint(toTableName, fkName)
			if err != nil {
				return err
			}

			// Then drop the column
			err = S.Gorm.Migrator().DropColumn(toTableName, fmt.Sprintf("%s_id", toFieldName))
			if err != nil {
				return err
			}
		case "has_many":
			//same for one to one & one to many
			query := fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY fk_%s_%s_id; ALTER TABLE `%s` DROP COLUMN %s_id;", fromTableName, fromTableName, fromFieldName, fromTableName, fromFieldName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		}
	case "has_many":
		switch to.Relation {
		case "has_many":
			//same for one to one & one to many
			query := fmt.Sprintf(`DROP TABLE IF EXISTS %s_%s;`, fromTableName, toTableName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		case "has_one":
			//same for one to one & one to many
			query := fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT %s_id;", toTableName, toFieldName)
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (S *SQLDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {

	if param.FieldInfo.InputType == "geo" {
		return nil, errors.New("geo Field is not supported in PostgreSQL. We will be integrating it soon")
	}

	if parent_field == "" && !isUpdate {
		if (!isUpdate && param.FieldInfo.Serial == 0) || len(param.Model.Fields) == 0 { // new field cant be zero
			param.FieldInfo.Serial = uint32(len(param.Model.Fields) + 1)
		}
		param.Model.Fields = append(param.Model.Fields, param.FieldInfo)
	} else if parent_field != "" {
		for _, f := range param.Model.Fields {
			if f.Identifier == parent_field {
				subField := param.FieldInfo
				var found bool
				for i, s := range f.SubFieldInfo {
					if s.Identifier == param.FieldInfo.Identifier {
						f.SubFieldInfo[i] = subField
						found = true
						break
					}
				}
				if !found {
					subField.Serial = uint32(len(f.SubFieldInfo)) + 1
					f.SubFieldInfo = append(f.SubFieldInfo, subField)
				}
			}
		}
	}

	if parent_field != "" {
		return param.Model, nil // dont create anything
		// todo transform this to one to many relation
	}

	var datatype string
	var validations []string
	switch param.FieldInfo.FieldType {
	case _const.TextField:
		datatype = "TEXT"
	case _const.MultilineField:
		datatype = "TEXT"
	case _const.DateField:
		datatype = "DATE"
	case _const.BooleanField:
		datatype = "BOOLEAN"
	case _const.MediaField:
		if param.FieldInfo.Validation.IsGallery {
			datatype = "JSON"
		} else {
			datatype = "JSON"
		}
	case _const.NumberField:
		switch param.FieldInfo.InputType {
		case "int":
			datatype = "INTEGER"
		case "double":
			datatype = "NUMERIC"
		}
	case _const.ListField:
		if param.FieldInfo.Validation != nil && len(param.FieldInfo.Validation.FixedListElements) > 0 && !param.FieldInfo.Validation.IsMultiChoice {
			datatype = "TEXT"
		} else {
			datatype = "JSON"
		}
	case _const.RepeatedField, _const.ObjectField:
		datatype = "JSON"
	}

	if param.FieldInfo.Validation != nil && param.FieldInfo.Validation.Required {
		var defaultValue interface{}
		switch param.FieldInfo.InputType {
		case _const.StringInput:
			defaultValue = "''"
		case _const.IntInput:
			defaultValue = 0
		case _const.BoolInput:
			defaultValue = false
		case _const.DoubleInput:
			defaultValue = 0.0
		}
		validations = append(validations, fmt.Sprintf("NOT NULL DEFAULT %v", defaultValue))
	} else if param.FieldInfo.Validation != nil && param.FieldInfo.Validation.Unique {
		validations = append(validations, "UNIQUE")
	}

	tableName := utility.SingularResourceName(param.Model.Name)

	// local support
	if param.FieldInfo.Validation != nil && len(param.FieldInfo.Validation.Locals) > 0 {
		for _, local := range param.FieldInfo.Validation.Locals {
			var column string
			if local != "en" {
				column = fmt.Sprintf(`%s_%s`, param.FieldInfo.Identifier, local)
			} else {
				column = param.FieldInfo.Identifier
			}
			//Then execute your query for creating table
			query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN IF NOT EXISTS %s %s %s;", tableName, column, datatype, strings.Join(validations, " "))
			err := S.Gorm.Exec(query).Error
			if err != nil {
				return nil, err
			}
		}
	} else {
		//Then execute your query for creating table
		query := fmt.Sprintf("ALTER TABLE `%s` ADD %s %s %s;", tableName, param.FieldInfo.Identifier, datatype, strings.Join(validations, " "))
		err := S.Gorm.Exec(query).Error
		if err != nil {
			return nil, err
		}
	}

	return param.Model, nil
}

func (S *SQLDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	panic("add team member to project not implemented")
}

func (S *SQLDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberId string) error {
	panic("remove a team member from project not implemented")
}

func (S *SQLDriver) CreateMediaDocument(ctx context.Context, projectId string, media *models.FileDetails) (*models.FileDetails, error) {

	data := map[string]interface{}{
		"id": media.ID,
	}
	if media.UploadParam != nil {
		if media.UploadParam.ModelName != "" {
			data["model"] = media.UploadParam.ModelName
		}
		if media.UploadParam.FieldName != "" {
			data["field"] = media.UploadParam.FieldName
		}
	}
	if media.ContentType != "" {
		data["media_type"] = media.ContentType
	}
	if media.FileExtension != "" {
		data["file_extension"] = media.FileExtension
	}
	if media.FileName != "" {
		data["file_name"] = media.FileName
	}
	if media.Size != 0 {
		data["size"] = media.Size
	}
	if media.S3Key != "" {
		data["s3_key"] = media.S3Key
	}
	if media.URL != "" {
		data["url"] = media.URL
	}

	err := S.Gorm.Table("media").Create(data).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (S *SQLDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {

	err := S.Gorm.Transaction(func(tx *gorm.DB) error {

		tableName := utility.SingularResourceName(param.Model.Name)

		data := map[string]interface{}{
			"id": doc.ID,
		}
		if param.ProjectType == models.ProjectType_SaaS {
			if doc.TenantID != "" {
				data["tenant_id"] = doc.TenantID
			} else {
				return errors.New("tenant id is required for a saas project")
			}
		}
		for k, v := range doc.Data {
			if val, ok := v.(map[string]interface{}); ok {
				if html, ok := val["html"]; ok {
					data[k] = html
				}
			} else {
				data[k] = v
			}
		}
		err := tx.Table(tableName).Create(data).Error
		if err != nil {
			return err
		}

		// now insert a meta data
		metaData := map[string]interface{}{
			"id":         uuid.New().String(),
			"created_by": doc.Meta.CreatedBy.ID,
			"updated_by": doc.Meta.LastModifiedBy.ID,
			"status":     doc.Meta.Status,
			"doc_id":     doc.ID,
		}
		if param.ProjectType == models.ProjectType_SaaS {
			if doc.TenantID != "" {
				data["tenant_id"] = doc.TenantID
			} else {
				return errors.New("tenant id is required for a saas project")
			}
		}
		err = tx.Table("meta").Create(metaData).Error
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (S *SQLDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {

	var multilineFields []string
	var pictureField []string
	var galleryField []string
	var listFields []string
	var repeatedFields []string
	for _, f := range param.Model.Fields {
		if f.FieldType == _const.MultilineField {
			multilineFields = append(multilineFields, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && f.Validation.IsGallery {
			galleryField = append(galleryField, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && !f.Validation.IsGallery {
			pictureField = append(pictureField, f.Identifier)
		} else if f.FieldType == _const.ListField && f.Validation != nil && (len(f.Validation.FixedListElements) == 0 || f.Validation.IsMultiChoice) {
			listFields = append(listFields, f.Identifier)
		} else if f.FieldType == _const.RepeatedField {
			repeatedFields = append(repeatedFields, f.Identifier)
		}
	}

	tableName := utility.SingularResourceName(doc.Type)

	data := map[string]interface{}{}
	for k, v := range doc.Data {
		// if it's a map then it must be a media field
		kind := reflect.ValueOf(v).Kind()
		switch kind {
		case reflect.String, reflect.Int, reflect.Float64, reflect.Bool:
			data[k] = v
			break
		case reflect.Map:
			val := v.(map[string]interface{})
			if utility.ArrayContains(multilineFields, k) {
				if html, ok := val["html"]; ok {
					data[k] = html
				}
			} else if utility.ArrayContains(pictureField, k) {
				b, _ := json.Marshal(v)
				data[k] = datatypes.JSON(b)
			}
			break
		case reflect.Ptr:
			fmt.Println(v)
			break
		case reflect.Slice:
			if utility.ArrayContains(galleryField, k) || utility.ArrayContains(listFields, k) || utility.ArrayContains(repeatedFields, k) {
				b, err := json.Marshal(v)
				if err != nil {
					return err
				}
				data[k] = datatypes.JSON(b)
			}
			break
		default:
			panic("unhandled default case")
		}

	}
	err := S.Gorm.Table(tableName).Where("id = ?", doc.ID).Updates(data).Error
	if err != nil {
		return err
	}

	// now insert a meta data
	metaData := map[string]interface{}{
		"updated_at": utility.GetCurrentTime(),
		"updated_by": param.UserID,
	}

	err = S.Gorm.Table("meta").Where("doc_id = ?", doc.ID).Updates(metaData).Error
	if err != nil {
		return err
	}

	return nil
}

func (S *SQLDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {

	tableName := utility.SingularResourceName(param.Model.Name)

	err := S.Gorm.Table(tableName).Where("id = ?", param.DocumentID).Delete(nil).Error
	if err != nil {
		return err
	}
	return nil
}
