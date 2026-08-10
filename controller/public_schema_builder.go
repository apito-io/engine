package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/graph-gophers/dataloader"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func remove(s []*models.ModelType, i int) []*models.ModelType {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func (g *GraphCtrl) publicSchemaBuilder(ctx context.Context, cache *models.ApplicationCache) (*models.ApplicationCache, error) {
	if err := checkCtxDone(ctx); err != nil {
		return nil, err
	}

	project := cache.Project
	if project.Schema == nil {
		return nil, errors.New("user Defined Schema Not Found")
	}

	if cache.Param != nil && cache.Param.Role == nil {
		return nil, errors.New("cant Build Schema Without a Role")
	}

	role := cache.Param.Role

	var span trace.Span
	if g.cfg != nil && g.cfg.SchemaBuildTelemetry {
		ctx, span = tracer.Start(ctx, "publicSchemaBuilder")
		defer span.End()
		span.SetAttributes(
			attribute.String("project_id", project.ID),
			attribute.Int("model_count", len(project.Schema.Models)),
		)
	}

	roleAgnostic := g.cfg != nil && g.cfg.RoleAgnosticSchemaCache
	schemaRole := schemaRoleForPublicSchemaBuild(role, roleAgnostic)

	start := time.Now()
	var buildErr error
	defer func() {
		recordSchemaBuildDuration(ctx, g.cfg, time.Since(start), buildErr)
	}()

	permissions, filteredModels, filteredFunctions, operationType, err := collectFilteredModelsForPublicSchema(project, cache, schemaRole)
	if err != nil {
		buildErr = err
		recordSchemaBuildOutcome(ctx, g.cfg, "error")
		return nil, err
	}

	if g.cfg != nil && g.cfg.AdjustPublicSchemaForRequestHook != nil {
		if err := g.cfg.AdjustPublicSchemaForRequestHook(ctx, cache, project, permissions, &filteredModels); err != nil {
			buildErr = err
			recordSchemaBuildOutcome(ctx, g.cfg, "error")
			return nil, err
		}
	}

	// Model/function filters can be empty when the request is only public auth
	// roots (myEffectivePermissions, loginUser, myTenant, …). Those fields are
	// merged later via PublicAuthQueryFields — do not treat that as "not found".
	if len(filteredFunctions) == 0 && len(filteredModels) == 0 {
		authNames := publicAuthQueryNameSet(g.gqlServer)
		if !incomingRequestIsPublicAuthOnly(cache, authNames) {
			buildErr = errors.New("query not found in schema. please re-check")
			recordSchemaBuildOutcome(ctx, g.cfg, "error")
			return nil, buildErr
		}
	}

	if g.cfg != nil && g.cfg.MaxModelsPerProject > 0 && len(filteredModels) > g.cfg.MaxModelsPerProject {
		if span != nil {
			span.SetStatus(codes.Error, "too_many_models")
		}
		buildErr = fmt.Errorf("schema too large: %d models exceeds MaxModelsPerProject=%d", len(filteredModels), g.cfg.MaxModelsPerProject)
		recordSchemaBuildOutcome(ctx, g.cfg, "error")
		return nil, buildErr
	}

	allLoaders := make(map[string]*dataloader.Loader)
	allLoaders["system_user_loader"] = dataloader.NewBatchedLoader(g.gqlServer.SystemUserMetaLoader)

	requestShapeFP := ""
	if g.cfg != nil && g.cfg.AdjustPublicSchemaForRequestHook != nil {
		requestShapeFP = fingerprintPublicSchemaRequestShape(permissions, filteredModels)
	}
	preConnKey := fingerprintPreConnection(project, fingerprintRoleForPreConnectionCache(role, roleAgnostic), cache.IncomingRequest, requestShapeFP)

	var localeList []string
	if project.Settings != nil {
		localeList = project.Settings.Locals
	}
	localEnum := enums.BuildLocalEnum(localeList)
	metaObject := objects.BuildMetaObject(ctx, project.ID)

	st := &publicSchemaBuildState{
		g:                 g,
		ctx:               ctx,
		span:              span,
		cache:             cache,
		project:           project,
		schemaRole:        schemaRole,
		permissions:       permissions,
		filteredModels:    filteredModels,
		filteredFunctions: filteredFunctions,
		operationType:     operationType,
		preConnKey:        preConnKey,
		localEnum:         localEnum,
		metaObject:        metaObject,
		allLoaders:        allLoaders,
	}

	if err := st.loadOrBuildPreConnectionMaps(); err != nil {
		buildErr = err
		recordSchemaBuildOutcome(ctx, g.cfg, "error")
		return nil, err
	}

	if g.cfg != nil && g.cfg.EnableCompiledSchemaCache {
		if st.skipPre {
			recordSchemaBuildOutcome(ctx, g.cfg, "hit")
		} else {
			recordSchemaBuildOutcome(ctx, g.cfg, "miss")
		}
	}

	st.attachConnectionFields()
	st.buildQueryAndMutationTypes()

	out, err := st.mergeFunctionAndPluginFields()
	if err != nil {
		buildErr = err
		recordSchemaBuildOutcome(ctx, g.cfg, "error")
		return nil, err
	}

	return out, nil
}
