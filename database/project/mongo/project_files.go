package mongo

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func projectIDFromContext(ctx context.Context) string {
	if v := ctx.Value("project_id"); v != nil {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (m *MongoDriver) projectFilesCollection(ctx context.Context) (*mongo.Collection, error) {
	pid := projectIDFromContext(ctx)
	if pid == "" {
		return nil, fmt.Errorf("project id is required in context")
	}
	return m.Database.Collection(fmt.Sprintf("p_%s_files", pid)), nil
}

func (m *MongoDriver) EnsureFilesTable(ctx context.Context) error {
	pid := projectIDFromContext(ctx)
	if pid == "" {
		return fmt.Errorf("project id is required in context")
	}
	collName := fmt.Sprintf("p_%s_files", pid)
	names, err := m.Database.ListCollectionNames(ctx, bson.M{"name": collName})
	if err != nil {
		return err
	}
	if len(names) > 0 {
		return m.migrateLegacyMongoMedia(ctx, pid)
	}
	if err := m.Database.CreateCollection(ctx, collName); err != nil {
		return err
	}
	return m.migrateLegacyMongoMedia(ctx, pid)
}

func (m *MongoDriver) migrateLegacyMongoMedia(ctx context.Context, pid string) error {
	legacy := fmt.Sprintf("p_%s_media", pid)
	names, err := m.Database.ListCollectionNames(ctx, bson.M{"name": legacy})
	if err != nil || len(names) == 0 {
		return nil
	}
	filesColl, err := m.projectFilesCollection(ctx)
	if err != nil {
		return err
	}
	legacyColl := m.Database.Collection(legacy)
	cur, err := legacyColl.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return err
		}
		file := mediaDocToProjectFile(doc, pid)
		if file == nil {
			continue
		}
		_, _ = filesColl.InsertOne(ctx, file)
	}
	return mongoDropCollectionIgnoringNotFound(ctx, legacyColl)
}

func mediaDocToProjectFile(doc bson.M, projectID string) *models.ProjectFile {
	id, _ := doc["id"].(string)
	if id == "" {
		if oid, ok := doc["_id"].(string); ok {
			id = oid
		}
	}
	if id == "" {
		return nil
	}
	mediaType, _ := doc["media_type"].(string)
	fileType := models.InferFileTypeFromMIME(mediaType)
	fileName, _ := doc["file_name"].(string)
	fileExt, _ := doc["file_extension"].(string)
	s3Key, _ := doc["s3_key"].(string)
	url, _ := doc["url"].(string)
	var size int64
	switch v := doc["size"].(type) {
	case int32:
		size = int64(v)
	case int64:
		size = v
	case float64:
		size = int64(v)
	}
	createdAt, _ := doc["created_at"].(string)
	return &models.ProjectFile{
		ID:            id,
		ProjectID:     projectID,
		FileType:      fileType,
		FileName:      fileName,
		FileExtension: fileExt,
		ContentType:   mediaType,
		Size:          size,
		StorageKey:    s3Key,
		URL:           url,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
}

func (m *MongoDriver) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	if err := m.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	coll, err := m.projectFilesCollection(ctx)
	if err != nil {
		return nil, err
	}
	_, err = coll.InsertOne(ctx, file)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (m *MongoDriver) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, fmt.Errorf("file id is required")
	}
	coll, err := m.projectFilesCollection(ctx)
	if err != nil {
		return nil, err
	}
	var file models.ProjectFile
	err = coll.FindOne(ctx, bson.M{"id": fileID}).Decode(&file)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (m *MongoDriver) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	if err := m.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	fileType, limit, offset := models.SystemFileListParams(param)
	filter := bson.M{}
	if fileType != "" {
		filter["file_type"] = fileType
	}
	coll, err := m.projectFilesCollection(ctx)
	if err != nil {
		return nil, err
	}
	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var files []*models.ProjectFile
	for cursor.Next(ctx) {
		var f models.ProjectFile
		if err := cursor.Decode(&f); err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	return &models.SearchResponse[models.ProjectFile]{
		Results: files,
		Total:   total,
	}, nil
}

func (m *MongoDriver) DeleteProjectFiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	coll, err := m.projectFilesCollection(ctx)
	if err != nil {
		return err
	}
	_, err = coll.DeleteMany(ctx, bson.M{"id": bson.M{"$in": ids}})
	return err
}

func (m *MongoDriver) SumProjectFilesSize(ctx context.Context) (int64, error) {
	if err := m.EnsureFilesTable(ctx); err != nil {
		return 0, err
	}
	coll, err := m.projectFilesCollection(ctx)
	if err != nil {
		return 0, err
	}
	cursor, err := coll.Aggregate(ctx, bson.A{
		bson.M{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$size"},
		}},
	})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	if !cursor.Next(ctx) {
		return 0, nil
	}
	var out struct {
		Total int64 `bson:"total"`
	}
	if err := cursor.Decode(&out); err != nil {
		return 0, err
	}
	return out.Total, nil
}
