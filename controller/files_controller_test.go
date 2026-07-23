package controller

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type mockUploader struct {
	uploaded []string
}

func (m *mockUploader) Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) (string, error) {
	m.uploaded = append(m.uploaded, key)
	return "https://cdn.example/" + key, nil
}

func (m *mockUploader) DeleteObjects(ctx context.Context, keys []string) ([]string, error) {
	return nil, nil
}

type mockProjectFilesDB struct {
	files []*models.ProjectFile
}

func (m *mockProjectFilesDB) EnsureFilesTable(context.Context) error { return nil }

func (m *mockProjectFilesDB) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	m.files = append(m.files, file)
	return file, nil
}

func (m *mockProjectFilesDB) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	for _, f := range m.files {
		if f.ID == fileID {
			return f, nil
		}
	}
	return nil, echo.ErrNotFound
}

func (m *mockProjectFilesDB) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	var out []*models.ProjectFile
	for _, f := range m.files {
		if f.ProjectID == param.ProjectID {
			out = append(out, f)
		}
	}
	return &models.SearchResponse[models.ProjectFile]{Results: out, Total: int64(len(out))}, nil
}

func (m *mockProjectFilesDB) SumProjectFilesSize(ctx context.Context) (int64, error) {
	var total int64
	for _, f := range m.files {
		total += f.Size
	}
	return total, nil
}

func (m *mockProjectFilesDB) DeleteProjectFiles(ctx context.Context, ids []string) error {
	next := m.files[:0]
	remove := map[string]struct{}{}
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	for _, f := range m.files {
		if _, ok := remove[f.ID]; ok {
			continue
		}
		next = append(next, f)
	}
	m.files = next
	return nil
}

type stubCacheProvider struct {
	cache *models.ApplicationCache
}

func (s *stubCacheProvider) GetApplicationCache(echo.Context) (*models.ApplicationCache, error) {
	return s.cache, nil
}

type stubProjectDriverResolver struct {
	store projectFilesStore
}

func (s *stubProjectDriverResolver) GetProjectFilesStore(context.Context) (projectFilesStore, error) {
	return s.store, nil
}

func TestFilesControllerUpload(t *testing.T) {
	e := echo.New()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "photo.png")
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	require.NoError(t, err)
	require.NoError(t, w.WriteField("file_type", models.SystemFileTypeMedia))
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/secured/files/upload", body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	db := &mockProjectFilesDB{}
	uploader := &mockUploader{}
	fc := &FilesController{
		Cfg: &models.Config{},
		cacheProvider: &stubCacheProvider{cache: &models.ApplicationCache{
			Ctx:     context.Background(),
			Project: &models.Project{ID: "proj-1"},
			Param:   &models.CommonSystemParams{UserID: "user-1"},
		}},
		driverResolver: &stubProjectDriverResolver{store: db},
		Storage: func(project *models.Project, cfg *models.Config) (ObjectUploader, error) {
			return uploader, nil
		},
	}

	require.NoError(t, fc.Upload(c))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, db.files, 1)
	require.Equal(t, models.SystemFileTypeMedia, db.files[0].FileType)
	require.Len(t, uploader.uploaded, 1)
	require.True(t, strings.HasSuffix(uploader.uploaded[0], ".png"))
}

func TestFilesControllerUploadBlobFilenameUsesMIMEExtension(t *testing.T) {
	e := echo.New()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="blob"`)
	h.Set("Content-Type", "application/octet-stream")
	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/secured/files/upload", body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	db := &mockProjectFilesDB{}
	uploader := &mockUploader{}
	fc := &FilesController{
		Cfg: &models.Config{},
		cacheProvider: &stubCacheProvider{cache: &models.ApplicationCache{
			Ctx:     context.Background(),
			Project: &models.Project{ID: "proj-1"},
		}},
		driverResolver: &stubProjectDriverResolver{store: db},
		Storage: func(project *models.Project, cfg *models.Config) (ObjectUploader, error) {
			return uploader, nil
		},
	}

	require.NoError(t, fc.Upload(c))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, db.files, 1)
	require.Equal(t, "png", db.files[0].FileExtension)
	require.Equal(t, "image/png", db.files[0].ContentType)
	require.Equal(t, "file", db.files[0].FileName)
	require.True(t, strings.HasSuffix(db.files[0].URL, ".png"))
	require.True(t, strings.HasSuffix(uploader.uploaded[0], "proj-1/media/"+db.files[0].ID+".png"))
}

func TestFilesControllerUploadSaaSRequiresTenant(t *testing.T) {
	e := echo.New()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "photo.png")
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	require.NoError(t, err)
	require.NoError(t, w.WriteField("file_type", models.SystemFileTypeMedia))
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/secured/files/upload", body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	fc := &FilesController{
		Cfg: &models.Config{},
		cacheProvider: &stubCacheProvider{cache: &models.ApplicationCache{
			Ctx:     context.Background(),
			Project: &models.Project{ID: "proj-1"},
			Param: &models.CommonSystemParams{
				UserID: "user-1",
				Ext:    map[string]interface{}{"pro_project_type": int32(1)},
			},
		}},
		driverResolver: &stubProjectDriverResolver{store: &mockProjectFilesDB{}},
		Storage: func(project *models.Project, cfg *models.Config) (ObjectUploader, error) {
			return &mockUploader{}, nil
		},
	}

	require.NoError(t, fc.Upload(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "tenant id is required")
}

func TestFilesControllerUploadSaaSUsesTenantKey(t *testing.T) {
	e := echo.New()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "photo.png")
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	require.NoError(t, err)
	require.NoError(t, w.WriteField("file_type", models.SystemFileTypeMedia))
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/secured/files/upload", body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("tenant_id", "01TENANT")

	db := &mockProjectFilesDB{}
	uploader := &mockUploader{}
	fc := &FilesController{
		Cfg: &models.Config{},
		cacheProvider: &stubCacheProvider{cache: &models.ApplicationCache{
			Ctx:     context.Background(),
			Project: &models.Project{ID: "proj-1"},
			Param: &models.CommonSystemParams{
				UserID: "user-1",
				Ext: map[string]interface{}{
					"pro_project_type": int32(1),
					"tenant_id":        "01TENANT",
				},
			},
		}},
		driverResolver: &stubProjectDriverResolver{store: db},
		Storage: func(project *models.Project, cfg *models.Config) (ObjectUploader, error) {
			return uploader, nil
		},
	}

	require.NoError(t, fc.Upload(c))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, db.files, 1)
	require.True(t, strings.HasSuffix(uploader.uploaded[0], "proj-1/01TENANT/media/"+db.files[0].ID+".png"))
}

func TestFilesControllerList(t *testing.T) {
	e := echo.New()
	db := &mockProjectFilesDB{
		files: []*models.ProjectFile{{
			ID: "f1", ProjectID: "proj-1", FileType: models.SystemFileTypeMedia,
			FileName: "a", URL: "https://cdn/a", Size: 1,
		}},
	}
	fc := &FilesController{
		cacheProvider: &stubCacheProvider{cache: &models.ApplicationCache{
			Ctx: context.Background(), Project: &models.Project{ID: "proj-1"},
		}},
		driverResolver: &stubProjectDriverResolver{store: db},
	}
	req := httptest.NewRequest(http.MethodGet, "/secured/files/list", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, fc.List(c))
	require.Equal(t, http.StatusOK, rec.Code)
}
