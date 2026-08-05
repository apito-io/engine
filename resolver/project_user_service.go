package resolver

import (
	"context"
	"errors"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

type projectAuthUserStore interface {
	EnsureUsersTable(ctx context.Context) error
	CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error)
	GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error)
	GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error)
	ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error)
	ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error)
	ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error)
	ListProjectAuthUsersByOAuthSub(ctx context.Context, tenantID, provider, oauthSub string) ([]*models.ProjectAuthUser, error)
	SearchProjectAuthUsers(ctx context.Context, tenantID, q string, limit, offset int) ([]*models.ProjectAuthUser, int, error)
	CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error)
	UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error
	DeleteProjectAuthUser(ctx context.Context, userID string) error
}

type ProjectUserService struct {
	server    *GraphQLServer
	cache     *models.ApplicationCache
	ctx       context.Context
	projectID string
	store     projectAuthUserStore
	sys       interfaces.ApitoSystemDB
}

func (s *GraphQLServer) ProjectUserService(cache *models.ApplicationCache, ctx context.Context) (*ProjectUserService, error) {
	if s == nil || cache == nil || cache.Project == nil {
		return nil, errors.New("project cache required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	svc := &ProjectUserService{
		server:    s,
		cache:     cache,
		ctx:       ctx,
		projectID: cache.Project.ID,
		sys:       s.systemDB(),
	}
	if s.GraphQLExecutor == nil {
		return svc, nil
	}
	dbCtx := PublicProjectDBContext(cache, ctx)
	drv, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return svc, nil
	}
	store, ok := drv.(projectAuthUserStore)
	if ok {
		svc.store = store
	}
	return svc, nil
}

func tenantIDFromCacheCtx(cache *models.ApplicationCache) string {
	if cache == nil || cache.Ctx == nil {
		return ""
	}
	if v := cache.Ctx.Value("tenant_id"); v != nil {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (svc *ProjectUserService) writeThroughUser(u *models.User, tenantID string) {
	if svc == nil || svc.store == nil || u == nil {
		return
	}
	row := models.ProjectAuthUserFromUser(u, tenantID)
	if row == nil {
		return
	}
	if tid := strings.TrimSpace(tenantID); tid != "" {
		row.TenantID = tid
	} else if tid := strings.TrimSpace(u.ProjectID); tid != "" {
		_ = tid
	}
	if existing, _ := svc.store.GetProjectAuthUser(svc.ctx, u.ID); existing != nil {
		_ = svc.store.UpdateProjectAuthUser(svc.ctx, row)
		return
	}
	_, _ = svc.store.CreateProjectAuthUser(svc.ctx, row)
}

func (svc *ProjectUserService) userFromProjectRow(row *models.ProjectAuthUser) *models.User {
	if row == nil {
		return nil
	}
	u := models.UserFromProjectAuthUser(svc.projectID, row)
	if u != nil && strings.TrimSpace(row.TenantID) == "" {
		if tid := tenantIDFromCacheCtx(svc.cache); tid != "" {
			// per-tenant DB: expose tenant_id from request context in GraphQL via pro hook
			_ = tid
		}
	}
	return u
}

func (svc *ProjectUserService) getUserWithFallback(userID string, tenantID string) (*models.User, error) {
	if svc.store != nil {
		row, err := svc.store.GetProjectAuthUser(svc.ctx, userID)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, err
		}
		if row != nil {
			return svc.userFromProjectRow(row), nil
		}
	}
	if svc.sys == nil {
		return nil, nil
	}
	u, err := svc.sys.GetUser(svc.ctx, svc.projectID, userID)
	if err != nil {
		return nil, err
	}
	if u != nil {
		svc.writeThroughUser(u, tenantID)
	}
	return u, nil
}

func (svc *ProjectUserService) getUserByUsernameWithFallback(username, tenantID string) (*models.User, error) {
	if svc.store != nil {
		row, err := svc.store.GetProjectAuthUserByUsername(svc.ctx, username)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, err
		}
		if row != nil {
			return svc.userFromProjectRow(row), nil
		}
	}
	if svc.sys == nil {
		return nil, nil
	}
	u, err := svc.sys.GetUserByUsername(svc.ctx, svc.projectID, username)
	if err != nil {
		return nil, err
	}
	if u != nil {
		svc.writeThroughUser(u, tenantID)
	}
	return u, nil
}

func (svc *ProjectUserService) listByEmailWithFallback(tenantID, email string) ([]*models.User, error) {
	if svc.store != nil {
		rows, err := svc.store.ListProjectAuthUsersByEmail(svc.ctx, tenantID, email)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, err
		}
		if len(rows) > 0 {
			return projectAuthRowsToUsers(svc, rows), nil
		}
	}
	if svc.sys == nil {
		return nil, nil
	}
	rows, err := svc.sys.ListUsersByEmail(svc.ctx, svc.projectID, email)
	if err != nil {
		return nil, err
	}
	for _, u := range rows {
		svc.writeThroughUser(u, tenantID)
	}
	return rows, nil
}

func (svc *ProjectUserService) listByPhoneWithFallback(tenantID, phone string) ([]*models.User, error) {
	if svc.store != nil {
		rows, err := svc.store.ListProjectAuthUsersByPhone(svc.ctx, tenantID, phone)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, err
		}
		if len(rows) > 0 {
			return projectAuthRowsToUsers(svc, rows), nil
		}
	}
	if svc.sys == nil {
		return nil, nil
	}
	rows, err := svc.sys.ListUsersByPhone(svc.ctx, svc.projectID, phone)
	if err != nil {
		return nil, err
	}
	for _, u := range rows {
		svc.writeThroughUser(u, tenantID)
	}
	return rows, nil
}

func (svc *ProjectUserService) listByGoogleSubWithFallback(tenantID, googleSub string) ([]*models.User, error) {
	if svc.store != nil {
		rows, err := svc.store.ListProjectAuthUsersByGoogleSub(svc.ctx, tenantID, googleSub)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, err
		}
		if len(rows) > 0 {
			return projectAuthRowsToUsers(svc, rows), nil
		}
	}
	if svc.sys == nil {
		return nil, nil
	}
	rows, err := svc.sys.ListUsersByGoogleSub(svc.ctx, svc.projectID, googleSub)
	if err != nil {
		return nil, err
	}
	for _, u := range rows {
		svc.writeThroughUser(u, tenantID)
	}
	return rows, nil
}

func (svc *ProjectUserService) listByOAuthSubWithFallback(tenantID, provider, oauthSub string) ([]*models.User, error) {
	prov := strings.ToLower(strings.TrimSpace(provider))
	sub := strings.TrimSpace(oauthSub)
	if prov == "" || sub == "" {
		return nil, nil
	}
	if svc.store == nil {
		return nil, nil
	}
	rows, err := svc.store.ListProjectAuthUsersByOAuthSub(svc.ctx, tenantID, prov, sub)
	if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
		return nil, err
	}
	return projectAuthRowsToUsers(svc, rows), nil
}

func (svc *ProjectUserService) searchWithFallback(tenantID, q string, limit, offset int) ([]*models.User, int, error) {
	if svc.store != nil {
		rows, count, err := svc.store.SearchProjectAuthUsers(svc.ctx, tenantID, q, limit, offset)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return nil, 0, err
		}
		if count > 0 || len(rows) > 0 {
			return projectAuthRowsToUsers(svc, rows), count, nil
		}
	}
	if svc.sys == nil {
		return nil, 0, nil
	}
	rows, count, err := svc.sys.SearchProjectUsers(svc.ctx, svc.projectID, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for _, u := range rows {
		svc.writeThroughUser(u, tenantID)
	}
	return rows, count, nil
}

func (svc *ProjectUserService) countByRoleWithFallback(tenantID string) (map[string]int, error) {
	if svc.store != nil {
		counts, err := svc.store.CountProjectAuthUsersByRole(svc.ctx, tenantID)
		if err == nil && len(counts) > 0 {
			return counts, nil
		}
		// Fall back to system project_users when store is unsupported or tenant DB fails.
	}
	if svc.sys == nil {
		return map[string]int{}, nil
	}
	return svc.sys.CountProjectUsersByRole(svc.ctx, svc.projectID)
}

// CountUsersByRole returns app end-user counts grouped by role for the current project.
func (svc *ProjectUserService) CountUsersByRole() (map[string]int, error) {
	if svc == nil {
		return nil, errors.New("project user service required")
	}
	return svc.countByRoleWithFallback("")
}

func projectAuthRowsToUsers(svc *ProjectUserService, rows []*models.ProjectAuthUser) []*models.User {
	out := make([]*models.User, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, svc.userFromProjectRow(row))
		}
	}
	return out
}

func (svc *ProjectUserService) createUser(user *models.User, tenantID string) (*models.User, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	user.ProjectID = svc.projectID
	row := models.ProjectAuthUserFromUser(user, tenantID)
	if svc.store != nil {
		created, err := svc.store.CreateProjectAuthUser(svc.ctx, row)
		if err != nil {
			if !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
				return nil, err
			}
		} else if created != nil {
			return svc.userFromProjectRow(created), nil
		}
	}
	if svc.sys == nil {
		return nil, errors.New("user store not available")
	}
	return svc.sys.CreateUser(svc.ctx, user)
}

func (svc *ProjectUserService) updateUser(user *models.User, tenantID string) error {
	if user == nil {
		return errors.New("user is required")
	}
	user.ProjectID = svc.projectID
	row := models.ProjectAuthUserFromUser(user, tenantID)
	if svc.store != nil {
		existing, err := svc.store.GetProjectAuthUser(svc.ctx, user.ID)
		if err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return err
		}
		if existing != nil {
			if strings.TrimSpace(row.TenantID) == "" {
				row.TenantID = existing.TenantID
			}
			if row.Secret == "" {
				row.Secret = existing.Secret
			}
			if strings.TrimSpace(row.Username) == "" {
				row.Username = existing.Username
			}
			if row.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
		}
		if err := svc.store.UpdateProjectAuthUser(svc.ctx, row); err != nil {
			if !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
				return err
			}
		} else {
			return nil
		}
	}
	if svc.sys == nil {
		return errors.New("user store not available")
	}
	return svc.sys.UpdateUser(svc.ctx, user)
}

func (svc *ProjectUserService) deleteUser(userID string) error {
	if svc.store != nil {
		if err := svc.store.DeleteProjectAuthUser(svc.ctx, userID); err != nil && !errors.Is(err, ae.ErrProjectAuthUsersUnsupported) {
			return err
		}
	}
	if svc.sys == nil {
		return nil
	}
	return svc.sys.DeleteUser(svc.ctx, svc.projectID, userID)
}

func (svc *ProjectUserService) SearchWithFallback(tenantID, q string, limit, offset int) ([]*models.User, int, error) {
	return svc.searchWithFallback(tenantID, q, limit, offset)
}

func (svc *ProjectUserService) ListByEmailWithFallback(tenantID, email string) ([]*models.User, error) {
	return svc.listByEmailWithFallback(tenantID, email)
}

func (svc *ProjectUserService) ListByPhoneWithFallback(tenantID, phone string) ([]*models.User, error) {
	return svc.listByPhoneWithFallback(tenantID, phone)
}

func (svc *ProjectUserService) ListByGoogleSubWithFallback(tenantID, googleSub string) ([]*models.User, error) {
	return svc.listByGoogleSubWithFallback(tenantID, googleSub)
}

func (svc *ProjectUserService) ListByOAuthSubWithFallback(tenantID, provider, oauthSub string) ([]*models.User, error) {
	return svc.listByOAuthSubWithFallback(tenantID, provider, oauthSub)
}

func (svc *ProjectUserService) GetUserWithFallback(userID, tenantID string) (*models.User, error) {
	return svc.getUserWithFallback(userID, tenantID)
}

// GetProjectAuthUserRow loads the raw project DB users row (includes tenant_id for SaaS shared DB).
func (svc *ProjectUserService) GetProjectAuthUserRow(userID string) (*models.ProjectAuthUser, error) {
	if svc == nil || svc.store == nil {
		return nil, nil
	}
	return svc.store.GetProjectAuthUser(svc.ctx, userID)
}

func (svc *ProjectUserService) GetUserByUsernameWithFallback(username, tenantID string) (*models.User, error) {
	return svc.getUserByUsernameWithFallback(username, tenantID)
}

func (svc *ProjectUserService) CreateUserRecord(user *models.User, tenantID string) (*models.User, error) {
	return svc.createUser(user, tenantID)
}

func (svc *ProjectUserService) UpdateUserRecord(user *models.User, tenantID string) error {
	return svc.updateUser(user, tenantID)
}

func (svc *ProjectUserService) DeleteUserRecord(userID string) error {
	return svc.deleteUser(userID)
}

func (svc *ProjectUserService) ResolveCreateUsername(uid, usernameArg, tenantID string) (string, error) {
	return svc.resolveCreateUsername(uid, usernameArg, tenantID)
}

func (svc *ProjectUserService) WriteThroughUser(u *models.User, tenantID string) {
	svc.writeThroughUser(u, tenantID)
}

func (svc *ProjectUserService) UserFromProjectRow(row *models.ProjectAuthUser) *models.User {
	return svc.userFromProjectRow(row)
}

func (svc *ProjectUserService) resolveCreateUsername(uid, usernameArg, tenantID string) (string, error) {
	uname := NormalizeUserUsernameArg(usernameArg)
	if uname == "" {
		return internalUserUsername(uid), nil
	}
	ex, err := svc.getUserByUsernameWithFallback(uname, tenantID)
	if err != nil {
		return "", err
	}
	if ex != nil {
		return "", errors.New("username already exists")
	}
	return uname, nil
}
