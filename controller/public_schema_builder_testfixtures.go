package controller

import (
	"fmt"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func baseProject(id string) *models.Project {
	return &models.Project{
		ID: id,
		Settings: &models.ProjectSettings{
			Locals: []string{"en"},
		},
	}
}

func adminRole() *models.Role {
	return &models.Role{ID: "admin", IsAdmin: true}
}

// fixtureOneModelAdmin returns a single-model project with an admin role.
func fixtureOneModelAdmin() *models.ApplicationCache {
	p := baseProject("proj-golden-1")
	p.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{
				Name: "article",
				Fields: []*models.FieldInfo{
					{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
				},
			},
		},
	}
	return &models.ApplicationCache{
		Project: p,
		Param: &models.CommonSystemParams{
			ProjectID: "proj-golden-1",
			Role:      adminRole(),
		},
	}
}

// fixtureTenModelsAdmin returns ten simple models (stress baseline for -long).
func fixtureTenModelsAdmin() *models.ApplicationCache {
	var modelsList []*models.ModelType
	for i := 0; i < 10; i++ {
		modelsList = append(modelsList, &models.ModelType{
			Name: fmt.Sprintf("m%d", i),
			Fields: []*models.FieldInfo{
				{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
			},
		})
	}
	p := baseProject("proj-golden-10")
	p.Schema = &models.ProjectSchema{Models: modelsList}
	return &models.ApplicationCache{
		Project: p,
		Param: &models.CommonSystemParams{
			ProjectID: p.ID,
			Role:      adminRole(),
		},
	}
}

// fixtureRelationsAdmin has has_one and has_many between models.
func fixtureRelationsAdmin() *models.ApplicationCache {
	p := baseProject("proj-golden-rel")
	p.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{
				Name: "author",
				Fields: []*models.FieldInfo{
					{Identifier: "name", InputType: _const.StringInput, FieldType: "string"},
				},
			},
			{
				Name: "post",
				Fields: []*models.FieldInfo{
					{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
				},
				Connections: []*models.ConnectionType{
					{Model: "author", Relation: "has_one", KnownAs: ""},
				},
			},
			{
				Name: "tag",
				Fields: []*models.FieldInfo{
					{Identifier: "label", InputType: _const.StringInput, FieldType: "string"},
				},
			},
			{
				Name: "bucket",
				Fields: []*models.FieldInfo{
					{Identifier: "name", InputType: _const.StringInput, FieldType: "string"},
				},
				Connections: []*models.ConnectionType{
					{Model: "tag", Relation: "has_many", KnownAs: "tags"},
				},
			},
		},
	}
	return &models.ApplicationCache{
		Project: p,
		Param: &models.CommonSystemParams{
			ProjectID: p.ID,
			Role:      adminRole(),
		},
	}
}

// fixtureRestrictedReader has read on article only (no admin).
func fixtureRestrictedReader() *models.ApplicationCache {
	p := baseProject("proj-golden-r")
	p.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{
				Name: "article",
				Fields: []*models.FieldInfo{
					{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
				},
			},
			{
				Name: "secret",
				Fields: []*models.FieldInfo{
					{Identifier: "body", InputType: _const.StringInput, FieldType: "string"},
				},
			},
		},
	}
	return &models.ApplicationCache{
		Project: p,
		Param: &models.CommonSystemParams{
			ProjectID: "proj-golden-r",
			Role: &models.Role{
				ID:      "reader",
				IsAdmin: false,
				APIPermissions: map[string]*models.APIPermission{
					// Create allowed so the public schema exposes at least one mutation (graphql.NewSchema requires non-empty mutation object).
					"article": {Read: "all", Create: "all", Update: "none", Delete: "none"},
				},
			},
		},
	}
}
