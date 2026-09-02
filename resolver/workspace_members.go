package resolver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

func workspaceMemberGrantObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "WorkspaceMemberGrant",
		Fields: graphql.Fields{
			"project_id":        &graphql.Field{Type: graphql.String},
			"project_name":      &graphql.Field{Type: graphql.String},
			"role":              &graphql.Field{Type: graphql.String},
			"permissions":       &graphql.Field{Type: graphql.NewList(graphql.String)},
			"invite_status":     &graphql.Field{Type: graphql.String},
			"invite_expires_at": &graphql.Field{Type: graphql.String},
		},
	})
}

func workspaceMemberObject(grantObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "WorkspaceMember",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.String},
			"email":      &graphql.Field{Type: graphql.String},
			"first_name": &graphql.Field{Type: graphql.String},
			"last_name":  &graphql.Field{Type: graphql.String},
			"avatar":     &graphql.Field{Type: graphql.String},
			"grants":     &graphql.Field{Type: graphql.NewList(grantObj)},
		},
	})
}

func (s *GraphQLServer) workspaceMemberQueryFields(memberObj *graphql.Object) graphql.Fields {
	return graphql.Fields{
		"workspaceMembers": &graphql.Field{
			Name:        "WorkspaceMembers",
			Description: "Console operators grouped by user across projects the caller can administer",
			Type:        graphql.NewList(memberObj),
			Resolve:     s.WorkspaceMembersResolverFn,
		},
	}
}

func (s *GraphQLServer) workspaceMemberMutationFields() graphql.Fields {
	return graphql.Fields{
		"inviteWorkspaceMember": &graphql.Field{
			Name:        "InviteWorkspaceMember",
			Description: "Invite a SystemUser onto selected projects with console-section permissions",
			Type:        graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"email": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"project_ids": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
				},
				"administrative_permissions": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				"make_admin": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: s.InviteWorkspaceMemberResolverFn,
		},
		"updateWorkspaceMember": &graphql.Field{
			Name:        "UpdateWorkspaceMember",
			Description: "Replace a member's grants on caller-administered projects",
			Type:        graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"project_ids": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
				},
				"administrative_permissions": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				"make_admin": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: s.UpdateWorkspaceMemberResolverFn,
		},
		"removeWorkspaceMember": &graphql.Field{
			Name:        "RemoveWorkspaceMember",
			Description: "Remove one project grant, or all caller-managed grants when project_id is omitted",
			Type:        graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"user_id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"project_id": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: s.RemoveWorkspaceMemberResolverFn,
		},
	}
}

func (s *GraphQLServer) WorkspaceMembersResolverFn(p graphql.ResolveParams) (interface{}, error) {
	router, param, err := s.workspaceCaller(p)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapMembersRead); err != nil {
		return nil, err
	}
	return s.listWorkspaceMembers(p.Context, param.UserID)
}

func (s *GraphQLServer) InviteWorkspaceMemberResolverFn(p graphql.ResolveParams) (interface{}, error) {
	router, param, err := s.workspaceCaller(p)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapMembersWrite); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(getArgString(p.Args, "email"))
	if email == "" {
		return nil, errors.New("email is required")
	}
	projectIDs := argStringSlice(p.Args["project_ids"])
	makeAdmin, _ := p.Args["make_admin"].(bool)
	perms := argStringSlice(p.Args["administrative_permissions"])
	user, created, err := s.resolveOrCreateSystemUserByEmail(p.Context, email, param.ProjectID)
	if err != nil {
		return nil, err
	}
	rawToken, err := s.applyWorkspaceMemberGrants(p.Context, param.UserID, user.ID, projectIDs, perms, makeAdmin, false, true)
	if err != nil {
		return nil, err
	}
	tempPass := ""
	if created {
		tempPass = user.TempPassword
	}
	s.sendTeamInviteEmail(user, s.projectDisplayNames(p.Context, projectIDs), tempPass, models.AcceptInviteURL(s.corsOrigin(), rawToken))
	return true, nil
}

func (s *GraphQLServer) UpdateWorkspaceMemberResolverFn(p graphql.ResolveParams) (interface{}, error) {
	router, param, err := s.workspaceCaller(p)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapMembersWrite); err != nil {
		return nil, err
	}
	memberID := strings.TrimSpace(getArgString(p.Args, "user_id"))
	if memberID == "" {
		return nil, errors.New("user_id is required")
	}
	projectIDs := argStringSlice(p.Args["project_ids"])
	makeAdmin, _ := p.Args["make_admin"].(bool)
	perms := argStringSlice(p.Args["administrative_permissions"])
	if _, err := s.applyWorkspaceMemberGrants(p.Context, param.UserID, memberID, projectIDs, perms, makeAdmin, true, false); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *GraphQLServer) RemoveWorkspaceMemberResolverFn(p graphql.ResolveParams) (interface{}, error) {
	router, param, err := s.workspaceCaller(p)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapMembersWrite); err != nil {
		return nil, err
	}
	memberID := strings.TrimSpace(getArgString(p.Args, "user_id"))
	if memberID == "" {
		return nil, errors.New("user_id is required")
	}
	if memberID == param.UserID {
		return nil, errors.New("cannot remove yourself from workspace members")
	}
	projectID := strings.TrimSpace(getArgString(p.Args, "project_id"))
	adminSet, err := s.callerAdministrableProjects(p.Context, param.UserID)
	if err != nil {
		return nil, err
	}
	if projectID != "" {
		if _, ok := adminSet[projectID]; !ok {
			return nil, errors.New("you cannot manage members on one or more selected projects")
		}
		if err := s.SystemDriver.RemoveATeamMemberFromProject(p.Context, projectID, memberID); err != nil {
			return nil, err
		}
		return true, nil
	}
	for pid := range adminSet {
		if err := s.SystemDriver.RemoveATeamMemberFromProject(p.Context, pid, memberID); err != nil {
			return nil, err
		}
	}
	return true, nil
}

func (s *GraphQLServer) workspaceCaller(p graphql.ResolveParams) (echo.Context, *models.CommonSystemParams, error) {
	v := p.Context.Value
	router, ok := v("router").(echo.Context)
	if !ok || router == nil {
		return nil, nil, errors.New("user has to be logged in for this action")
	}
	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(param.UserID) == "" {
		return nil, nil, errors.New("user has to be logged in for this action")
	}
	return router, param, nil
}

func (s *GraphQLServer) callerAdministrableProjects(ctx context.Context, userID string) (map[string]*models.Project, error) {
	rows, err := s.SystemDriver.FindUserProjectsWithRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := map[string]*models.Project{}
	for _, row := range rows {
		if row == nil || row.Project == nil {
			continue
		}
		if !services.IsAdministrableRole(row.Role) {
			continue
		}
		out[row.Project.ID] = row.Project
	}
	return out, nil
}

func (s *GraphQLServer) applyWorkspaceMemberGrants(ctx context.Context, callerID, memberID string, projectIDs, perms []string, makeAdmin, replace, asInvite bool) (string, error) {
	if memberID == "" {
		return "", errors.New("member is required")
	}
	if memberID == callerID {
		return "", errors.New("cannot edit your own workspace grants here")
	}
	adminSet, err := s.callerAdministrableProjects(ctx, callerID)
	if err != nil {
		return "", err
	}
	wanted := uniqueNonEmpty(projectIDs)
	if !replace && len(wanted) == 0 {
		return "", errors.New("select at least one project")
	}
	for _, pid := range wanted {
		if _, ok := adminSet[pid]; !ok {
			return "", errors.New("you cannot manage members on one or more selected projects")
		}
	}
	role := models.NormalizeMembershipRole("", makeAdmin)
	permissions := models.MembershipPermissions(perms, makeAdmin)
	if !makeAdmin && len(permissions) == 0 {
		return "", errors.New("select at least one administrative permission")
	}
	var rawToken, tokenHash string
	now := time.Now().UTC()
	if asInvite {
		rawToken, tokenHash, err = models.NewInviteToken()
		if err != nil {
			return "", err
		}
	}
	wantedSet := map[string]struct{}{}
	for _, pid := range wanted {
		wantedSet[pid] = struct{}{}
		existing, err := s.lookupUserProjectGrant(ctx, memberID, pid)
		if err != nil {
			return "", err
		}
		if existing != nil && services.IsAdministrableRole(existing.Role) && existing.Role != models.MembershipRoleAdmin {
			// Do not downgrade an owner row via invite/edit.
			continue
		}
		req := &models.TeamMemberAddRequest{
			ProjectID:   pid,
			UserID:      memberID,
			Role:        role,
			Permissions: permissions,
		}
		if asInvite {
			models.StampInviteOnRequest(req, existing, tokenHash, now, s.inviteTTL())
		}
		if err := s.SystemDriver.AddATeamMemberToProject(ctx, req); err != nil {
			return "", err
		}
	}
	if !replace {
		return rawToken, nil
	}
	for pid := range adminSet {
		if _, keep := wantedSet[pid]; keep {
			continue
		}
		existing, err := s.lookupUserProjectGrant(ctx, memberID, pid)
		if err != nil || existing == nil {
			continue
		}
		if services.IsAdministrableRole(existing.Role) && existing.Role != models.MembershipRoleAdmin {
			continue
		}
		if err := s.SystemDriver.RemoveATeamMemberFromProject(ctx, pid, memberID); err != nil {
			return "", err
		}
	}
	return rawToken, nil
}

type userProjectGrantStore interface {
	GetUserProjectGrant(ctx context.Context, userID, projectID string) (*models.UserProject, error)
}

func (s *GraphQLServer) lookupUserProjectGrant(ctx context.Context, userID, projectID string) (*models.UserProject, error) {
	if s == nil || s.SystemDriver == nil {
		return nil, nil
	}
	store, ok := s.SystemDriver.(userProjectGrantStore)
	if !ok {
		return nil, nil
	}
	return store.GetUserProjectGrant(ctx, userID, projectID)
}

func (s *GraphQLServer) listWorkspaceMembers(ctx context.Context, callerID string) ([]*models.WorkspaceMember, error) {
	adminSet, err := s.callerAdministrableProjects(ctx, callerID)
	if err != nil {
		return nil, err
	}
	byUser := map[string]*models.WorkspaceMember{}
	var order []string
	for pid, proj := range adminSet {
		members, err := s.SystemDriver.GetTeamsMembers(ctx, pid)
		if err != nil {
			return nil, err
		}
		for _, u := range members {
			if u == nil || u.ID == "" {
				continue
			}
			m, ok := byUser[u.ID]
			if !ok {
				m = &models.WorkspaceMember{
					ID:        u.ID,
					Email:     u.Email,
					FirstName: u.FirstName,
					LastName:  u.LastName,
					Avatar:    u.Avatar,
				}
				byUser[u.ID] = m
				order = append(order, u.ID)
			}
			name := ""
			if proj != nil {
				name = proj.Name
			}
			m.Grants = append(m.Grants, &models.WorkspaceMemberGrant{
				ProjectID:       pid,
				ProjectName:     name,
				Role:            u.ProjectAssignedRole,
				Permissions:     append([]string(nil), u.ProjectAccessPermissions...),
				InviteStatus:    u.InviteStatus,
				InviteExpiresAt: u.InviteExpiresAt,
			})
		}
	}
	out := make([]*models.WorkspaceMember, 0, len(order))
	for _, id := range order {
		out = append(out, byUser[id])
	}
	return out, nil
}

func (s *GraphQLServer) resolveOrCreateSystemUserByEmail(ctx context.Context, email, currentProjectID string) (*models.SystemUser, bool, error) {
	user, err := s.SystemDriver.GetSystemUserByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
		user = nil
	}
	if user != nil {
		return user, false, nil
	}
	tempPass := utility.RandomStringGenerator(10)
	registerRequest := &models.RegisterRequest{
		User: &models.SystemUser{
			Email:            email,
			RegisterProvider: "system",
			TempPassword:     tempPass,
			CurrentProjectID: currentProjectID,
		},
	}
	user, err = s.AuthService.Signup(ctx, registerRequest)
	if err != nil {
		return nil, false, err
	}
	user, err = s.SystemDriver.CreateSystemUser(ctx, user)
	if err != nil {
		return nil, false, err
	}
	return user, true, nil
}

func (s *GraphQLServer) corsOrigin() string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	return s.Cfg.CORSOrigin
}

func (s *GraphQLServer) inviteTTL() time.Duration {
	if s == nil || s.Cfg == nil {
		return 0
	}
	return s.Cfg.InviteExpireDuration()
}

func (s *GraphQLServer) projectDisplayNames(ctx context.Context, projectIDs []string) []string {
	if s == nil || s.SystemDriver == nil {
		return nil
	}
	var names []string
	for _, id := range projectIDs {
		proj, err := s.SystemDriver.GetProject(ctx, id)
		if err != nil || proj == nil {
			continue
		}
		name := strings.TrimSpace(proj.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (s *GraphQLServer) sendTeamInviteEmail(user *models.SystemUser, projectNames []string, tempPassword, acceptURL string) {
	if s == nil || s.Mailer == nil || user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	go func(_user *models.SystemUser, names []string, pass, link string) {
		ctx := context.Background()
		req := &models.EmailSendRequest{
			AppURL:       s.corsOrigin(),
			AcceptURL:    link,
			Recipients:   []string{_user.Email},
			TempPassword: pass,
			ProjectNames: names,
		}
		services.ComposeTeamInvite(req)
		if err := s.Mailer.Send(ctx, req); err != nil {
			log.Printf("team invite email: send failed to %s: %v", _user.Email, err)
			return
		}
		log.Printf("team invite email: sent to %s", _user.Email)
		if s.Cfg != nil && strings.EqualFold(s.Cfg.Environment, "local") && strings.TrimSpace(link) != "" {
			log.Printf("team invite email: accept %s", link)
		}
	}(user, projectNames, tempPassword, acceptURL)
}

func argStringSlice(v interface{}) []string {
	switch vals := v.(type) {
	case []interface{}:
		var out []string
		for _, item := range vals {
			s, _ := item.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return uniqueNonEmpty(vals)
	default:
		return nil
	}
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func requireWorkspaceMemberAdmin(pwr *models.ProjectWithRoles) error {
	if pwr == nil || !services.IsAdministrableRole(pwr.Role) {
		return fmt.Errorf("admin role required")
	}
	return nil
}
