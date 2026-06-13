package bbolt

import (
	"context"

	"github.com/apito-io/engine/database/project/projectfiles"
	"github.com/apito-io/engine/models"
)

func (b *BBoltDriver) EnsureFilesTable(ctx context.Context) error {
	return projectfiles.EnsureFilesTable(ctx)
}

func (b *BBoltDriver) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	return projectfiles.CreateProjectFile(ctx, file)
}

func (b *BBoltDriver) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	return projectfiles.GetProjectFile(ctx, fileID)
}

func (b *BBoltDriver) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	return projectfiles.SearchProjectFiles(ctx, param)
}

func (b *BBoltDriver) DeleteProjectFiles(ctx context.Context, ids []string) error {
	return projectfiles.DeleteProjectFiles(ctx, ids)
}

func (b *BBoltDriver) SumProjectFilesSize(ctx context.Context) (int64, error) {
	return projectfiles.SumProjectFilesSize(ctx)
}
