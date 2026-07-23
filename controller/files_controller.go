package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/services/storage"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
)

const defaultMaxUploadBytes = 50 << 20 // 50 MiB

// ObjectUploader uploads and deletes blobs in project storage (injectable for tests).
type ObjectUploader interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) (publicURL string, err error)
	DeleteObjects(ctx context.Context, keys []string) (failed []string, err error)
}

type applicationCacheProvider interface {
	GetApplicationCache(echo.Context) (*models.ApplicationCache, error)
}

type projectFilesStore interface {
	EnsureFilesTable(ctx context.Context) error
	CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error)
	GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error)
	SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error)
	DeleteProjectFiles(ctx context.Context, ids []string) error
	SumProjectFilesSize(ctx context.Context) (int64, error)
}

type projectDriverResolver interface {
	GetProjectFilesStore(ctx context.Context) (projectFilesStore, error)
}

type graphqlProjectFilesResolver struct {
	exec interface {
		GetProjectDriver(ctx context.Context) (interfaces.ProjectDBInterface, error)
	}
}

func (g *graphqlProjectFilesResolver) GetProjectFilesStore(ctx context.Context) (projectFilesStore, error) {
	drv, err := g.exec.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}
	store, ok := drv.(projectFilesStore)
	if !ok {
		return nil, ae.ErrProjectFilesUnsupported
	}
	return store, nil
}

// FilesController handles /secured/files REST endpoints.
type FilesController struct {
	Cfg           *models.Config
	cacheProvider applicationCacheProvider
	driverResolver projectDriverResolver
	Storage       func(project *models.Project, cfg *models.Config) (ObjectUploader, error)
}

// NewFilesController creates a FilesController with default S3 storage factory.
func NewFilesController(cfg *models.Config, server *resolver.GraphQLServer) *FilesController {
	return &FilesController{
		Cfg:           cfg,
		cacheProvider: server,
		driverResolver: &graphqlProjectFilesResolver{exec: server.GraphQLExecutor},
		Storage: func(project *models.Project, cfg *models.Config) (ObjectUploader, error) {
			return storage.NewProjectS3Storage(project, cfg)
		},
	}
}

type systemFileJSON struct {
	ID            string `json:"id"`
	FileType      string `json:"file_type"`
	FileName      string `json:"file_name"`
	FileExtension string `json:"file_extension,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Size          int64  `json:"size"`
	URL           string `json:"url"`
	CreatedBy     string `json:"created_by,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

func toSystemFileJSON(f *models.ProjectFile) systemFileJSON {
	if f == nil {
		return systemFileJSON{}
	}
	return systemFileJSON{
		ID:            f.ID,
		FileType:      f.FileType,
		FileName:      f.FileName,
		FileExtension: f.FileExtension,
		ContentType:   f.ContentType,
		Size:          f.Size,
		URL:           f.URL,
		CreatedBy:     f.CreatedBy,
		CreatedAt:     f.CreatedAt,
	}
}

func (fc *FilesController) isDemoReadOnly(cache *models.ApplicationCache) bool {
	if cache == nil || cache.Param == nil || cache.Param.Role == nil {
		return false
	}
	return cache.Param.Role.ID == "demo" && cache.Param.Role.SystemGenerated
}

func (fc *FilesController) jsonError(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]interface{}{
		"success": false,
		"message": msg,
	})
}

func (fc *FilesController) projectFilesStore(cache *models.ApplicationCache, c echo.Context) (projectFilesStore, context.Context, error) {
	if cache == nil || cache.Project == nil {
		return nil, nil, errors.New("project is required")
	}
	if fc.driverResolver == nil {
		return nil, nil, errors.New("database service unavailable")
	}
	dbCtx := resolver.PublicProjectDBContext(cache, c.Request().Context())
	store, err := fc.driverResolver.GetProjectFilesStore(dbCtx)
	if err != nil {
		return nil, nil, err
	}
	return store, dbCtx, nil
}

// Upload handles POST /secured/files/upload (multipart field "file", optional "file_type").
func (fc *FilesController) Upload(c echo.Context) error {
	cache, err := fc.cacheProvider.GetApplicationCache(c)
	if err != nil {
		return fc.jsonError(c, http.StatusUnauthorized, err.Error())
	}
	if fc.isDemoReadOnly(cache) {
		return fc.jsonError(c, http.StatusForbidden, ae.NotAllowed.Error())
	}

	store, dbCtx, err := fc.projectFilesStore(cache, c)
	if err != nil {
		if errors.Is(err, ae.ErrProjectFilesUnsupported) {
			return fc.jsonError(c, http.StatusBadRequest, err.Error())
		}
		return fc.jsonError(c, http.StatusServiceUnavailable, err.Error())
	}

	upload, err := c.FormFile("file")
	if err != nil || upload == nil {
		return fc.jsonError(c, http.StatusBadRequest, "file is required")
	}
	if upload.Size > defaultMaxUploadBytes {
		return fc.jsonError(c, http.StatusBadRequest, "file exceeds maximum upload size")
	}

	if models.UseFreeCloudStorageEffective(cache.Project) {
		currentBytes, err := store.SumProjectFilesSize(dbCtx)
		if err != nil {
			return fc.jsonError(c, http.StatusInternalServerError, "could not verify storage usage")
		}
		limitBytes := models.FreeCloudStorageLimitBytes(fc.Cfg)
		if currentBytes+upload.Size > limitBytes {
			return fc.jsonError(c, http.StatusForbidden, ae.MEDIA_STORAGE_LIMIT_REACHED)
		}
	}

	fileType := strings.TrimSpace(c.FormValue("file_type"))
	src, err := upload.Open()
	if err != nil {
		return fc.jsonError(c, http.StatusBadRequest, "could not read uploaded file")
	}
	defer src.Close()

	body, err := io.ReadAll(src)
	if err != nil {
		return fc.jsonError(c, http.StatusBadRequest, "could not read uploaded file")
	}

	contentType, ext := models.ResolveUploadMIME(upload.Filename, upload.Header.Get("Content-Type"), body)
	if fileType == "" {
		fileType = models.InferFileTypeFromMIME(contentType)
	}
	if err := models.ValidateFileType(fileType); err != nil {
		return fc.jsonError(c, http.StatusBadRequest, err.Error())
	}

	tenantID := resolveUploadTenantID(c, cache)
	if isSaaSProjectParam(cache.Param) {
		if tenantID == "" {
			return fc.jsonError(c, http.StatusBadRequest, "tenant id is required for SaaS file upload")
		}
	} else {
		// General projects never embed a tenant segment in the object key.
		tenantID = ""
	}

	fileID := utility.NewID()
	storageKey, err := storage.BuildObjectKey(cache.Project.ID, tenantID, fileType, fileID, ext)
	if err != nil {
		return fc.jsonError(c, http.StatusBadRequest, err.Error())
	}

	objStore, err := fc.Storage(cache.Project, fc.Cfg)
	if err != nil {
		return fc.jsonError(c, http.StatusBadRequest, err.Error())
	}

	publicURL, err := objStore.Upload(dbCtx, storageKey, bytes.NewReader(body), contentType, upload.Size)
	if err != nil {
		return fc.jsonError(c, http.StatusInternalServerError, "upload to storage failed")
	}

	now := utility.GetCurrentTime()
	userID := ""
	if cache.Param != nil {
		userID = cache.Param.UserID
	}
	baseName := strings.TrimSuffix(filepath.Base(upload.Filename), filepath.Ext(upload.Filename))
	baseName = sanitizeFileBaseName(baseName)
	if models.IsGenericUploadBaseName(baseName) && ext != "" {
		baseName = "file"
	}

	record := &models.ProjectFile{
		ID:            fileID,
		ProjectID:     cache.Project.ID,
		FileType:      fileType,
		FileName:      baseName,
		FileExtension: ext,
		ContentType:   contentType,
		Size:          upload.Size,
		StorageKey:    storageKey,
		URL:           publicURL,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	saved, err := store.CreateProjectFile(dbCtx, record)
	if err != nil {
		_, _ = objStore.DeleteObjects(dbCtx, []string{storageKey})
		return fc.jsonError(c, http.StatusInternalServerError, "failed to save file metadata")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"file":    toSystemFileJSON(saved),
	})
}

// List handles GET /secured/files/list.
func (fc *FilesController) List(c echo.Context) error {
	cache, err := fc.cacheProvider.GetApplicationCache(c)
	if err != nil {
		return fc.jsonError(c, http.StatusUnauthorized, err.Error())
	}

	store, dbCtx, err := fc.projectFilesStore(cache, c)
	if err != nil {
		if errors.Is(err, ae.ErrProjectFilesUnsupported) {
			return fc.jsonError(c, http.StatusBadRequest, err.Error())
		}
		return fc.jsonError(c, http.StatusServiceUnavailable, err.Error())
	}

	limit := 50
	if v := c.QueryParam("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := c.QueryParam("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	fileType := strings.TrimSpace(c.QueryParam("file_type"))
	if fileType != "" {
		if err := models.ValidateFileType(fileType); err != nil {
			return fc.jsonError(c, http.StatusBadRequest, err.Error())
		}
	}

	param := &models.CommonSystemParams{
		ProjectID: cache.Project.ID,
		Ext: map[string]interface{}{
			models.ExtKeySystemFileType:    fileType,
			models.ExtKeySystemFilesLimit:  limit,
			models.ExtKeySystemFilesOffset: offset,
		},
	}

	resp, err := store.SearchProjectFiles(dbCtx, param)
	if err != nil {
		return fc.jsonError(c, http.StatusInternalServerError, err.Error())
	}

	files := make([]systemFileJSON, 0, len(resp.Results))
	for _, f := range resp.Results {
		files = append(files, toSystemFileJSON(f))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   files,
		"total":   resp.Total,
	})
}

type deleteFilesRequest struct {
	IDs []string `json:"ids"`
}

// Delete handles POST /secured/files/delete.
func (fc *FilesController) Delete(c echo.Context) error {
	cache, err := fc.cacheProvider.GetApplicationCache(c)
	if err != nil {
		return fc.jsonError(c, http.StatusUnauthorized, err.Error())
	}
	if fc.isDemoReadOnly(cache) {
		return fc.jsonError(c, http.StatusForbidden, ae.NotAllowed.Error())
	}

	store, dbCtx, err := fc.projectFilesStore(cache, c)
	if err != nil {
		if errors.Is(err, ae.ErrProjectFilesUnsupported) {
			return fc.jsonError(c, http.StatusBadRequest, err.Error())
		}
		return fc.jsonError(c, http.StatusServiceUnavailable, err.Error())
	}

	var body deleteFilesRequest
	if err := c.Bind(&body); err != nil {
		return fc.jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if len(body.IDs) == 0 {
		return fc.jsonError(c, http.StatusBadRequest, "ids are required")
	}

	objStore, err := fc.Storage(cache.Project, fc.Cfg)
	if err != nil {
		return fc.jsonError(c, http.StatusBadRequest, err.Error())
	}

	var keys []string
	var foundIDs []string
	var storageFailed []string

	for _, id := range body.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		file, err := store.GetProjectFile(dbCtx, id)
		if err != nil {
			continue
		}
		keys = append(keys, file.StorageKey)
		foundIDs = append(foundIDs, id)
	}

	if len(foundIDs) == 0 {
		return fc.jsonError(c, http.StatusBadRequest, "no matching files found for this project")
	}

	failedKeys, err := objStore.DeleteObjects(dbCtx, keys)
	if err != nil {
		storageFailed = failedKeys
	}

	if err := store.DeleteProjectFiles(dbCtx, foundIDs); err != nil {
		return fc.jsonError(c, http.StatusInternalServerError, "failed to delete file records")
	}

	out := map[string]interface{}{
		"success":        true,
		"deleted_ids":    foundIDs,
		"storage_failed": storageFailed,
	}
	if len(storageFailed) > 0 {
		out["success"] = false
		out["message"] = "some objects could not be removed from storage"
	}
	return c.JSON(http.StatusOK, out)
}

func sanitizeFileBaseName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		return "file"
	}
	return s
}

// resolveUploadTenantID reads tenant context from echo (token/middleware) then param Ext.
func resolveUploadTenantID(c echo.Context, cache *models.ApplicationCache) string {
	if c != nil {
		if v, ok := c.Get("tenant_id").(string); ok {
			if tid := strings.TrimSpace(v); tid != "" {
				return tid
			}
		}
	}
	if cache != nil && cache.Param != nil && cache.Param.Ext != nil {
		switch v := cache.Param.Ext["tenant_id"].(type) {
		case string:
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// isSaaSProjectParam mirrors pro ParamIsSaaSProject without importing pro packages.
// pro_project_type is set by SetParamProProjectType (1 = SaaS).
func isSaaSProjectParam(p *models.CommonSystemParams) bool {
	if p == nil || p.Ext == nil {
		return false
	}
	switch v := p.Ext["pro_project_type"].(type) {
	case int32:
		return v == 1
	case int:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return int(v) == 1
	default:
		return false
	}
}
