// Package starterdemo fills the public engine's first-run project with a notebook
// people can open immediately: lists, notes, tags, and sample rows.
package starterdemo

import (
	"context"
	"fmt"
	"log"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	projectdb "github.com/apito-io/open_driver/project"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
	"github.com/apito-io/types"
)

// Ensure creates the Todo Notes schema and sample rows when the starter project
// has no note model yet. A second boot leaves the existing project alone.
func Ensure(ctx context.Context, cfg *models.Config, systemDB interfaces.ApitoSystemDB) error {
	if cfg == nil || systemDB == nil {
		return fmt.Errorf("starter demo: missing config or system database")
	}
	project, err := systemDB.GetProject(ctx, bootstrapmeta.StarterProjectID)
	if err != nil {
		return fmt.Errorf("starter demo: load project: %w", err)
	}
	if project == nil || project.Driver == nil {
		return fmt.Errorf("starter demo: project %s has no database", bootstrapmeta.StarterProjectID)
	}
	if hasModel(project, "note") {
		return nil
	}
	project.Name = "Todo Notes"
	project.Description = "A sample notebook with lists, notes, and tags you can open immediately."
	if err := systemDB.UpdateProject(ctx, project, false); err != nil {
		return fmt.Errorf("starter demo: name project: %w", err)
	}

	admin, err := systemDB.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil {
		return fmt.Errorf("starter demo: load admin: %w", err)
	}
	if admin == nil || admin.ID == "" {
		return fmt.Errorf("starter demo: admin user is missing")
	}

	driver, err := projectdb.GetProjectDriver(project.Driver, cfg)
	if err != nil {
		return fmt.Errorf("starter demo: open project database: %w", err)
	}
	if closer, ok := driver.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	param := &models.CommonSystemParams{ProjectID: project.ID}
	if err := driver.InitProjectBase(ctx, param, nil); err != nil {
		return fmt.Errorf("starter demo: init project tables: %w", err)
	}

	notebook, note, tag := demoModels()
	project.Schema = nil
	for _, model := range []*models.ModelType{notebook, note, tag} {
		if _, err := driver.AddModel(ctx, project, model); err != nil {
			return fmt.Errorf("starter demo: add model %s: %w", model.Name, err)
		}
		for _, field := range model.Fields {
			fieldParam := &models.CommonSystemParams{
				ProjectID: project.ID,
				Model:     model,
				FieldInfo: field,
			}
			if _, err := driver.AddFieldToModel(ctx, fieldParam, false, ""); err != nil {
				return fmt.Errorf("starter demo: add field %s.%s: %w", model.Name, field.Identifier, err)
			}
		}
	}
	if err := driver.EnsureRelationArtifactsFromSchema(ctx, project.Schema.Models); err != nil {
		return fmt.Errorf("starter demo: relation tables: %w", err)
	}
	for _, model := range project.Schema.Models {
		if err := systemDB.UpsertModelType(ctx, project.ID, model); err != nil {
			return fmt.Errorf("starter demo: save model %s: %w", model.Name, err)
		}
	}

	rows, err := insertDemoRows(ctx, driver, project, admin.ID, notebook, note, tag)
	if err != nil {
		return err
	}
	if err := connectDemoRows(ctx, driver, project, note, rows); err != nil {
		return err
	}
	log.Printf("starter demo: %s ready (%d notes)", project.Name, len(rows.notes))
	return nil
}

func hasModel(project *models.Project, name string) bool {
	if project == nil || project.Schema == nil {
		return false
	}
	for _, model := range project.Schema.Models {
		if model != nil && model.Name == name {
			return true
		}
	}
	return false
}

func demoModels() (notebook, note, tag *models.ModelType) {
	notebook = &models.ModelType{
		Name:           "notebook",
		HasConnections: true,
		Fields: []*models.FieldInfo{
			field("name", "Name", _const.TextField, _const.StringInput, 1),
			field("description", "Description", _const.MultilineField, _const.StringInput, 2),
		},
	}
	note = &models.ModelType{
		Name:           "note",
		HasConnections: true,
		Fields: []*models.FieldInfo{
			field("title", "Title", _const.TextField, _const.StringInput, 1),
			field("body", "Body", _const.MultilineField, _const.StringInput, 2),
			field("done", "Done", _const.BooleanField, _const.BoolInput, 3),
			field("due_on", "Due", _const.DateField, _const.StringInput, 4),
		},
	}
	tag = &models.ModelType{
		Name:           "tag",
		HasConnections: true,
		Fields: []*models.FieldInfo{
			field("name", "Name", _const.TextField, _const.StringInput, 1),
		},
	}
	notebook.Connections = []*models.ConnectionType{
		{Model: "note", Relation: "has_many", Type: "forward"},
	}
	note.Connections = []*models.ConnectionType{
		{Model: "notebook", Relation: "has_one", Type: "backward"},
		{Model: "tag", Relation: "has_many", Type: "forward"},
	}
	tag.Connections = []*models.ConnectionType{
		{Model: "note", Relation: "has_many", Type: "backward"},
	}
	return notebook, note, tag
}

func field(identifier, label, fieldType, inputType string, serial uint32) *models.FieldInfo {
	return &models.FieldInfo{
		Identifier: identifier,
		Label:      label,
		FieldType:  fieldType,
		InputType:  inputType,
		Serial:     serial,
	}
}

type demoRows struct {
	notebooks map[string]string
	tags      map[string]string
	notes     []seedNote
}

type seedNote struct {
	id       string
	notebook string
	tags     []string
}

func insertDemoRows(ctx context.Context, driver interfaces.ProjectDBInterface, project *models.Project, userID string, notebook, note, tag *models.ModelType) (*demoRows, error) {
	rows := &demoRows{
		notebooks: map[string]string{},
		tags:      map[string]string{},
	}
	notebooks := []struct{ name, description string }{
		{"Work", "Shipping notes for the week."},
		{"Home", "Errands and the flat."},
		{"Reading", "Books and articles to come back to."},
	}
	for _, item := range notebooks {
		id, err := insertDoc(ctx, driver, project.ID, userID, notebook, map[string]interface{}{
			"name":        item.name,
			"description": item.description,
		})
		if err != nil {
			return nil, fmt.Errorf("starter demo: notebook %s: %w", item.name, err)
		}
		rows.notebooks[item.name] = id
	}
	for _, name := range []string{"urgent", "personal", "writing", "later"} {
		id, err := insertDoc(ctx, driver, project.ID, userID, tag, map[string]interface{}{
			"name": name,
		})
		if err != nil {
			return nil, fmt.Errorf("starter demo: tag %s: %w", name, err)
		}
		rows.tags[name] = id
	}
	notes := []struct {
		title, body, due, notebook string
		done                       bool
		tags                       []string
	}{
		{"Draft the release notes", "Cover the public builder, the sample notebook, and what stays on Cloud.", "2026-10-16", "Work", false, []string{"writing"}},
		{"Fix the login redirect on Safari", "Same-origin session cookie was dropping the project claim.", "2026-10-10", "Work", true, []string{"urgent"}},
		{"Pair on the schema screen", "Walk the pending table with Nadia before publishing.", "2026-10-13", "Work", false, nil},
		{"Buy oat milk, coffee, and eggs", "The corner shop closes at 8.", "2026-10-12", "Home", false, []string{"personal"}},
		{"Replace the hallway bulb", "Warm white, the same fitting as the kitchen.", "2026-10-18", "Home", false, nil},
		{"Email the landlord about the kitchen leak", "Photo is in the thread from Tuesday.", "2026-10-08", "Home", true, []string{"urgent", "personal"}},
		{"Finish the SQLite internals chapter", "Stop after the pager section and write down the page size.", "2026-10-20", "Reading", false, []string{"later"}},
		{"Save the grocery-list idea", "A shared list for the flat, pinned to the fridge note.", "", "Reading", false, []string{"writing", "later"}},
	}
	for _, item := range notes {
		data := map[string]interface{}{
			"title": item.title,
			"body":  item.body,
			"done":  item.done,
		}
		if item.due != "" {
			data["due_on"] = item.due
		}
		id, err := insertDoc(ctx, driver, project.ID, userID, note, data)
		if err != nil {
			return nil, fmt.Errorf("starter demo: note %s: %w", item.title, err)
		}
		rows.notes = append(rows.notes, seedNote{id: id, notebook: item.notebook, tags: item.tags})
	}
	return rows, nil
}

func insertDoc(ctx context.Context, driver interfaces.ProjectDBInterface, projectID, userID string, model *models.ModelType, data map[string]interface{}) (string, error) {
	id := utility.NewID()
	now := utility.GetCurrentTime()
	doc := &types.DefaultDocumentStructure{
		Key:  id,
		ID:   id,
		Type: model.Name,
		Data: data,
		Meta: &types.MetaField{
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: &types.SystemUser{ID: userID},
			LastModifiedBy: &types.SystemUser{
				ID: userID,
			},
			Status: "published",
		},
	}
	param := &models.CommonSystemParams{ProjectID: projectID, Model: model}
	if _, err := driver.AddDocumentToProject(ctx, param, doc); err != nil {
		return "", err
	}
	return id, nil
}

func connectDemoRows(ctx context.Context, driver interfaces.ProjectDBInterface, project *models.Project, note *models.ModelType, rows *demoRows) error {
	for _, item := range rows.notes {
		payload := map[string]interface{}{
			"notebook_id": rows.notebooks[item.notebook],
		}
		if len(item.tags) > 0 {
			ids := make([]interface{}, 0, len(item.tags))
			for _, name := range item.tags {
				ids = append(ids, rows.tags[name])
			}
			payload["tag_ids"] = ids
		}
		built, err := utility.ConnectDisconnectParamBuilder(project, item.id, payload, note)
		if err != nil {
			return fmt.Errorf("starter demo: connect %s: %w", item.id, err)
		}
		param := &models.CommonSystemParams{
			ProjectID:   project.ID,
			Model:       note,
			ConDisParam: built,
		}
		if err := driver.ConnectBuilder(ctx, param); err != nil {
			return fmt.Errorf("starter demo: connect %s: %w", item.id, err)
		}
	}
	return nil
}
