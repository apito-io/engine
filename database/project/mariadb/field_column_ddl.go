package mariadb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// fieldSQLDataType maps Apito field metadata to a portable SQL column type for ALTER TABLE ... ADD.
func fieldSQLDataType(f *models.FieldInfo) (string, error) {
	if f == nil {
		return "", errors.New("nil FieldInfo")
	}
	switch f.FieldType {
	case _const.TextField:
		return "TEXT", nil
	case _const.MultilineField:
		return "TEXT", nil
	case _const.DateField:
		return "DATE", nil
	case _const.BooleanField:
		return "BOOLEAN", nil
	case _const.MediaField:
		if f.Validation != nil && f.Validation.IsGallery {
			return "JSON", nil
		}
		return "JSON", nil
	case _const.NumberField:
		switch f.InputType {
		case "int":
			return "INTEGER", nil
		case "double":
			return "NUMERIC", nil
		default:
			return "", fmt.Errorf("unsupported number input_type %q for SQL column", f.InputType)
		}
	case _const.ListField:
		if f.Validation != nil && len(f.Validation.FixedListElements) > 0 && !f.Validation.IsMultiChoice {
			return "TEXT", nil
		}
		return "JSON", nil
	case _const.RepeatedField, _const.ObjectField:
		return "JSON", nil
	default:
		return "", fmt.Errorf("unsupported FieldType %q for SQL column", f.FieldType)
	}
}

func fieldSQLValidations(f *models.FieldInfo) []string {
	if f == nil || f.Validation == nil {
		return nil
	}
	var validations []string
	if f.Validation.Required {
		var defaultValue interface{}
		switch f.InputType {
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
	}
	if f.Validation.Unique {
		validations = append(validations, "UNIQUE")
	}
	return validations
}

// AlterTableAddFieldSQL returns DDL statements to add physical column(s) for one FieldInfo (locals expand to multiple columns).
// Engine must match DriverCredentials.Engine (e.g. postgresql, sqlite, libsql, mysql).
func AlterTableAddFieldSQL(engine, tableName string, f *models.FieldInfo) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	if f.InputType == "geo" {
		return nil, errors.New("geo Field is not supported in SQL DDL here")
	}
	datatype, err := fieldSQLDataType(f)
	if err != nil {
		return nil, err
	}
	validations := fieldSQLValidations(f)
	valSuffix := strings.TrimSpace(strings.Join(validations, " "))
	if valSuffix != "" {
		valSuffix = " " + valSuffix
	}

	eng := strings.ToLower(strings.TrimSpace(engine))

	if f.Validation != nil && len(f.Validation.Locals) > 0 {
		var out []string
		for _, local := range f.Validation.Locals {
			var column string
			if local != "en" {
				column = fmt.Sprintf(`%s_%s`, f.Identifier, local)
			} else {
				column = f.Identifier
			}
			switch eng {
			case _const.PostgreSQLDriver:
				tq := QuotePGIdent(tableName)
				cq := QuotePGIdent(column)
				out = append(out, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s%s`, tq, cq, datatype, valSuffix))
			default:
				t := strings.ReplaceAll(tableName, "`", "``")
				out = append(out, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN IF NOT EXISTS %s %s%s", t, column, datatype, valSuffix))
			}
		}
		return out, nil
	}

	switch eng {
	case _const.PostgreSQLDriver:
		tq := QuotePGIdent(tableName)
		fq := QuotePGIdent(f.Identifier)
		return []string{fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s%s`, tq, fq, datatype, valSuffix)}, nil
	case _const.MySQLDriver, _const.MariaDBDriver:
		t := strings.ReplaceAll(tableName, "`", "``")
		cq := QuotePGIdent(f.Identifier) // reuse for MySQL/MariaDB
		return []string{fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN IF NOT EXISTS %s %s%s", t, cq, datatype, valSuffix)}, nil
	default:
		// SQLite / libsql / unknown: SQLite does NOT support UNIQUE in ALTER TABLE ADD COLUMN.
		// Strategy: add column without UNIQUE, then create a UNIQUE INDEX separately.
		t := strings.ReplaceAll(tableName, "`", "``")
		cleanSuffix := strings.ReplaceAll(valSuffix, "UNIQUE", "")
		cleanSuffix = strings.TrimSpace(cleanSuffix)
		
		var stmts []string
		// Add column without UNIQUE constraint
		stmt := fmt.Sprintf("ALTER TABLE `%s` ADD %s %s%s", t, f.Identifier, datatype, cleanSuffix)
		stmts = append(stmts, stmt)
		// If original valSuffix contained UNIQUE, add a separate UNIQUE index
		if strings.Contains(valSuffix, "UNIQUE") {
			idxName := fmt.Sprintf("idx_%s_%s", utility.PhysicalSQLTableName(tableName), f.Identifier)
			// SQLite identifier quoting: variables already contain quotes
			idxQ := QuotePGIdent(idxName)           // returns double-quoted identifier: "idx_name"
			tableQ := strings.ReplaceAll(tableName, "`", "``") // backtick-quoted: `table_name`
			colQ := strings.ReplaceAll(f.Identifier, "`", "``") // backtick-quoted: `col_name`
			// Note: tableQ and colQ already include backticks, so don't add extra in format string
			stmts = append(stmts, fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(%s)", idxQ, tableQ, colQ))
		}
		return stmts, nil
	}
}

func isDuplicateSQLColumnError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "duplicate column") {
		return true
	}
	if strings.Contains(s, "already exists") {
		return true
	}
	// PostgreSQL duplicate_column
	if strings.Contains(s, "42701") {
		return true
	}
	return false
}

// EnsureModelUserFieldColumns runs ALTER TABLE ADD for each top-level model.Fields entry so physical tables
// match schema (e.g. SaaS tenant catalogue name/logo after CreateModelTable created id-only table).
// Idempotent: duplicate column errors are ignored.
func (d *Driver) EnsureModelUserFieldColumns(ctx context.Context, model *models.ModelType) error {
	if model == nil || len(model.Fields) == 0 {
		return nil
	}
	tableName := utility.PhysicalSQLTableName(model.Name)
	for _, f := range model.Fields {
		if f == nil || strings.TrimSpace(f.Identifier) == "" {
			continue
		}
		if skipDDLSyntheticSystemRelationField(f, model) {
			continue
		}
		if f.InputType == "geo" {
			continue
		}
		stmts, err := AlterTableAddFieldSQL(d.DriverCredential.Engine, tableName, f)
		if err != nil {
			return err
		}
		for _, q := range stmts {
			if _, err := d.ORM.NewRaw(q).Exec(ctx); err != nil {
				if isDuplicateSQLColumnError(err) {
					continue
				}
				return fmt.Errorf("%w\n%s", err, q)
			}
		}
	}
	return nil
}
