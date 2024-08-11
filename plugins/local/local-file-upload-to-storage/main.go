package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/apito-io/buffers/plugins"
	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
)

type LocalCDN struct {
}

var mutex sync.RWMutex
var Db driver.Database
var EnvVars = map[string]interface{}{}
var projectId string
var host, port, user, password, database string

// Init system Function Implementation
func (g *LocalCDN) Init(envs []*plugins.EnvVariables) error {
	fmt.Println(fmt.Sprintf("Running Local Plugin Init %#v", envs))

	for _, env := range envs {
		switch env.Key {
		case "HOST":
			host = env.Value
		case "PORT":
			port = env.Value
		case "USER":
			user = env.Value
		case "PASSWORD":
			password = env.Value
		case "DATABASE":
			database = env.Value
		case "PROJECT_ID":
			projectId = env.Value
		}
	}

	if len(envs) > 0 {
		for _, v := range envs {
			mutex.Lock()
			EnvVars[v.Key] = v.Value
			mutex.Unlock()
		}
	}

	if Db == nil {
		fmt.Println(fmt.Sprintf("Connecting To DB"))
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

	return nil
}

// Migration system Function Implementation
func (g *LocalCDN) Migration() error {
	return nil
}

// SchemaRegister system Function Implementation
func (g *LocalCDN) SchemaRegister() (*plugins.ThirdPartyGraphQLSchemas, error) {
	return nil, nil
}

// RESTApiRegister system Function Implementation
func (g *LocalCDN) RESTApiRegister() ([]*plugins.ThirdPartyRESTApi, error) {
	return nil, nil
}

// Upload system Function Implementation
func (g *LocalCDN) Upload(_req map[string]interface{}) (interface{}, error) {

	fmt.Println(fmt.Sprintf(`Envs : %v`, EnvVars))

	var fileDetails plugins.FileDetails
	if val, ok := _req["file_info"].(*plugins.FileDetails); ok {
		fileDetails = *val
	}

	var buffer []byte
	if val, ok := _req["buffer"].([]byte); ok {
		buffer = val
	}

	fileName := fmt.Sprintf(`%s.%s`, fileDetails.FileName, fileDetails.FileExtension)

	fmt.Println(fileName)

	reader := bytes.NewReader(buffer)

	var uploadDIR string
	if val, ok := EnvVars["UPLOAD_DIR"].(string); ok && val != "" {
		uploadDIR = val
	}

	w, err := os.Create(filepath.Join(uploadDIR, fileName))
	if err != nil {
		return nil, err
	}
	defer w.Close()

	// do the actual work
	_, err = io.Copy(w, reader)
	if err != nil {
		return nil, err
	}

	var cdnURL string
	if val, ok := EnvVars["CDN_URL"].(string); ok && val != "" {
		cdnURL = val
	}

	// give back the url
	fileDetails.Url = filepath.Join(cdnURL, "static/media", fileName)

	return fileDetails, nil
}

// ListFiles list all the files that this plugin is serving
func (g *LocalCDN) ListFiles(filter map[string]interface{}) ([]*plugins.FileDetails, error) {

	fmt.Println(projectId, host, port, user, database, password)
	fmt.Println(filter)

	query, err := queryBuilder(projectId, filter)
	if err != nil {
		return nil, err
	}

	fmt.Println(query)

	ctx := context.Background()
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

// DeleteFile delete a single file
func (g *LocalCDN) DeleteFile(id string) error {
	return nil
}

func queryBuilder(projectId string, filter map[string]interface{}) (string, error) {

	limit := limitBuilder(filter)
	var models []string
	if val, ok := filter["models"].([]interface{}); ok {
		for _, v := range val {
			models = append(models, v.(string))
		}
	}

	var filters []string
	if len(models) > 0 {
		filters = append(filters, fmt.Sprintf(`x.upload_param.model_name IN ['%s']`, strings.Join(models, `','`)))
	}

	if val, ok := filter["search"].(string); ok {
		filters = append(filters, fmt.Sprintf(`LOWER(x.file_name) LIKE '%%%s%%'`, strings.ToLower(val)))
	}

	if ids, ok := filter["ids_in"].([]interface{}); ok {
		var mediaIds []string
		for _, id := range ids {
			mediaIds = append(mediaIds, id.(string))
		}
		filters = append(filters, fmt.Sprintf(`x.id IN ['%s']`, strings.Join(mediaIds, `','`)))
	}

	var _filter string
	if len(filters) > 0 {
		_filter = fmt.Sprintf(`FILTER %s`, strings.Join(filters, " AND "))
	}

	query := fmt.Sprintf("FOR x in `p_%s_media` %s SORT x.created_at DESC LIMIT %s RETURN x", projectId, _filter, limit)

	return query, nil
}

func limitBuilder(filter map[string]interface{}) string {

	limit := 10
	if val, ok := filter["limit"]; ok {
		limit = val.(int)
	}

	start := 0
	if val, ok := filter["start"]; ok {
		start = val.(int)
	}

	page := 1
	if val, ok := filter["page"]; ok {
		page = val.(int)
	}

	if page > 1 {
		offset := limit * (page - 1)
		return fmt.Sprintf(`%d, %d`, offset, limit)
	}

	return fmt.Sprintf(`%d, %d`, start, limit)
}

// LocalFileUpload because plugin Name is email-auth exported
// This exported Name is case-sensitive for the extension to load
var LocalFileUpload LocalCDN
