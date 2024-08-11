package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apito-io/buffers/plugins"
	svg "github.com/h2non/go-is-svg"
	"github.com/labstack/echo/v4"
	"gopkg.in/h2non/filetype.v1"
)

type HttpResponse struct {
	Message string      `json:"message,omitempty"`
	Body    interface{} `json:"body,omitempty"`
	Code    uint32      `json:"code,omitempty"`
	Token   string      `json:"token,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// UploadPhoto system Function Implementation
func (g *ApitoCDN) UploadPhoto(c echo.Context) error {

	if val := c.Get("project"); val != nil {
		projectId = val.(string)
	}

	fmt.Println("projectId:", projectId)

	// Multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}
	files := form.File["files"]

	var uploads []*plugins.FileDetails
	for _, file := range files {

		fileInfo, err := g.PrepareFileInfo(file, projectId)
		if err != nil {
			return c.JSON(http.StatusBadRequest, HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}

		random := RandomStringGenerator(10)
		key := fmt.Sprintf(`%s_%s.%s`, random, fileInfo.FileName, fileInfo.FileExtension)
		fileInfo.FileName = key
		fileInfo.RemoteFilePath = filepath.Join(s3Folder, projectId)

		fmt.Println("Uploading to S3")
		ctx := c.Request().Context()

		uploaded, err := g.UploadFile(ctx, fileInfo)
		if err != nil {
			return c.JSON(http.StatusBadRequest, HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}

		uploads = append(uploads, uploaded)
	}

	return c.JSON(200, HttpResponse{
		Code:    http.StatusOK,
		Body:    uploads,
		Message: "upload successful",
	})
}

func (g *ApitoCDN) PrepareFileInfo(file *multipart.FileHeader, projectID string) (*plugins.FileDetails, error) {

	buf := bytes.NewBuffer(nil)
	f, _ := file.Open()
	defer f.Close()

	if _, err := io.Copy(buf, f); err != nil {
		panic(err)
	}
	image := buf.Bytes()

	fileInfo, err := g.GatherFileInfo(image)
	if err != nil {
		return nil, err
	}
	var re = regexp.MustCompile(`[^a-zA-Z0-9]`)
	fileName := re.ReplaceAllString(strings.Split(file.Filename, ".")[0], `_$1`)
	fileInfo.FileName = fileName
	fileInfo.Size = file.Size

	fileInfo.UploadParam = &plugins.UploadParams{ProjectId: projectId}

	fileInfo.Buffer = buf.Bytes()

	return fileInfo, nil
}

func (g *ApitoCDN) PrepareFileInfoFromRouter(router echo.Context, projectID string) (*plugins.FileDetails, error) {
	file, err := router.FormFile("file")
	if err != nil {
		return nil, errors.New("no Upload File")
	}

	buf := bytes.NewBuffer(nil)
	f, _ := file.Open()
	defer f.Close()

	if _, err := io.Copy(buf, f); err != nil {
		panic(err)
	}
	image := buf.Bytes()

	fileInfo, err := g.GatherFileInfo(image)
	if err != nil {
		return nil, err
	}
	var re = regexp.MustCompile(`[^a-zA-Z0-9]`)
	fileName := re.ReplaceAllString(strings.Split(file.Filename, ".")[0], `_$1`)
	fileInfo.FileName = fileName
	fileInfo.Size = file.Size

	modelName := router.FormValue("model")
	fileInfo.UploadParam = &plugins.UploadParams{
		ModelName: modelName,
	}

	// get the id
	docId := router.FormValue("id")
	if docId != "" {
		fileInfo.UploadParam.DocId = docId
	}

	fieldName := router.FormValue("field_name")
	if fieldName != "" {
		fileInfo.UploadParam.FieldName = fieldName
	}

	provider := router.FormValue("provider")
	if provider != "" {
		fileInfo.UploadParam.Provider = provider
	}

	fileInfo.UploadParam.ProjectId = projectID

	fileInfo.Buffer = buf.Bytes()

	return fileInfo, nil
}

func (g *ApitoCDN) GatherFileInfo(image []byte) (*plugins.FileDetails, error) {
	fileInfo := plugins.FileDetails{}
	kind, unknown := filetype.Match(image)
	if unknown != nil {
		return nil, errors.New(fmt.Sprintf(`No Upload File1 %s`, unknown))
	}
	if kind.Extension == "unknown" {
		if svg.Is(image) {
			fileInfo.FileExtension = "svg"
			fileInfo.ContentType = "image/svg+xml"
		} else {
			fmt.Println("Unknown File Type")
		}
	} else {
		fileInfo.FileExtension = kind.Extension
		fileInfo.ContentType = kind.MIME.Value
	}
	return &fileInfo, nil
}
