package sql

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	_const "github.com/apito-io/databasedriver"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/denisenkom/go-mssqldb"

	"github.com/apito-io/types/protobuff"
)

type PostgreSQLDriver struct {
	ORM              *bun.DB
	DriverCredential *models.DriverCredentials
}

func GetSystemSQLDriver(driverCredentials *models.DriverCredentials) (*PostgreSQLDriver, error) {

	var orm *bun.DB

	switch driverCredentials.Engine {
	case _const.MySQLDriver, _const.MariaDBDriver:
		sqldb, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
			driverCredentials.User, driverCredentials.Password,
			driverCredentials.Host, driverCredentials.Port,
			driverCredentials.Database,
		))
		if err != nil {
			return nil, err
		}
		orm = bun.NewDB(sqldb, mysqldialect.New())
	case _const.SQLServerDriver:
		sqldb, err := sql.Open("sqlserver", fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			driverCredentials.User, driverCredentials.Password,
			driverCredentials.Host, driverCredentials.Port,
			driverCredentials.Database,
		))
		if err != nil {
			return nil, err
		}
		orm = bun.NewDB(sqldb, mssqldialect.New())
	case _const.SQLiteDriver:

		// Replace with your SQLite database file path
		dbPath := fmt.Sprintf("./%s.sqlite", driverCredentials.Database)

		// Check if the database file exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			// Create the database file
			file, err := os.Create(dbPath)
			if err != nil {
				return nil, err
			}
			file.Close()
			fmt.Printf("Database %s created\n", dbPath)
		}

		// Create a new database connection
		//sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
		sqldb, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, err
		}
		//defer sqldb.Close()
		orm = bun.NewDB(sqldb, sqlitedialect.New())
	case _const.PostgresSQLDriver:
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			driverCredentials.User, driverCredentials.Password,
			driverCredentials.Host, driverCredentials.Port,
			driverCredentials.Database,
		)
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		orm = bun.NewDB(sqldb, pgdialect.New())
		if err := orm.Ping(); err != nil {
			return nil, err
		}
	default:
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			driverCredentials.User, driverCredentials.Password,
			driverCredentials.Host, driverCredentials.Port,
			driverCredentials.Database,
		)
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		orm = bun.NewDB(sqldb, pgdialect.New())
		if err := orm.Ping(); err != nil {
			return nil, err
		}
	}

	return &PostgreSQLDriver{ORM: orm, DriverCredential: driverCredentials}, nil
}

func (p *PostgreSQLDriver) Ping() error {
	ctx := context.Background()
	
	switch p.ORM.Dialect().Name() {
	case dialect.SQLite:
		// SQLite: Test with a simple query since it's file-based
		var result int
		err := p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
		return err
		
	case dialect.PG:
		// PostgreSQL: Test connection with a simple query
		var result int
		err := p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
		return err
		
	case dialect.MySQL:
		// MySQL/MariaDB: Test connection with a simple query
		var result int
		err := p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
		return err
		
	case dialect.MSSQL:
		// SQL Server: Test connection with a simple query
		var result int
		err := p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
		return err
		
	default:
		return fmt.Errorf("database dialect not supported: %s", p.ORM.Dialect().Name())
	}
}

func (p *PostgreSQLDriver) RunMigration(ctx context.Context) error {

	models := []interface{}{
		(*models.SystemUser)(nil),
		(*models.Project)(nil),
		(*models.ProjectSchema)(nil),
		(*protobuff.PluginDetails)(nil),
		(*models.APIToken)(nil),
		(*models.DriverCredentials)(nil),
		(*models.SystemMessage)(nil),
		(*models.ModelType)(nil),
		(*models.ApitoFunction)(nil),
	}

	for _, model := range models {
		_, err := p.ORM.NewCreateTable().IfNotExists().Model(model).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetTeams retrieves teams for a given user using SQL
func (p *PostgreSQLDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	var teams []*models.Team

	err := p.ORM.NewSelect().
		Model(&teams).
		Join("JOIN user_teams ut ON ut.team_id = team.id").
		Where("ut.user_id = ?", userId).
		Scan(ctx)

	return teams, err
}

// GetTeamsMembers retrieves team members for a project using SQL
func (p *PostgreSQLDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := p.ORM.NewSelect().
		Model(&users).
		Join("JOIN project_teams pt ON pt.user_id = system_user.id").
		Where("pt.project_id = ?", projectId).
		Scan(ctx)

	return users, err
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions using SQL
func (p *PostgreSQLDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	var projectWithRoles []*models.ProjectWithRoles

	err := p.ORM.NewSelect().
		Model(&projectWithRoles).
		Join("JOIN user_projects up ON up.project_id = project.id").
		Where("up.user_id = ?", userId).
		Scan(ctx)

	return projectWithRoles, err
}

// SearchResource searches for resources based on common system parameters using SQL
func (p *PostgreSQLDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// This is a generic search function - implementation depends on specific resource type
	// For now, return empty results
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization using SQL
func (p *PostgreSQLDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := p.ORM.NewSelect().
		Model(user).
		Join("JOIN user_organizations uo ON uo.user_id = system_user.id").
		Where("uo.organization_id = ? AND uo.role = ?", orgId, "admin").
		Limit(1).
		Scan(ctx)

	return user, err
}

// SaveAuditLog saves an audit log entry using SQL
func (p *PostgreSQLDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	_, err := p.ORM.NewInsert().
		Model(auditLog).
		Exec(ctx)

	return err
}

// SearchAuditLogs searches for audit logs based on common system parameters using SQL
func (p *PostgreSQLDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	var logs []*models.AuditLogs

	query := p.ORM.NewSelect().Model(&logs)

	if param.UserID != "" {
		query = query.Where("user_id = ?", param.UserID)
	}
	if param.ProjectID != "" {
		query = query.Where("project_id = ?", param.ProjectID)
	}

	err := query.Order("created_at DESC").Limit(100).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations retrieves organizations for a given user using SQL
func (p *PostgreSQLDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	var organizations []*models.Organization

	err := p.ORM.NewSelect().
		Model(&organizations).
		Join("JOIN user_organizations uo ON uo.organization_id = organization.id").
		Where("uo.user_id = ?", userId).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}
