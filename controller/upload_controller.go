//go:build !cloudflare

package controller

/*
import (
	"bytes"
	"errors"
	"fmt"
	"github.com/apito-io/buffers/protobuff"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/resolver"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type uploadCtrl struct {
	cfg          *models.Config
	SQLConnPools *sync.Map
	graphQLServer     *resolver.GraphQLServer
}

// #todo Check Singleton
func GetUploadController(cfg *models.Config, graphQLServer *resolver.GraphQLServer) *uploadCtrl {
	return &uploadCtrl{
		cfg:      cfg,
		graphQLServer: graphQLServer,
		// Enforce Interface Implementation of UploadService
	}
}

func (u *uploadCtrl) MultiUpload(router echo.Context) error {

	cache, err := u.graphQLServer.GetApplicationCache(router)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	param := cache.Param

	if param.Role.Id == "demo" && param.Role.SystemGenerated {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "You Cant Change Anything in a Demo Project",
		})
	}

	files, err := u.prepareMultiFileInfo(router, param.ProjectId)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	var urls []string
	var uploadedFiles []*protobuff.FileDetails

	for _, file := range files {
		send, err := u.graphQLServer.UploadService.UploadFile(file, file.Buffer)
		if err != nil {
			return router.JSON(http.StatusBadRequest, models.HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}

		if send != nil {
			// get the id
			uploadedFiles = append(uploadedFiles, send)
			urls = append(urls, send.Url)
		}
	}

	if len(uploadedFiles) > 0 {
		docId := router.FormValue("id")
		if docId != "" {
			fieldName := router.FormValue("field_name")
			if fieldName == "" {
				return router.JSON(http.StatusBadRequest, models.HttpResponse{
					Code:    http.StatusBadRequest,
					Message: "Field Name Required if you are using id",
				})
			}

			doc, err := u.graphQLServer.GetProjectDriver().GetSingleProjectDocument(models.CommonSystemParams{
				ProjectId:  param.ProjectId,
				DocumentId: docId,
			})
			if err != nil {
				return router.JSON(http.StatusBadRequest, models.HttpResponse{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				})
			}

			if val, ok := doc.Data[fieldName].([]interface{}); ok {
				// update the field value
				for _, v := range val {
					urls = append(urls, v.(string))
				}
				doc.Data[fieldName] = urls
			} else {
				doc.Data[fieldName] = urls
			}

			err = u.graphQLServer.GetProjectDriver().UpdateDocumentOfProject(*param, doc, false)
			if err != nil {
				return router.JSON(http.StatusBadRequest, models.HttpResponse{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				})
			}
		}
	}

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		Body: uploadedFiles,
	})
}

func (u *uploadCtrl) DeletePictures(router echo.Context) error {

	cache, err := u.graphQLServer.GetApplicationCache(router)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	param := cache.Param

	if param.Role.Id == "demo" && param.Role.SystemGenerated {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "You Cant Change Anything in a Demo Project",
		})
	}

	var deleteRequest *protobuff.PictureDeleteRequest
	if err := router.Bind(&deleteRequest); err != nil {
		// if json bind error then 400 bad request
		return u.errorHandler(router, http.StatusBadRequest, "What the hell ?")
	}

	// remove files from the system
	var folderName string
	if deleteRequest.Model != "" {
		folderName = fmt.Sprintf("accounts/%s/%s", param.ProjectId, deleteRequest.Model)
	} else {
		return u.errorHandler(router, http.StatusBadRequest, "Model not Found ")
	}

	// delete files from s3
	err = u.graphQLServer.S3.DeleteFiles(deleteRequest.Urls, folderName)
	if err != nil {
		return u.errorHandler(router, http.StatusBadRequest, err.Error())
	}

	if deleteRequest.Id != "" {
		doc, err := u.graphQLServer.GetProjectDriver().GetSingleProjectDocument(models.CommonSystemParams{
			ProjectId:  param.ProjectId,
			DocumentId: deleteRequest.Id,
		})
		if err != nil {
			return router.JSON(http.StatusBadRequest, models.HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}

		if vals, ok := doc.Data[deleteRequest.FieldName].([]interface{}); ok {
			// update the field value
			for _, v := range deleteRequest.Urls {
				for i, url := range vals {
					if v == url {
						vals = append(vals[:i], vals[i+1:]...)
						break
					}
				}
			}
			doc.Data[deleteRequest.FieldName] = vals
		} else {
			return u.errorHandler(router, http.StatusBadRequest, "There is nothing to delete")
		}

		err = u.graphQLServer.GetProjectDriver().UpdateDocumentOfProject(*param, doc, false)
		if err != nil {
			return router.JSON(http.StatusBadRequest, models.HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}

	} else {
		return u.errorHandler(router, http.StatusBadRequest, "Id is required")
	}

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		Body: "Image Deleted",
	})
}

func (u *uploadCtrl) Upload(router echo.Context) error {

	cache, err := u.graphQLServer.GetApplicationCache(router)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	param := cache.Param

	if param.Role.Id == "demo" && param.Role.SystemGenerated {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "You Cant Change Anything in a Demo Project",
		})
	}

	fileInfo, buf, err := u.graphQLServer.PrepareFileInfo(router, param.ProjectId)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	pluginCache := u.graphQLServer.LocalPluginCache["local-file-upload-to-storage"]
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	pluginLookUp, err := pluginCache.Plugin.Lookup(pluginCache.PluginConfigurations.ExportedVariable)
	if err != nil {
		return err
	}

	pluginId := pluginCache.PluginConfigurations.Id

	// 3. Assert that loaded symbol is of a desired type
	// in this case interface type Greeter (defined above)
	var loadedPlugin pluginService.StoragePluginInterface
	loadedPlugin, ok := pluginLookUp.(pluginService.StoragePluginInterface)
	if !ok {
		return errors.New(fmt.Sprintf("%s plugin load failed", pluginId))
	}

	fmt.Println(fmt.Sprintf(`------ Loading %s Plugin -------`, pluginId))

	//send, err := u.gqlServer.UploadService.UploadFile(fileInfo, buf.Bytes())
	result, err := loadedPlugin.Upload(map[string]interface{}{
		"file_info": fileInfo,
		"buffer":    buf.Bytes(),
	})
	if err != nil {
		return errors.New(fmt.Sprintf("%s %s call failed", pluginId, err.Error()))
	}

	fileInfo = result.(*protobuff.FileDetails)

	err = u.graphQLServer.TrackUploadHistory(param, fileInfo)
	if err != nil {
		return router.JSON(http.StatusInternalServerError, &models.HttpResponse{
			//Message: captureInternalServerError(err).Error(),
			Code: http.StatusInternalServerError,
		})
	}

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		Body: fileInfo,
	})
}

func (u *uploadCtrl) PluginUpload(router echo.Context) error {

	req := make(map[string]interface{})
	if err := router.Bind(&req); err != nil {
		return router.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

		build a _plugin config from the req
		_pluginDetails := &protobuff.PluginDetails{
			Icon:             req["icon"].(string),
			Id:               req,
			Title:            "",
			Version:          "",
			Description:      "",
			Type:             0,
			Role:             "",
			EnvVars:          nil,
			ExportedVariable: "",
			Enable:           false,
			RepositoryUrl:    "",
			Branch:           "",
			Author:           "",
			LoadStatus:       0,
		}

		if val, ok := req["icon"].(string); ok {
			_pluginDetails.Icon = val
		}

		if val, ok := req["id"].(string); ok {
			_pluginDetails.Id = val
		}

		if val, ok := req["title"].(string); ok {
			_pluginDetails.Title = val
		}

		if val, ok := req["version"].(string); ok {
			_pluginDetails.Version = val
		}

		if val, ok := req["description"].(string); ok {
			_pluginDetails.Description = val
		}

		if val, ok := req["type"].(string); ok {
			_pluginDetails.Type = val
		}

		if val, ok := req["role"].(string); ok {
			_pluginDetails.Role = val
		}

		if val, ok := req["env"].(string); ok {
			_pluginDetails.Icon = val
		}

		if val, ok := req["icon"].(string); ok {
			_pluginDetails.Icon = val
		}

		if val, ok := req["icon"].(string); ok {
			_pluginDetails.Icon = val
		}


	var pluginID string
	if val, ok := req["id"].(string); ok {
		pluginID = val
	}

	cache, err := u.graphQLServer.GetApplicationCache(router)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	param := cache.Param

	if param.Role.Id == "demo" && param.Role.SystemGenerated {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "You Cant Change Anything in a Demo Project",
		})
	}

	_, buffer, err := u.graphQLServer.PrepareFileInfo(router, param.ProjectId)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	reader := bytes.NewReader(buffer.Bytes())

	dir := fmt.Sprintf(`plugins/local/%s`, pluginID)

	path := fmt.Sprintf(`%s/main.so`, dir)

	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	w, err := os.Create(path)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}
	defer w.Close()

	// do the actual work
	_, err = io.Copy(w, reader)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	_pluginDetails, err := u.graphQLServer.LoadLocalPlugin(cache.Project.Schema, dir)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		Body: _pluginDetails,
	})
}

func (u *uploadCtrl) prepareMultiFileInfo(router echo.Context, projectID string) ([]*protobuff.FileDetails, error) {

	// Multipart form
	form, _ := router.MultipartForm()
	files := form.File["files[]"]

	var uploadedFiles []*protobuff.FileDetails

	for _, file := range files {
		buf := bytes.NewBuffer(nil)
		f, _ := file.Open()
		if _, err := io.Copy(buf, f); err != nil {
			return nil, err
		}
		image := buf.Bytes()

		fileInfo, err := u.graphQLServer.UploadService.GatherFileInfo(image)
		if err != nil {
			return nil, err
		}
		fileInfo.Buffer = image
		var re = regexp.MustCompile(`[^a-zA-Z0-9]`)
		fileName := re.ReplaceAllString(strings.Split(file.Filename, ".")[0], `_$1`)
		fileInfo.FileName = fileName
		fileInfo.Size_ = file.Size

		modelName := router.FormValue("model")
		fileInfo.UploadParam = &protobuff.UploadParams{
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
		fileInfo.UploadParam.ProjectId = projectID

		uploadedFiles = append(uploadedFiles, fileInfo)
		f.Close()
	}
	return uploadedFiles, nil
}

// This is front end validation error
func (a *uploadCtrl) bindJSON(router echo.Context) {
	router.JSON(http.StatusBadRequest, gin.H{
		"code": http.StatusBadRequest,
		//"message": fmt.Sprintf("ApiGateway :: AuthController :: %s :: JsonBind :: %s", msg, err.Error()),
		"message": "Invalid JSON",
	})
}

func (a *uploadCtrl) errorHandler(router echo.Context, code int, msg string) error {
	return router.JSON(code, &models.HttpResponse{
		Code:    uint32(code),
		Message: msg,
	})
}
*/
