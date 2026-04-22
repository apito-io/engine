package sql

import (
	"context"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// CreateModelTable creates a model table with only an id primary key column.
// Use ifNotExists to guard against duplicate table errors during provisioning.
func (S *SQLDriver) CreateModelTable(ctx context.Context, model *models.ModelType, ifNotExists bool) error {
	tableName := utility.SingularResourceName(model.Name)
	ifClause := " "
	if ifNotExists {
		ifClause = " IF NOT EXISTS "
	}
	var query string
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		t := strings.ReplaceAll(tableName, `"`, `""`)
		query = fmt.Sprintf("CREATE TABLE%s%s ( id VARCHAR(36) NOT NULL PRIMARY KEY );", ifClause, QuotePGIdent(t))
	default:
		t := strings.ReplaceAll(tableName, "`", "``")
		query = fmt.Sprintf("CREATE TABLE%s`%s`( id VARCHAR(36) NOT NULL PRIMARY KEY );", ifClause, t)
	}
	_, err := S.ORM.NewRaw(query).Exec(ctx)
	return err
}
