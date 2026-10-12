package resolver

import (
	"context"
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services/schemadraft"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

type localSchemaReplayKey struct{}

type localSchemaOp struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Args      map[string]interface{} `json:"args"`
	CreatedAt string                 `json:"created_at"`
}

type localSchemaDraft struct {
	ProjectID   string          `json:"project_id"`
	ChangesetID string          `json:"changeset_id,omitempty"`
	Ops         []localSchemaOp `json:"ops"`
}

func echoProjectID(c echo.Context) string {
	for _, key := range []string{"project_id", "project"} {
		v := c.Get(key)
		s, ok := v.(string)
		if ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func readLocalSchemaDraft(projectID string) (*localSchemaDraft, error) {
	file, err := schemadraft.Load(projectID)
	if err != nil {
		return nil, err
	}
	draft := &localSchemaDraft{ProjectID: file.ProjectID, ChangesetID: file.ChangesetID}
	for _, op := range file.Ops {
		draft.Ops = append(draft.Ops, localSchemaOp{ID: op.ID, Type: op.Type, Args: op.Args, CreatedAt: op.CreatedAt})
	}
	return draft, nil
}

func (s *GraphQLServer) stageLocalSchemaDraft(project *models.Project, operationType string, args map[string]interface{}) (interface{}, error) {
	if project == nil || strings.TrimSpace(project.ID) == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "project is required")
	}
	created := utility.FormatReadableDate("", "")
	if created == "" {
		created = "staged"
	}
	op := schemadraft.Op{
		ID:        utility.NewID(),
		Type:      operationType,
		Args:      args,
		CreatedAt: created,
	}
	if _, err := schemadraft.Append(project.ID, op); err != nil {
		return nil, err
	}
	return localStagingResponse(operationType, args, op.ID), nil
}

func localStagingResponse(operationType string, args map[string]interface{}, id string) interface{} {
	name, _ := args["name"].(string)
	if name == "" {
		name, _ = args["model_name"].(string)
	}
	row := map[string]interface{}{
		"name":       name,
		"model_name": name,
		"staged":     true,
		"draft_id":   id,
		"message":    "Staged on this machine. Publish the schema draft to apply it.",
	}
	switch operationType {
	case models.SchemaOpTypeCreateModel, models.SchemaOpTypeCreateConnection, models.SchemaOpTypeUpdateConnection:
		return []interface{}{row}
	default:
		return row
	}
}

func (s *GraphQLServer) replayLocalSchemaOp(ctx context.Context, op localSchemaOp) error {
	fn := s.localSchemaReplayFn(op.Type)
	if fn == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot publish "+op.Type)
	}
	_, err := fn(graphql.ResolveParams{Context: ctx, Args: op.Args})
	return err
}

func (s *GraphQLServer) localSchemaReplayFn(operationType string) func(graphql.ResolveParams) (interface{}, error) {
	switch operationType {
	case models.SchemaOpTypeCreateModel:
		return s.AddModelToProjectResolverFn
	case models.SchemaOpTypeUpdateModel, models.SchemaOpTypeDeleteModel, models.SchemaOpTypeRenameModel, models.SchemaOpTypeDuplicateModel, models.SchemaOpTypeConvertModel:
		return s.UpdateModelResolverFn
	case models.SchemaOpTypeAddField:
		return s.UpsertFieldToModelResolverFn
	case models.SchemaOpTypeRearrangeField:
		return s.RearrangeFieldOfModelResolverFn
	case models.SchemaOpTypeModelFieldOp:
		return s.ModelFieldOperationResolverFn
	case models.SchemaOpTypeCreateConnection:
		return s.CreateConnectionTypeResolverFn
	case models.SchemaOpTypeUpdateConnection:
		return s.StageRelationTransitionResolverFn
	case models.SchemaOpTypeDeleteConnection:
		return s.DeleteConnectionFromModelResolverFn
	case models.SchemaOpTypeUpsertUniqueKey:
		return s.UpsertUniqueKeyResolverFn
	case models.SchemaOpTypeDeleteUniqueKey:
		return s.DeleteUniqueKeyResolverFn
	default:
		return nil
	}
}

// HandleLocalSchemaDraft serves the in-process schema draft for the active project.
func HandleLocalSchemaDraft(c echo.Context, server interface{}) error {
	gs, _ := server.(*GraphQLServer)
	if gs == nil {
		return c.JSON(http.StatusServiceUnavailable, &models.HttpResponse{Message: "schema draft unavailable", Code: http.StatusServiceUnavailable})
	}
	projectID := strings.TrimSpace(c.QueryParam("project_id"))
	if projectID == "" {
		projectID = echoProjectID(c)
	}
	if projectID == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "project_id is required", Code: http.StatusBadRequest})
	}
	switch c.Request().Method {
	case http.MethodDelete:
		if err := schemadraft.Remove(projectID); err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		return c.JSON(http.StatusOK, &models.HttpResponse{Code: http.StatusOK, Message: "draft discarded"})
	case http.MethodPost:
		return gs.publishLocalSchemaDraft(c, projectID)
	default:
		draft, err := readLocalSchemaDraft(projectID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		return c.JSON(http.StatusOK, &models.HttpResponse{Code: http.StatusOK, Body: draft})
	}
}

func (s *GraphQLServer) publishSchemaDraftOps(ctx context.Context, project *models.Project, ops []schemadraft.Op) error {
	ctx = context.WithValue(ctx, localSchemaReplayKey{}, true)
	for _, op := range ops {
		if err := s.replayLocalSchemaOp(ctx, localSchemaOp{ID: op.ID, Type: op.Type, Args: op.Args, CreatedAt: op.CreatedAt}); err != nil {
			return err
		}
	}
	_ = project
	return nil
}

func (s *GraphQLServer) publishLocalSchemaDraft(c echo.Context, projectID string) error {
	draft, err := readLocalSchemaDraft(projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
	}
	if len(draft.Ops) == 0 {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "no schema draft to publish", Code: http.StatusBadRequest})
	}
	ctx := context.WithValue(c.Request().Context(), "router", c)
	ops := make([]schemadraft.Op, 0, len(draft.Ops))
	for _, op := range draft.Ops {
		ops = append(ops, schemadraft.Op{ID: op.ID, Type: op.Type, Args: op.Args, CreatedAt: op.CreatedAt})
	}
	if err := s.publishSchemaDraftOps(ctx, &models.Project{ID: projectID}, ops); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
	}
	if err := schemadraft.Remove(projectID); err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{Code: http.StatusOK, Message: "schema draft published"})
}
