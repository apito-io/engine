package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/buffers/plugins"
	"github.com/google/uuid"
)

func (a *ApitoCDN) CreateMediaDocument(ctx context.Context, media *plugins.FileDetails) (*plugins.FileDetails, error) {

	// add meta info
	if media.Id == "" {
		media.Id = uuid.NewString()
		media.XKey = media.Id
		media.Type = "picture"
	}

	media.CreatedAt = GetCurrentTime()

	projectMediaCollection := fmt.Sprintf(`p_%s_media`, projectId)
	// create an empty project media collection
	mediaCollection, err := Db.Collection(ctx, projectMediaCollection)
	if err != nil {
		return nil, err
	}

	_, err = mediaCollection.CreateDocument(ctx, media)
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (a *ApitoCDN) DeleteMediaDocument(ctx context.Context, media *plugins.FileDetails) error {

	if media.Id == "" {
		return errors.New("media id is required")
	}

	projectMediaCollection := fmt.Sprintf(`p_%s_media`, projectId)
	// create an empty project media collection
	mediaCollection, err := Db.Collection(ctx, projectMediaCollection)
	if err != nil {
		return err
	}

	_, err = mediaCollection.RemoveDocument(ctx, media.XKey)
	if err != nil {
		return err
	}

	return nil
}

func queryBuilder(projectId string, filter map[string]interface{}, count bool) (string, error) {

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

	if val, ok := filter["ids_in"]; ok && val != nil {
		switch val.(type) {
		case []interface{}:
			if ids, ok := filter["ids_in"].([]interface{}); ok {
				var mediaIds []string
				for _, id := range ids {
					mediaIds = append(mediaIds, id.(string))
				}
				filters = append(filters, fmt.Sprintf(`x.id IN ['%s']`, strings.Join(mediaIds, `','`)))
			}
		case []string:
			if ids, ok := filter["ids_in"].([]string); ok {
				var mediaIds []string
				for _, id := range ids {
					mediaIds = append(mediaIds, id)
				}
				filters = append(filters, fmt.Sprintf(`x.id IN ['%s']`, strings.Join(mediaIds, `','`)))
			}
		default:
			return "", errors.New("invalid filter type")
		}
	}

	var _filter string
	if len(filters) > 0 {
		_filter = fmt.Sprintf(`FILTER %s`, strings.Join(filters, " AND "))
	}

	var query string
	if count {
		query = fmt.Sprintf("RETURN COUNT(FOR x in `p_%s_media` %s RETURN x.id)", projectId, _filter)
	} else {
		query = fmt.Sprintf("FOR x in `p_%s_media` %s SORT x.created_at DESC LIMIT %s RETURN x", projectId, _filter, limit)
	}

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
