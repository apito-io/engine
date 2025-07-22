package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// GetProjects retrieves multiple projects by their IDs using DynamoDB
func (d *DynamoDBDriver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	var projects []*models.Project

	for _, key := range keys {
		project, err := d.GetProject(ctx, key)
		if err != nil {
			continue // Skip missing projects
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// AddATeamMemberToProject adds a team member to a project using DynamoDB
func (d *DynamoDBDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// Get the user to add
	user, err := d.GetSystemUser(ctx, req.UserID)
	if err != nil {
		return err
	}

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// Store team member in project_teams table
	projectTeamsTable := d.TablePrefix + "project_teams"
	projectTeamItem := map[string]types.AttributeValue{
		"project_id": &types.AttributeValueMemberS{Value: req.ProjectID},
		"user_id":    &types.AttributeValueMemberS{Value: req.UserID},
		"user_data":  &types.AttributeValueMemberS{Value: string(userDataJson)},
	}

	input1 := &dynamodb.PutItemInput{
		TableName: aws.String(projectTeamsTable),
		Item:      projectTeamItem,
	}

	_, err = d.Client.PutItem(ctx, input1)
	if err != nil {
		return err
	}

	// Store user-project relation with role
	project, err := d.GetProject(ctx, req.ProjectID)
	if err != nil {
		return err
	}

	projectDataJson, _ := json.Marshal(project)
	permissionsData, _ := attributevalue.Marshal(req.Permissions)

	userProjectsTable := d.TablePrefix + "user_projects"
	userProjectItem := map[string]types.AttributeValue{
		"user_id":      &types.AttributeValueMemberS{Value: req.UserID},
		"project_id":   &types.AttributeValueMemberS{Value: req.ProjectID},
		"project_data": &types.AttributeValueMemberS{Value: string(projectDataJson)},
		"role":         &types.AttributeValueMemberS{Value: req.Role},
		"permissions":  permissionsData,
	}

	input2 := &dynamodb.PutItemInput{
		TableName: aws.String(userProjectsTable),
		Item:      userProjectItem,
	}

	_, err = d.Client.PutItem(ctx, input2)
	return err
}

// GetSystemUsers retrieves multiple system users by their IDs using DynamoDB
func (d *DynamoDBDriver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	for _, key := range keys {
		user, err := d.GetSystemUser(ctx, key)
		if err != nil {
			continue // Skip missing users
		}
		users = append(users, user)
	}

	return users, nil
}

// FindUserOrganizations retrieves all organizations for a given user using DynamoDB
func (d *DynamoDBDriver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	tableName := d.TablePrefix + "user_organizations"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var organizations []*models.Organization
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if orgDataStr, ok := data["organization_data"].(string); ok {
			var org models.Organization
			if err := json.Unmarshal([]byte(orgDataStr), &org); err == nil {
				organizations = append(organizations, &org)
			}
		}
	}

	return organizations, nil
}

// CreateOrganization creates a new organization using DynamoDB
func (d *DynamoDBDriver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	orgDataJson, err := json.Marshal(org)
	if err != nil {
		return nil, err
	}

	tableName := d.TablePrefix + "organizations"
	item := map[string]types.AttributeValue{
		"id":                &types.AttributeValueMemberS{Value: org.ID},
		"name":              &types.AttributeValueMemberS{Value: org.Name},
		"organization_data": &types.AttributeValueMemberS{Value: string(orgDataJson)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return org, err
}

// AssignTeamToOrganization assigns a team to an organization using DynamoDB
func (d *DynamoDBDriver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	tableName := d.TablePrefix + "organization_teams"

	item := map[string]types.AttributeValue{
		"organization_id": &types.AttributeValueMemberS{Value: orgId},
		"team_id":         &types.AttributeValueMemberS{Value: teamId},
		"assigned_by":     &types.AttributeValueMemberS{Value: userId},
		"assigned_at":     &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err := d.Client.PutItem(ctx, input)
	return err
}

// RemoveATeamFromOrganization removes a team from an organization using DynamoDB
func (d *DynamoDBDriver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	tableName := d.TablePrefix + "organization_teams"

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"organization_id": &types.AttributeValueMemberS{Value: orgId},
			"team_id":         &types.AttributeValueMemberS{Value: teamId},
		},
	}

	_, err := d.Client.DeleteItem(ctx, input)
	return err
}

// AssignProjectToOrganization assigns a project to an organization using DynamoDB
func (d *DynamoDBDriver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	tableName := d.TablePrefix + "organization_projects"

	item := map[string]types.AttributeValue{
		"organization_id": &types.AttributeValueMemberS{Value: orgId},
		"project_id":      &types.AttributeValueMemberS{Value: projectId},
		"assigned_by":     &types.AttributeValueMemberS{Value: userId},
		"assigned_at":     &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err := d.Client.PutItem(ctx, input)
	return err
}

// RemoveProjectFromOrganization removes a project from an organization using DynamoDB
func (d *DynamoDBDriver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	tableName := d.TablePrefix + "organization_projects"

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"organization_id": &types.AttributeValueMemberS{Value: orgId},
			"project_id":      &types.AttributeValueMemberS{Value: projectId},
		},
	}

	_, err := d.Client.DeleteItem(ctx, input)
	return err
}

// GetProjectTeams retrieves team information for a project using DynamoDB
func (d *DynamoDBDriver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	tableName := d.TablePrefix + "team_projects"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: projectId},
		},
		Limit: aws.Int32(1),
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("team not found for project")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Items[0], &data)
	if err != nil {
		return nil, err
	}

	if teamDataStr, ok := data["team_data"].(string); ok {
		var team models.Team
		if err := json.Unmarshal([]byte(teamDataStr), &team); err != nil {
			return nil, err
		}
		return &team, nil
	}

	return nil, fmt.Errorf("team data not found")
}

// FindUserTeams retrieves all teams for a given user using DynamoDB
func (d *DynamoDBDriver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	tableName := d.TablePrefix + "user_teams"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var teams []*models.Team
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if teamDataStr, ok := data["team_data"].(string); ok {
			var team models.Team
			if err := json.Unmarshal([]byte(teamDataStr), &team); err == nil {
				teams = append(teams, &team)
			}
		}
	}

	return teams, nil
}

// CreateTeam creates a new team using DynamoDB
func (d *DynamoDBDriver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	teamDataJson, err := json.Marshal(team)
	if err != nil {
		return nil, err
	}

	// Store the team
	teamsTable := d.TablePrefix + "teams"
	teamItem := map[string]types.AttributeValue{
		"id":        &types.AttributeValueMemberS{Value: team.ID},
		"name":      &types.AttributeValueMemberS{Value: team.Name},
		"team_data": &types.AttributeValueMemberS{Value: string(teamDataJson)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(teamsTable),
		Item:      teamItem,
	}

	_, err = d.Client.PutItem(ctx, input)
	if err != nil {
		return nil, err
	}

	// Store user-team relations for each user
	userTeamsTable := d.TablePrefix + "user_teams"
	for _, user := range team.Users {
		userTeamItem := map[string]types.AttributeValue{
			"user_id":   &types.AttributeValueMemberS{Value: user.ID},
			"team_id":   &types.AttributeValueMemberS{Value: team.ID},
			"team_data": &types.AttributeValueMemberS{Value: string(teamDataJson)},
		}

		userTeamInput := &dynamodb.PutItemInput{
			TableName: aws.String(userTeamsTable),
			Item:      userTeamItem,
		}

		_, err = d.Client.PutItem(ctx, userTeamInput)
		if err != nil {
			return nil, err
		}
	}

	return team, nil
}

// FindUserProjects retrieves all projects for a given user using DynamoDB
func (d *DynamoDBDriver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	tableName := d.TablePrefix + "user_projects"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var projects []*models.Project
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if projectDataStr, ok := data["project_data"].(string); ok {
			var project models.Project
			if err := json.Unmarshal([]byte(projectDataStr), &project); err == nil {
				projects = append(projects, &project)
			}
		}
	}

	return projects, nil
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions using DynamoDB
func (d *DynamoDBDriver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	// Get the project
	project, err := d.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	// Get the user
	user, err := d.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	// Get user-project relation with role and permissions
	tableName := d.TablePrefix + "user_projects"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id":    &types.AttributeValueMemberS{Value: userId},
			"project_id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user is not associated with this project")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
		return nil, err
	}

	role, _ := data["role"].(string)

	var permissions []string
	if permissionsData, ok := data["permissions"].([]interface{}); ok {
		for _, perm := range permissionsData {
			if p, ok := perm.(string); ok {
				permissions = append(permissions, p)
			}
		}
	}

	return &models.ProjectWithRoles{
		User:        user,
		Project:     project,
		Role:        role,
		Permissions: permissions,
	}, nil
}
