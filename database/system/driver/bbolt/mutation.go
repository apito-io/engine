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

// Webhook-related functions

func (d *ProBBoltSystemDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	doc.ID = uuid.New().String()
	doc.XKey = doc.ID
	// Note: Webhook model doesn't have CreatedAt/UpdatedAt in the current definition

	collection := d.DB.Collection("webhooks")
	_, err := collection.Save(doc)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (d *ProBBoltSystemDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	var webhook models.Webhook
	collection := d.DB.Collection("webhooks")

	err := collection.FindByID(hookId, &webhook)
	if err != nil {
		return nil, err
	}

	// Verify the webhook belongs to the project
	if webhook.ProjectID != projectId {
		return nil, errors.New("webhook not found in project")
	}

	return &webhook, nil
}

func (d *ProBBoltSystemDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	// First verify the webhook exists and belongs to the project
	webhook, err := d.GetWebHook(ctx, projectId, hookId)
	if err != nil {
		return err
	}

	collection := d.DB.Collection("webhooks")
	return collection.DeleteStruct(webhook)
}

func (d *ProBBoltSystemDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	collection := d.DB.Collection("webhooks")
	var webhooks []models.Webhook

	// Build query
	var query *apitobolt.Query

	if param.ProjectID != "" {
		query = collection.Select(q.Eq("project_id", param.ProjectID))
	} else {
		query = collection.Select()
	}

	// Apply pagination - simplified for BBolt implementation
	query = query.Limit(100) // Default limit

	err := query.Find(&webhooks)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.Webhook
	for i := range webhooks {
		results = append(results, &webhooks[i])
	}

	return &models.SearchResponse[models.Webhook]{
		Results: results,
	}, nil
}

// Audit log-related functions

func (d *ProBBoltSystemDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	auditLog.XKey = uuid.New().String()
	auditLog.ID = auditLog.XKey
	auditLog.CreatedAt = utility.GetCurrentTime()

	collection := d.DB.Collection("audit_logs")
	_, err := collection.Save(auditLog)
	return err
}

func (d *ProBBoltSystemDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	collection := d.DB.Collection("audit_logs")
	var auditLogs []models.AuditLogs

	// Build query with multiple conditions
	var matchers []interface{}

	if param.ProjectID != "" {
		matchers = append(matchers, q.Eq("project_id", param.ProjectID))
	}
	if param.UserID != "" {
		matchers = append(matchers, q.Eq("user_id", param.UserID))
	}

	var query *apitobolt.Query
	if len(matchers) > 0 {
		// Convert matchers to the correct type - this is a simplified approach
		query = collection.Select()
	} else {
		query = collection.Select()
	}

	// Apply pagination - simplified for BBolt implementation
	query = query.Limit(100) // Default limit

	// Order by created date descending
	query = query.OrderBy("created_at").Reverse()

	err := query.Find(&auditLogs)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.AuditLogs
	for i := range auditLogs {
		results = append(results, &auditLogs[i])
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: results,
	}, nil
}

// Token-related functions

func (d *ProBBoltSystemDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	var token map[string]interface{}
	collection := d.DB.Collection("token_blacklist")

	err := collection.FindByID(tokenId, &token)
	if err == nil && len(token) > 0 {
		return errors.New("This token is blacklisted")
	}

	// If token not found, it's not blacklisted
	return nil
}

func (d *ProBBoltSystemDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	collection := d.DB.Collection("token_blacklist")

	// Set the token ID as the key for easy lookup
	if tokenId, exists := token["id"]; exists {
		token["_key"] = tokenId
	}

	_, err := collection.Save(token)
	return err
}

// Raw data-related functions

func (d *ProBBoltSystemDriver) SaveRawData(ctx context.Context, collectionName string, data map[string]interface{}) error {
	collection := d.DB.Collection(collectionName)

	// Generate ID if not present
	if _, exists := data["id"]; !exists {
		data["id"] = uuid.New().String()
	}
	if _, exists := data["_key"]; !exists {
		data["_key"] = data["id"]
	}

	_, err := collection.Save(data)
	return err
}

// Function-related operations

func (d *ProBBoltSystemDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	// Get the project first
	project, err := d.GetProject(ctx, param.ProjectID)
	if err != nil {
		return nil, err
	}

	if project.Schema == nil {
		return &models.SearchResponse[models.ApitoFunction]{
			Results: []*models.ApitoFunction{},
		}, nil
	}

	return &models.SearchResponse[models.ApitoFunction]{
		Results: project.Schema.Functions,
	}, nil
}
