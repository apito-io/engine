package sqlite

import (
	"context"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func (d *Driver) EnsureFilesTable(ctx context.Context) error {
	if err := d.EnsureMetaFilesTables(ctx); err != nil {
		return err
	}
	return d.migrateLegacyMediaToFiles(ctx)
}

func (d *Driver) migrateLegacyMediaToFiles(ctx context.Context) error {
	if !d.legacyMediaTableExists(ctx) {
		return nil
	}
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		_, err := d.ORM.NewRaw(`
INSERT INTO files (id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_at, updated_at)
SELECT m.id,
  CASE WHEN m.media_type LIKE 'image/%' THEN 'media'
       WHEN m.media_type = 'application/pdf' THEN 'pdf'
       ELSE 'other' END,
  COALESCE(m.file_name, ''),
  m.file_extension,
  m.media_type,
  COALESCE(m.size, 0),
  COALESCE(m.s3_key, ''),
  m.url,
  COALESCE(m.created_at::text, ''),
  COALESCE(m.created_at::text, '')
FROM media m
WHERE NOT EXISTS (SELECT 1 FROM files f WHERE f.id = m.id);`).Exec(ctx)
		if err != nil {
			return err
		}
		_, err = d.ORM.NewRaw(`DROP TABLE IF EXISTS media`).Exec(ctx)
		return err
	case _const.MySQLDriver, _const.MariaDBDriver:
		_, err := d.ORM.NewRaw(`
INSERT INTO files (id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_at, updated_at)
SELECT m.id,
  CASE WHEN m.media_type LIKE 'image/%' THEN 'media'
       WHEN m.media_type = 'application/pdf' THEN 'pdf'
       ELSE 'other' END,
  COALESCE(m.file_name, ''),
  m.file_extension,
  m.media_type,
  COALESCE(m.size, 0),
  COALESCE(m.s3_key, ''),
  m.url,
  CAST(m.created_at AS CHAR),
  CAST(m.created_at AS CHAR)
FROM media m
WHERE NOT EXISTS (SELECT 1 FROM files f WHERE f.id = m.id);`).Exec(ctx)
		if err != nil {
			return err
		}
		_, err = d.ORM.NewRaw(`DROP TABLE IF EXISTS media`).Exec(ctx)
		return err
	case _const.SQLiteDriver:
		_, err := d.ORM.NewRaw(`
INSERT INTO files (id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_at, updated_at)
SELECT m.id,
  CASE WHEN m.media_type LIKE 'image/%' THEN 'media'
       WHEN m.media_type = 'application/pdf' THEN 'pdf'
       ELSE 'other' END,
  COALESCE(m.file_name, ''),
  m.file_extension,
  m.media_type,
  COALESCE(m.size, 0),
  COALESCE(m.s3_key, ''),
  m.url,
  COALESCE(CAST(m.created_at AS TEXT), ''),
  COALESCE(CAST(m.created_at AS TEXT), '')
FROM media m
WHERE NOT EXISTS (SELECT 1 FROM files f WHERE f.id = m.id);`).Exec(ctx)
		if err != nil {
			return err
		}
		_, err = d.ORM.NewRaw(`DROP TABLE IF EXISTS media`).Exec(ctx)
		return err
	default:
		return nil
	}
}

func (d *Driver) legacyMediaTableExists(ctx context.Context) bool {
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		var n int
		if sch := strings.TrimSpace(d.DriverCredential.Schema); sch != "" {
			if err := d.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = ? AND table_name = 'media'`, sch).Scan(ctx, &n); err != nil {
				return false
			}
		} else if err := d.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'media'`).Scan(ctx, &n); err != nil {
			return false
		}
		return n > 0
	case _const.MySQLDriver, _const.MariaDBDriver:
		var n int
		if err := d.ORM.NewRaw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'media'`).Scan(ctx, &n); err != nil {
			return false
		}
		return n > 0
	case _const.SQLiteDriver:
		var n int
		if err := d.ORM.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='media'`).Scan(ctx, &n); err != nil {
			return false
		}
		return n > 0
	default:
		return false
	}
}

func (d *Driver) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	if err := d.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	_, err := d.ORM.NewInsert().Model(file).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (d *Driver) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, fmt.Errorf("file id is required")
	}
	file := &models.ProjectFile{}
	err := d.ORM.NewSelect().
		Model(file).
		Where("id = ?", fileID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (d *Driver) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	if err := d.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	fileType, limit, offset := models.SystemFileListParams(param)

	q := d.ORM.NewSelect().Model((*models.ProjectFile)(nil))
	if fileType != "" {
		q = q.Where("file_type = ?", fileType)
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	var files []*models.ProjectFile
	err = q.Order("created_at DESC").Limit(limit).Offset(offset).Scan(ctx, &files)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.ProjectFile]{
		Results: files,
		Total:   int64(total),
	}, nil
}

func (d *Driver) DeleteProjectFiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := d.ORM.NewDelete().
		Model((*models.ProjectFile)(nil)).
		Where("id IN (?)", ids).
		Exec(ctx)
	return err
}

func (d *Driver) SumProjectFilesSize(ctx context.Context) (int64, error) {
	if err := d.EnsureFilesTable(ctx); err != nil {
		return 0, err
	}
	var total int64
	err := d.ORM.NewRaw("SELECT COALESCE(SUM(size), 0) FROM files").Scan(ctx, &total)
	return total, err
}
