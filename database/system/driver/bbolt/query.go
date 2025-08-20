package bbolt

import (
	"context"
	"errors"
	apitobolt "github.com/apito-io/apitoBolt"
	q "github.com/apito-io/apitoBolt/q"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
)

// User-related functions

func (d *ProBBoltSystemDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	user.XKey = uuid.New().String()
	user.ID = user.XKey
	user.CreatedAt = utility.GetCurrentTime()
	user.UpdatedAt = utility.GetCurrentTime()

	collection := d.DB.Collection("users")
	_, err := collection.Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (d *ProBBoltSystemDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	collection := d.DB.Collection("users")
	user.UpdatedAt = utility.GetCurrentTime()

	if replace {
		return collection.Update(user)
	} else {
		// For partial update, read existing and merge
		var existing models.SystemUser
		err := collection.FindByID(user.ID, &existing)
		if err != nil {
			return err
		}

		// Merge non-zero values
		if user.FirstName != "" {
			existing.FirstName = user.FirstName
		}
		if user.LastName != "" {
			existing.LastName = user.LastName
		}
		if user.Email != "" {
			existing.Email = user.Email
		}
		if user.Avatar != "" {
			existing.Avatar = user.Avatar
		}
		existing.UpdatedAt = user.UpdatedAt

		return collection.Update(&existing)
	}
}

func (d *ProBBoltSystemDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	// Call internal pro implementation
	proUser, err := d.getSystemUser(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert back to open-core SystemUser for interface compliance
	return proUser, nil
}

// Internal pro implementation
func (d *ProBBoltSystemDriver) getSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	var user models.SystemUser
	collection := d.DB.Collection("users")

	err := collection.FindByID(id, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (d *ProBBoltSystemDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	// Call internal pro implementation
	proUser, err := d.getSystemUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Convert back to open-core SystemUser for interface compliance
	return proUser, nil
}

// Internal pro implementation
func (d *ProBBoltSystemDriver) getSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	var users []models.SystemUser
	collection := d.DB.Collection("users")

	err := collection.Find("email", email, &users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, errors.New("user not found")
	}

	return &users[0], nil
}

func (d *ProBBoltSystemDriver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	collection := d.DB.Collection("users")
	var users []*models.SystemUser

	for _, key := range keys {
		var user models.SystemUser
		if err := collection.FindByID(key, &user); err == nil {
			users = append(users, &user)
		}
	}

	return users, nil
}

func (d *ProBBoltSystemDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	collection := d.DB.Collection("users")
	var users []models.SystemUser

	// Build query based on parameters - simplified for BBolt implementation
	// In production, you'd add Search, Limit, and Offset to CommonSystemParams

	// Get all users with default limit
	query := collection.Select().Limit(100)

	err := query.Find(&users)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.SystemUser
	for i := range users {
		results = append(results, &users[i])
	}

	return &models.SearchResponse[models.SystemUser]{
		Results: results,
	}, nil
}

func (d *ProBBoltSystemDriver) ListAllUsers(ctx context.Context) ([]*models.SystemUser, error) {
	collection := d.DB.Collection("users")
	var users []models.SystemUser

	err := collection.All(&users, apitobolt.Reverse())
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.SystemUser
	for i := range users {
		results = append(results, &users[i])
	}

	return results, nil
}

// SearchResource searches for any type of resource
func (d *ProBBoltSystemDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// This is a generic search function - for BBolt implementation,
	// we'll search in projects collection by default
	collection := d.DB.Collection("projects")
	var results []any

	// Build query based on parameters
	query := collection.Select().Limit(100)

	err := query.Find(&results)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var resultPtrs []*any
	for i := range results {
		resultPtrs = append(resultPtrs, &results[i])
	}

	return &models.SearchResponse[any]{
		Results: resultPtrs,
	}, nil
}

// Team and Organization related functions

func (d *ProBBoltSystemDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// For BBolt implementation, we'll store team memberships in a simple way
	// In production, you might want a separate collection for team memberships

	// Check if member already exists
	err := d.CheckTeamMemberExists(ctx, req.ProjectID, req.UserID)
	if err != nil {
		return err
	}

	// Create a team membership record
	membership := &TeamMembership{
		ID:          uuid.New().String(),
		ProjectID:   req.ProjectID,
		UserID:      req.UserID,
		Role:        req.Role,
		Permissions: req.Permissions,
		CreatedAt:   utility.GetCurrentTime(),
	}

	collection := d.DB.Collection("team_memberships")
	_, err = collection.Save(membership)
	return err
}

func (d *ProBBoltSystemDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	collection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	query := collection.Select(q.And(
		q.Eq("project_id", projectId),
		q.Eq("user_id", memberID),
	))

	err := query.Find(&memberships)
	if err != nil {
		return err
	}

	if len(memberships) > 0 {
		return errors.New("this member is already added to this project")
	}

	return nil
}

func (d *ProBBoltSystemDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	collection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	query := collection.Select(q.And(
		q.Eq("project_id", projectId),
		q.Eq("user_id", memberID),
	))

	err := query.Find(&memberships)
	if err != nil {
		return err
	}

	// Delete all memberships for this user in this project
	for _, membership := range memberships {
		collection.DeleteStruct(&membership)
	}

	return nil
}

func (d *ProBBoltSystemDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	// Get team memberships for the project
	membershipCollection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	err := membershipCollection.Find("project_id", projectId, &memberships)
	if err != nil {
		return nil, err
	}

	// Get users for each membership
	var userIDs []string
	for _, membership := range memberships {
		userIDs = append(userIDs, membership.UserID)
	}

	return d.GetSystemUsers(ctx, userIDs)
}

func (d *ProBBoltSystemDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	// For BBolt, this is a simple pass-through since we don't have complex metadata resolution
	return docs, nil
}

// Organization functions - simplified implementations

func (d *ProBBoltSystemDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	// Simplified implementation
	return &models.SearchResponse[models.Organization]{
		Results: []*models.Organization{},
	}, nil
}

func (d *ProBBoltSystemDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	// Simplified implementation
	return nil, errors.New("not implemented")
}

func (d *ProBBoltSystemDriver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	// Simplified implementation
	return []*models.Organization{}, nil
}

func (d *ProBBoltSystemDriver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	org.ID = uuid.New().String()
	org.XKey = org.ID
	// Note: Organization model doesn't have CreatedAt/UpdatedAt in the current definition

	collection := d.DB.Collection("organizations")
	_, err := collection.Save(org)
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (d *ProBBoltSystemDriver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	// Simplified implementation
	return errors.New("not implemented")
}

func (d *ProBBoltSystemDriver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	// Simplified implementation
	return errors.New("not implemented")
}

func (d *ProBBoltSystemDriver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	// Simplified implementation
	return errors.New("not implemented")
}

func (d *ProBBoltSystemDriver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	// Simplified implementation
	return errors.New("not implemented")
}

// Team functions

func (d *ProBBoltSystemDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	// Simplified implementation
	return []*models.Team{}, nil
}

func (d *ProBBoltSystemDriver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	// Simplified implementation
	return nil, errors.New("not implemented")
}

func (d *ProBBoltSystemDriver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	team.ID = uuid.New().String()
	team.XKey = team.ID
	// Note: Team model doesn't have CreatedAt/UpdatedAt in the current definition

	collection := d.DB.Collection("teams")
	_, err := collection.Save(team)
	if err != nil {
		return nil, err
	}

	return team, nil
}
