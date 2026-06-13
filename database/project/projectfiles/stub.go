package projectfiles

import (
	"context"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
)

func CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	_ = ctx
	_ = file
	return nil, ae.ErrProjectFilesUnsupported
}

func GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	_ = ctx
	_ = fileID
	return nil, ae.ErrProjectFilesUnsupported
}

func SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	_ = ctx
	_ = param
	return nil, ae.ErrProjectFilesUnsupported
}

func DeleteProjectFiles(ctx context.Context, ids []string) error {
	_ = ctx
	_ = ids
	return ae.ErrProjectFilesUnsupported
}

func EnsureFilesTable(ctx context.Context) error {
	_ = ctx
	return ae.ErrProjectFilesUnsupported
}

func SumProjectFilesSize(ctx context.Context) (int64, error) {
	_ = ctx
	return 0, ae.ErrProjectFilesUnsupported
}
