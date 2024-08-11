package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/apito-io/buffers/plugins"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"gopkg.in/h2non/filetype.v1"
)

func (s *ApitoCDN) GetPhotosAndCountInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return p, nil
}

func (s *ApitoCDN) GetPhotosInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	if val := router.Get("project"); val != nil {
		projectId = val.(string)
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	// 2. init the plugin
	files, err := s.ListFiles(p.Context, p.Args)
	if err != nil {
		return nil, err
	}

	//return s.Pixabay(p)
	return files, err
}

func (s *ApitoCDN) CountPhotosInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	if val := router.Get("project"); val != nil {
		projectId = val.(string)
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	// 2. init the plugin
	count, err := s.CountFiles(p.Context, p.Args)
	if err != nil {
		return nil, err
	}

	/*
		_param, err := s.buildCommonSystemParam(router)
		if err != nil {
			return nil, err
		}

		// forward the proxy
		p.Args = p.Source.(graphql.ResolveParams).Args

		param := s.NewParam(_param)
		param.ResolveParams = &p
		//return s.Pixabay(p)
		return s.GraphQLExecutor.GetProjectDriver(ctx).CountMedias(p.Context, param.ProjectId, &p)
	*/
	//return s.Pixabay(p)
	return count, err
}

func (s *ApitoCDN) UploadImageFromURLResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	if val := router.Get("project"); val != nil {
		projectId = val.(string)
	}

	var _url string
	if val, ok := p.Args["url"].(string); ok && val != "" {
		_url = val
	} else {
		return nil, errors.New("URL is Necessary")
	}
	m, err := s.HandleMediaURL(p.Context, map[string]interface{}{
		"url": _url,
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *ApitoCDN) DeleteMediaFileInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	if val := router.Get("project"); val != nil {
		projectId = val.(string)
	}

	var docIds []string
	if ids, ok := p.Args["ids"].([]interface{}); ok {
		for _, id := range ids {
			docIds = append(docIds, id.(string))
		}
	} else if len(docIds) == 0 {
		return nil, errors.New("id is required")
	} else {
		return nil, errors.New("invalid request")
	}

	var deletedIds []string
	for _, docId := range docIds {
		err := s.DeleteFile(ctx, docId)
		if err != nil {
			return nil, err
		}
		deletedIds = append(deletedIds, docId)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("%s file deleted", strings.Join(deletedIds, ",")),
	}, nil
}

func (s *ApitoCDN) UploadImageFromURL(ctx context.Context, projectId, modelName, imageUrl string) (*plugins.FileDetails, error) {

	res, err := http.Get(imageUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	kind, unknown := filetype.Match(data)
	if unknown != nil {
		fmt.Printf("Unknown: %s", unknown)
	}

	u, err := url.Parse(imageUrl)
	if err != nil {
		return nil, err
	}

	urlSplit := strings.Split(u.Path, "/")
	fileName := strings.TrimSuffix(urlSplit[len(urlSplit)-1], kind.Extension)

	random := RandomStringGenerator(10)
	var path string
	if modelName != "" {
		path = fmt.Sprintf(`%s/%s/%s_%s`, projectId, modelName, random, fileName)
	} else {
		path = fmt.Sprintf(`%s/%s_%s`, projectId, random, fileName)
	}

	key := fmt.Sprintf(`%s.%s`, path, kind.Extension)

	fmt.Println(int64(binary.Size(data)))

	fmt.Println("Uploading to S3 needs to be implemented")
	fmt.Println(key)
	/*err = s.S3.Upload(bytes.NewBuffer(data), key, kind.MIME.Value)
	if err != nil {
		fmt.Println(err.Error())
	}*/

	id := uuid.New()
	uid := id.String()

	// upload to server
	fileInfo := &plugins.FileDetails{
		Id:          uid,
		XKey:        uid,
		Type:        "picture",
		FileName:    fileName,
		ContentType: kind.MIME.Value,
		Url:         fmt.Sprintf(`%s/%s`, s3CdnURL, key),
		Size:        int64(binary.Size(data)),
		CreatedAt:   GetCurrentTime(),
		UploadParam: &plugins.UploadParams{
			ProjectId: projectId,
			ModelName: modelName,
		},
	}

	return fileInfo, nil
}

func (s *ApitoCDN) HandleMediaURL(ctx context.Context, media map[string]interface{}) (interface{}, error) {
	if imageUrl, ok := media["url"].(string); ok && imageUrl != "" && media["file_name"] == nil { // if it's a string then it's from user api
		// upload the picture from the url
		var modelName string

		uploadedImage, err := s.UploadImageFromURL(ctx, projectId, modelName, imageUrl)
		if err != nil {
			return nil, err
		}
		doc, err := s.CreateMediaDocument(ctx, uploadedImage)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}
	return media, nil
}
