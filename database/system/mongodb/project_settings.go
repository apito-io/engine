package mongodb

import (
	"context"
	"errors"
	"strings"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// SaveProjectAuthenticationSettings merges authentication_settings on the project document.
func (m *SystemMongoDriver) SaveProjectAuthenticationSettings(ctx context.Context, projectID string, auth *models.AuthenticationSettings) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id required")
	}
	coll := m.Database.Collection("projects")
	_, err := coll.UpdateOne(ctx, bson.M{"_id": projectID}, bson.M{
		"$set": bson.M{"authentication_settings": auth},
	})
	return err
}

// SaveProjectStorageSettings merges storage_settings on the project document.
func (m *SystemMongoDriver) SaveProjectStorageSettings(ctx context.Context, projectID string, storage *models.StorageSettings) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id required")
	}
	coll := m.Database.Collection("projects")
	_, err := coll.UpdateOne(ctx, bson.M{"_id": projectID}, bson.M{
		"$set": bson.M{"storage_settings": storage},
	})
	return err
}
