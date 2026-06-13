package mongodb

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const schemaOperationsCollection = "schema_operations"

func (m *SystemMongoDriver) CreateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	if m == nil || m.Database == nil || op == nil {
		return fmt.Errorf("create schema operation: invalid input")
	}
	_, err := m.Database.Collection(schemaOperationsCollection).InsertOne(ctx, op)
	return err
}

func (m *SystemMongoDriver) UpdateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	if m == nil || m.Database == nil || op == nil || op.ID == "" {
		return fmt.Errorf("update schema operation: invalid input")
	}
	_, err := m.Database.Collection(schemaOperationsCollection).ReplaceOne(ctx, bson.M{"id": op.ID}, op)
	return err
}

func (m *SystemMongoDriver) GetSchemaOperation(ctx context.Context, id string) (*models.SchemaOperation, error) {
	if id == "" {
		return nil, fmt.Errorf("get schema operation: id required")
	}
	var op models.SchemaOperation
	err := m.Database.Collection(schemaOperationsCollection).FindOne(ctx, bson.M{"id": id}).Decode(&op)
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (m *SystemMongoDriver) ListSchemaOperationsByStatus(ctx context.Context, projectID string, statuses []string, limit int) ([]*models.SchemaOperation, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{}
	if projectID != "" {
		filter["project_id"] = projectID
	}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	cur, err := m.Database.Collection(schemaOperationsCollection).Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "updated_at", Value: -1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var ops []*models.SchemaOperation
	for cur.Next(ctx) {
		var op models.SchemaOperation
		if err := cur.Decode(&op); err != nil {
			return nil, err
		}
		ops = append(ops, &op)
	}
	return ops, cur.Err()
}
