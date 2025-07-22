package firestore

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
)

func (a *FireStoreDriver) RemoveAuthAddOns(ctx context.Context, project *models.Project, option map[string]interface{}) error {
	return nil
}

func (a *FireStoreDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	_, err := a.Db.Collection(param.Model.Name).Doc(doc.ID).Set(ctx, doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
