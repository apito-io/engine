package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
)

type oauthMemStore struct {
	bySub   map[string]*models.ProjectAuthUser
	byEmail map[string]*models.ProjectAuthUser
}

func (s *oauthMemStore) EnsureUsersTable(context.Context) error { return nil }
func (s *oauthMemStore) CreateProjectAuthUser(_ context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	if s.bySub == nil {
		s.bySub = map[string]*models.ProjectAuthUser{}
	}
	if s.byEmail == nil {
		s.byEmail = map[string]*models.ProjectAuthUser{}
	}
	key := user.Provider + ":" + user.OAuthSub
	s.bySub[key] = user
	if user.Email != "" {
		s.byEmail[user.Email] = user
	}
	return user, nil
}
func (s *oauthMemStore) GetProjectAuthUser(context.Context, string) (*models.ProjectAuthUser, error) {
	return nil, nil
}
func (s *oauthMemStore) GetProjectAuthUserByUsername(context.Context, string) (*models.ProjectAuthUser, error) {
	return nil, nil
}
func (s *oauthMemStore) ListProjectAuthUsersByEmail(_ context.Context, _, email string) ([]*models.ProjectAuthUser, error) {
	if u, ok := s.byEmail[email]; ok {
		return []*models.ProjectAuthUser{u}, nil
	}
	return nil, nil
}
func (s *oauthMemStore) ListProjectAuthUsersByPhone(context.Context, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (s *oauthMemStore) ListProjectAuthUsersByGoogleSub(context.Context, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (s *oauthMemStore) ListProjectAuthUsersByOAuthSub(_ context.Context, _, provider, oauthSub string) ([]*models.ProjectAuthUser, error) {
	if u, ok := s.bySub[provider+":"+oauthSub]; ok {
		return []*models.ProjectAuthUser{u}, nil
	}
	return nil, nil
}
func (s *oauthMemStore) SearchProjectAuthUsers(context.Context, string, string, int, int) ([]*models.ProjectAuthUser, int, error) {
	return nil, 0, nil
}
func (s *oauthMemStore) CountProjectAuthUsersByRole(context.Context, string) (map[string]int, error) {
	return nil, nil
}
func (s *oauthMemStore) UpdateProjectAuthUser(_ context.Context, user *models.ProjectAuthUser) error {
	if s.bySub == nil {
		s.bySub = map[string]*models.ProjectAuthUser{}
	}
	s.bySub[user.Provider+":"+user.OAuthSub] = user
	if user.Email != "" {
		if s.byEmail == nil {
			s.byEmail = map[string]*models.ProjectAuthUser{}
		}
		s.byEmail[user.Email] = user
	}
	return nil
}
func (s *oauthMemStore) DeleteProjectAuthUser(context.Context, string) error { return nil }

func TestResolveUserForOAuthLoginBySub(t *testing.T) {
	store := &oauthMemStore{
		bySub: map[string]*models.ProjectAuthUser{
			"github:99": {
				ID: "u1", Provider: "github", OAuthSub: "99",
				Email: "a@example.com", Status: models.UserStatusActive, Role: "public",
			},
		},
	}
	svc := &ProjectUserService{
		ctx:       context.Background(),
		projectID: "p1",
		store:     store,
	}
	user, err := svc.ResolveUserForOAuthLogin(models.OAuthProviderGithub, "99", "a@example.com", true, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || user.ID != "u1" {
		t.Fatalf("unexpected user %#v", user)
	}
}

func TestResolveUserForOAuthLoginCreates(t *testing.T) {
	store := &oauthMemStore{}
	svc := &ProjectUserService{
		ctx:       context.Background(),
		projectID: "p1",
		store:     store,
	}
	user, err := svc.ResolveUserForOAuthLogin(models.OAuthProviderFacebook, "fb1", "b@example.com", true, "", "", func() (*models.User, error) {
		return &models.User{
			ID: "new", Email: "b@example.com", Provider: models.UserProviderFacebook,
			OAuthSub: "fb1", Status: models.UserStatusActive, Role: "public",
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "new" {
		t.Fatalf("expected created user, got %#v", user)
	}
}
