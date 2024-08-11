package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"

	"github.com/apito-io/buffers/plugins"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/utility"
	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/tailor-inc/graphql"
)

type ApitoCDN struct{}

var mutex sync.RWMutex
var Db driver.Database
var S3Session *session.Session
var EnvVars = map[string]interface{}{}
var projectId string
var host, port, user, password, database string
var s3AccessKey, s3SecretKey, s3CdnURL, s3Bucket, s3Folder, s3Region string

var FileDetailsType, _ = utility.GetGraphQLObject(protobuff.FileDetails{})

// Init system Function Implementation
func (g *ApitoCDN) Init(envs []*plugins.EnvVariables) error {
	fmt.Println(fmt.Sprintf("Running ApitoCDN Plugin Init %#v", envs))

	for _, env := range envs {
		switch env.Key {
		case "DB_HOST":
			host = env.Value
		case "DB_PORT":
			port = env.Value
		case "DB_USER":
			user = env.Value
		case "DB_PASSWORD":
			password = env.Value
		case "DB_DATABASE":
			database = env.Value
		case "PROJECT_ID":
			projectId = env.Value
		case "S3_CDN_URL":
			s3CdnURL = env.Value
		case "S3_FOLDER":
			s3Folder = env.Value
		case "S3_REGION":
			s3Region = env.Value
		case "S3_ACCESS_KEY":
			s3AccessKey = env.Value
		case "S3_SECRET_KEY":
			s3SecretKey = env.Value
		case "S3_BUCKET_NAME":
			s3Bucket = env.Value
		}
	}

	if len(envs) > 0 {
		for _, v := range envs {
			mutex.Lock()
			EnvVars[v.Key] = v.Value
			mutex.Unlock()
		}
	}

	if Db == nil && host != "" && port != "" && user != "" && password != "" && database != "" {
		fmt.Println(fmt.Sprintf("ApitoCDN Connecting To DB"))
		conn, err := http.NewConnection(http.ConnectionConfig{
			Endpoints: []string{fmt.Sprintf("%s:%s", host, port)},
			TLSConfig: &tls.Config{ /*...*/ },
		})
		if err != nil {
			return err
		}

		c, err := driver.NewClient(driver.ClientConfig{
			Connection:     conn,
			Authentication: driver.BasicAuthentication(user, password),
		})
		if err != nil {
			return err
		}

		ctx := context.Background()
		db, err := c.Database(ctx, database)
		if err != nil {
			return err
		}

		Db = db
	}

	if S3Session == nil {
		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(s3Region),
			Credentials: credentials.NewStaticCredentials(s3AccessKey, s3SecretKey, ""),
		})
		if err != nil {
			return err
		}
		S3Session = sess
	}

	return nil
}

// Migration system Function Implementation
func (g *ApitoCDN) Migration(ctx context.Context) error {
	return nil
}

// SchemaRegister system Function Implementation
func (g *ApitoCDN) SchemaRegister(ctx context.Context) (*plugins.ThirdPartyGraphQLSchemas, error) {

	queries := graphql.Fields{}
	mutations := graphql.Fields{}

	fmt.Println(fmt.Sprintf("ApitoCDN registering getPhotos query"))
	queries["getPhotos"] = &graphql.Field{
		Name: "ListAllDataOfAMedia",
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "GetAllPhotosResponse",
			Fields: graphql.Fields{
				"results": &graphql.Field{
					Type:    graphql.NewList(FileDetailsType),
					Resolve: g.GetPhotosInfoResolverFn,
				},
				"count": &graphql.Field{
					Type:    graphql.Int,
					Resolve: g.CountPhotosInfoResolverFn,
				},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"models": &graphql.ArgumentConfig{
				Type: graphql.NewList(graphql.String),
			},
			"search": &graphql.ArgumentConfig{
				Type: graphql.String,
			},
			"page": &graphql.ArgumentConfig{
				Type: graphql.Int,
			},
			"limit": &graphql.ArgumentConfig{
				Type: graphql.Int,
			},
			"ids_in": &graphql.ArgumentConfig{
				Type: graphql.NewList(graphql.String),
			},
		},
		Resolve: g.GetPhotosAndCountInfoResolverFn,
	}

	mutations["uploadImageFromUrl"] = &graphql.Field{
		Name: "UploadImageFromURL",
		Type: FileDetailsType,
		Args: graphql.FieldConfigArgument{
			"url": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
		},
		Resolve: g.UploadImageFromURLResolverFn,
	}

	fmt.Println(fmt.Sprintf("registering deleteMediaFile query"))
	mutations["deleteMediaFile"] = &graphql.Field{
		Name: "DeleteMediaFile",
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "DeleteMediaFileResponse",
			Fields: graphql.Fields{
				"message": &graphql.Field{Type: graphql.String},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"ids": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
			},
		},
		Resolve: g.DeleteMediaFileInfoResolverFn,
	}

	return &plugins.ThirdPartyGraphQLSchemas{
		Queries:   queries,
		Mutations: mutations,
	}, nil
}

// RESTApiRegister system Function Implementation
func (g *ApitoCDN) RESTApiRegister(ctx context.Context) ([]*plugins.ThirdPartyRESTApi, error) {
	fmt.Println(fmt.Sprintf("ApitoCDN registering upload REST Api"))

	return []*plugins.ThirdPartyRESTApi{
		{
			Path:       "/media/upload",
			Method:     "POST",
			Controller: g.UploadPhoto,
		},
	}, nil
}

// UploadFile system Function Implementation
func (g *ApitoCDN) UploadFile(ctx context.Context, file *plugins.FileDetails) (*plugins.FileDetails, error) {

	fullPath, err := g.S3Upload(ctx, file)
	if err != nil {
		return nil, err
	}

	fmt.Println("uploaded file ...")
	fmt.Println(fullPath)
	file.UploadedFullPath = fullPath
	file.Url = file.UploadedFullPath
	file.Buffer = nil // just make it null for response purposes

	_, err = g.CreateMediaDocument(ctx, file)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// ListFiles list all the files that this plugin is serving
func (g *ApitoCDN) ListFiles(ctx context.Context, filter map[string]interface{}) ([]*plugins.FileDetails, error) {

	fmt.Println(projectId, host, port, user, database, password)
	fmt.Println(filter)

	query, err := queryBuilder(projectId, filter, false)
	if err != nil {
		return nil, err
	}

	fmt.Println(query)

	cursor, err := Db.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	var docs []*plugins.FileDetails
	for {
		var doc plugins.FileDetails
		_, err := cursor.ReadDocument(nil, &doc)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, err
		}
		docs = append(docs, &doc)
	}

	return docs, nil
}

// CountFiles list all the files that this plugin is serving
func (g *ApitoCDN) CountFiles(ctx context.Context, filter map[string]interface{}) (int64, error) {

	fmt.Println(projectId, host, port, user, database, password)
	fmt.Println(filter)

	query, err := queryBuilder(projectId, filter, true)
	if err != nil {
		return -1, err
	}

	fmt.Println(query)

	cursor, err := Db.Query(ctx, query, nil)
	if err != nil {
		return -1, err
	}
	defer cursor.Close()

	var doc int64
	for {
		_, err := cursor.ReadDocument(ctx, &doc)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return -1, err
		}
	}

	return doc, nil
}

// DeleteFile delete a single file
func (g *ApitoCDN) DeleteFile(ctx context.Context, id string) error {

	// this is dangerous | replace with single file method in future
	medias, err := g.ListFiles(ctx, map[string]interface{}{
		"ids_in": []string{id},
	})
	if err != nil {
		return err
	}

	if len(medias) == 0 {
		return errors.New("nothing found to delete")
	}

	folderName := fmt.Sprintf("accounts/%s", projectId)

	// delete files from s3
	err = g.S3DeleteFiles(medias, folderName)
	if err != nil {
		return err
	}

	for _, media := range medias {
		err = g.DeleteMediaDocument(ctx, media)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	return nil

	/*projectUsages, err := s.SystemDriver.GetProjectUsages(p.Context, param.ProjectId, nil)
	if err != nil {
		return nil, err
	}

	projectUsages.Usages.MediaStorage = projectUsages.Usages.MediaStorage - sizeToDelete
	projectUsages.Usages.MediaBandwidth = projectUsages.Usages.MediaBandwidth + sizeToDelete
	projectUsages.Usages.ApiCalls++ // delete media is also count as an api call

	err = s.SystemDriver.UpdateProjectUsagesDoc(p.Context, projectUsages, true)
	if err != nil {
		return nil, err
	}*/
}

// ApitoCDNUpload because plugin Name is email-auth exported
// This exported Name is case-sensitive for the extension to load
var ApitoCDNUpload ApitoCDN
