package resolver

import (
	"context"
	"errors"
	stdsql "database/sql"
	"sync"
	"testing"

	_const "github.com/apito-io/engine/const"
	projectsql "gitlab.com/apito.io/open_driver/project/sqlite"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestApplySchemaDDLToBaseAndTenants_PropagatesTenantError(t *testing.T) {
	s := &GraphQLServer{
		Cfg: &models.Config{
			SchemaIterateHook: func(ctx context.Context, project *models.Project, fn func(context.Context, interface{}) error) error {
				return errors.New("tenant schema propagation partial failure: tenant t1: ddl failed")
			},
		},
	}
	drv := testSQLiteProjectDriver(t)
	err := s.applySchemaDDLToBaseAndTenants(context.Background(), &models.Project{ID: "p1"}, drv, func(d interfaces.ProjectDBInterface) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected tenant propagation error")
	}
}

func TestApplySchemaDDLToBaseAndTenants_BaseFailureSkipsTenants(t *testing.T) {
	tenantCalled := false
	s := &GraphQLServer{
		Cfg: &models.Config{
			SchemaIterateHook: func(ctx context.Context, project *models.Project, fn func(context.Context, interface{}) error) error {
				tenantCalled = true
				return nil
			},
		},
	}
	drv := testSQLiteProjectDriver(t)
	err := s.applySchemaDDLToBaseAndTenants(context.Background(), &models.Project{ID: "p1"}, drv, func(d interfaces.ProjectDBInterface) error {
		return errors.New("base failed")
	})
	if err == nil {
		t.Fatal("expected base error")
	}
	if tenantCalled {
		t.Fatal("tenants must not be called when base DDL fails")
	}
}

type trackingSystemDriver struct {
	updateCalled bool
	mu           sync.Mutex
}

func (t *trackingSystemDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	t.mu.Lock()
	t.updateCalled = true
	t.mu.Unlock()
	return nil
}

func (t *trackingSystemDriver) RunMigration(context.Context) error                      { return nil }
func (t *trackingSystemDriver) EnsureSystemBootstrap(context.Context) error             { return nil }
func (t *trackingSystemDriver) CreateProject(context.Context, string, *models.Project) (*models.Project, error) {
	return nil, nil
}
func (t *trackingSystemDriver) PersistProjectModelTypes(context.Context, string, []*models.ModelType) error {
	return nil
}
func (t *trackingSystemDriver) UpsertModelType(context.Context, string, *models.ModelType) error {
	return nil
}
func (t *trackingSystemDriver) DeleteModelType(context.Context, string, string) error { return nil }
func (t *trackingSystemDriver) TouchProjectUpdatedAt(context.Context, string) error   { return nil }
func (t *trackingSystemDriver) GetProjects(context.Context, []string) ([]*models.Project, error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetProject(context.Context, string) (*models.Project, error) {
	return nil, nil
}
func (t *trackingSystemDriver) CheckProjectName(context.Context, string) error { return nil }
func (t *trackingSystemDriver) SearchProjects(context.Context, *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	return nil, nil
}
func (t *trackingSystemDriver) FindUserProjects(context.Context, string) ([]*models.Project, error) {
	return nil, nil
}
func (t *trackingSystemDriver) FindUserProjectsWithRoles(context.Context, string) ([]*models.ProjectWithRoles, error) {
	return nil, nil
}
func (t *trackingSystemDriver) DeleteProjectFromSystem(context.Context, string) error { return nil }
func (t *trackingSystemDriver) SaveProjectAuthenticationSettings(context.Context, string, *models.AuthenticationSettings) error {
	return nil
}
func (t *trackingSystemDriver) SaveProjectStorageSettings(context.Context, string, *models.StorageSettings) error {
	return nil
}
func (t *trackingSystemDriver) CreateUser(context.Context, *models.User) (*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetUser(context.Context, string, string) (*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) UpdateUser(context.Context, *models.User) error { return nil }
func (t *trackingSystemDriver) DeleteUser(context.Context, string, string) error { return nil }
func (t *trackingSystemDriver) SearchProjectUsers(context.Context, string, string, int, int) ([]*models.User, int, error) {
	return nil, 0, nil
}
func (t *trackingSystemDriver) CountProjectUsersByRole(context.Context, string) (map[string]int, error) {
	return map[string]int{}, nil
}
func (t *trackingSystemDriver) GetUserByUsername(context.Context, string, string) (*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) ListUsersByEmail(context.Context, string, string) ([]*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) ListUsersByPhone(context.Context, string, string) ([]*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) ListUsersByGoogleSub(context.Context, string, string) ([]*models.User, error) {
	return nil, nil
}
func (t *trackingSystemDriver) AddATeamMemberToProject(context.Context, *models.TeamMemberAddRequest) error {
	return nil
}
func (t *trackingSystemDriver) RemoveATeamMemberFromProject(context.Context, string, string) error { return nil }
func (t *trackingSystemDriver) SearchFunctions(context.Context, *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetSystemUser(context.Context, string) (*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetSystemUserByEmail(context.Context, string) (*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetSystemUsers(context.Context, []string) ([]*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) CreateSystemUser(context.Context, *models.SystemUser) (*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) UpdateSystemUser(context.Context, *models.SystemUser, bool) error { return nil }
func (t *trackingSystemDriver) SearchSystemUsers(context.Context, *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	return nil, nil
}
func (t *trackingSystemDriver) CheckProjectWithRoles(context.Context, string, string) (*models.ProjectWithRoles, error) {
	return nil, nil
}
func (t *trackingSystemDriver) SearchResource(context.Context, *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetTeams(context.Context, string) ([]*models.Team, error) { return nil, nil }
func (t *trackingSystemDriver) GetOrganizations(context.Context, string) (*models.SearchResponse[models.Organization], error) {
	return nil, nil
}
func (t *trackingSystemDriver) FindOrganizationAdmin(context.Context, string) (*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) FindUserOrganizations(context.Context, string) ([]*models.Organization, error) {
	return nil, nil
}
func (t *trackingSystemDriver) CreateOrganization(context.Context, *models.Organization) (*models.Organization, error) {
	return nil, nil
}
func (t *trackingSystemDriver) AssignTeamToOrganization(context.Context, string, string, string) error {
	return nil
}
func (t *trackingSystemDriver) RemoveATeamFromOrganization(context.Context, string, string, string) error {
	return nil
}
func (t *trackingSystemDriver) AssignProjectToOrganization(context.Context, string, string, string) error {
	return nil
}
func (t *trackingSystemDriver) RemoveProjectFromOrganization(context.Context, string, string, string) error {
	return nil
}
func (t *trackingSystemDriver) GetProjectTeams(context.Context, string) (*models.Team, error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetTeamsMembers(context.Context, string) ([]*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) CreateTeam(context.Context, *models.Team) (*models.Team, error) {
	return nil, nil
}
func (t *trackingSystemDriver) AddTeamMetaInfo(context.Context, []*models.SystemUser) ([]*models.SystemUser, error) {
	return nil, nil
}
func (t *trackingSystemDriver) SaveAuditLog(context.Context, *models.AuditLogs) error { return nil }
func (t *trackingSystemDriver) SearchAuditLogs(context.Context, *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	return nil, nil
}
func (t *trackingSystemDriver) SearchWebHooks(context.Context, *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	return nil, nil
}
func (t *trackingSystemDriver) GetWebHook(context.Context, string, string) (*models.Webhook, error) {
	return nil, nil
}
func (t *trackingSystemDriver) DeleteWebhook(context.Context, string, string) error { return nil }
func (t *trackingSystemDriver) AddWebhookToProject(context.Context, *models.Webhook) (*models.Webhook, error) {
	return nil, nil
}
func (t *trackingSystemDriver) SaveRawData(context.Context, string, map[string]interface{}) error {
	return nil
}
func (t *trackingSystemDriver) CheckTokenBlacklisted(context.Context, string) error { return nil }
func (t *trackingSystemDriver) BlacklistAToken(context.Context, map[string]interface{}) error {
	return nil
}

func testSQLiteProjectDriverWithDSN(t *testing.T, dsn string) interfaces.ProjectDBInterface {
	t.Helper()
	sqldb, err := stdsql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	return &projectsql.Driver{
		Driver: sqlcommon.Driver{
			Base: sqlcommon.Base{
				ORM:              db,
				DriverCredential: &models.DriverCredentials{Engine: _const.SQLiteDriver},
			},
		},
	}
}

func TestAddFieldToModel_SecondDriverDoesNotDuplicateSchema(t *testing.T) {
	ctx := context.Background()
	drv1 := testSQLiteProjectDriverWithDSN(t, "file:tenant_a?mode=memory&cache=private")
	drv2 := testSQLiteProjectDriverWithDSN(t, "file:tenant_b?mode=memory&cache=private")

	model := &models.ModelType{
		Name: "posts",
		Fields: []*models.FieldInfo{
			{Identifier: "_id", FieldType: "text", InputType: "string", Serial: 1, SystemGenerated: true},
		},
	}
	project := &models.Project{ID: "p1", Schema: &models.ProjectSchema{}}

	if _, err := drv1.AddModel(ctx, project, model); err != nil {
		t.Fatal(err)
	}
	sqlDrv2, ok := drv2.(*projectsql.Driver)
	if !ok {
		t.Fatal("expected *sqlite.Driver")
	}
	if err := sqlDrv2.CreateModelTable(ctx, model, false); err != nil {
		t.Fatal(err)
	}

	title := &models.FieldInfo{
		Identifier: "title",
		Label:      "Title",
		FieldType:  "text",
		InputType:  "string",
	}
	param := &models.CommonSystemParams{Model: model, FieldInfo: title, ProjectID: "p1"}

	if _, err := drv1.AddFieldToModel(ctx, param, false, ""); err != nil {
		t.Fatal(err)
	}
	countAfterFirst := len(model.Fields)

	title2 := &models.FieldInfo{
		Identifier: "title",
		Label:      "Title",
		FieldType:  "text",
		InputType:  "string",
	}
	param2 := &models.CommonSystemParams{Model: model, FieldInfo: title2, ProjectID: "p1"}
	if _, err := drv2.AddFieldToModel(ctx, param2, false, ""); err != nil {
		t.Fatal(err)
	}

	if len(model.Fields) != countAfterFirst {
		t.Fatalf("schema field count = %d, want %d (tenant/base propagation must not duplicate in-memory fields)", len(model.Fields), countAfterFirst)
	}
}

func TestRunSchemaChange_SkipsSystemWhenBaseDDLFails(t *testing.T) {
	sys := &trackingSystemDriver{}
	s := &GraphQLServer{Cfg: &models.Config{}, SystemDriver: sys}
	project := &models.Project{ID: "p1", Schema: &models.ProjectSchema{}}
	drv := testSQLiteProjectDriver(t)
	err := s.runSchemaChange(context.Background(), schemasvc.RunInput{
		Ctx:           context.Background(),
		Project:       project,
		OperationType: models.SchemaOpTypeAddField,
		BaseDriver:    drv,
		ApplyDDL: func(interfaces.ProjectDBInterface) error {
			return errors.New("ddl failed")
		},
		PersistSystem: func() error {
			return sys.UpdateProject(context.Background(), project, true)
		},
		RefreshCache: func() error { return nil },
	})
	if err == nil {
		t.Fatal("expected ddl error")
	}
	sys.mu.Lock()
	called := sys.updateCalled
	sys.mu.Unlock()
	if called {
		t.Fatal("system schema must not persist when DDL fails")
	}
}
