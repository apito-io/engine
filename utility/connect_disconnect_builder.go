package utility

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
)

func ConnectDisconnectParamBuilder(project *models.Project, uid string, connectionIds map[string]interface{}, modelType *models.ModelType) ([]*models.ConnectDisconnectParam, error) {

	projectID := project.ID

	var collectionName string
	if project.ProjectType == models.ProjectType_SaaS {
		collectionName = fmt.Sprintf("p_%s_%s", projectID, modelType.Name)
	} else {
		collectionName = fmt.Sprintf("p_%s", projectID)
	}

	var connParams []*models.ConnectDisconnectParam

	for k, v := range connectionIds {
		connParam := models.ConnectDisconnectParam{}
		connParam.DocCollectionName = collectionName
		connParam.DocRelationName = fmt.Sprintf(`p_%s_relation`, projectID)

		var ids []string
		var relationTo string
		if strings.HasSuffix(k, "_ids") {
			switch v.(type) {
			case []interface{}:
				if val, ok := v.([]interface{}); ok && len(val) > 0 {
					for _, id := range val {
						if id != nil {
							ids = append(ids, id.(string))
						}
					}
				}
				relationTo = strings.TrimSuffix(k, "_ids")
			default:
				return nil, errors.New(fmt.Sprintf("invalid Relation '%s' Input Type, Expected a List", k))
			}
		} else if strings.HasSuffix(k, "_id") {
			if v != nil {
				switch v.(type) {
				case string:
					ids = append(ids, v.(string))
				default:
					return nil, errors.New(fmt.Sprintf("invalid Relation '%s' Input Type, Expected a String", k))
				}
				relationTo = strings.TrimSuffix(k, "_id")
			}
		}

		if relationTo == "" { // skip if relationTo is not found then no need to build the connection
			continue
		}

		var knownAs string
		var connValCheck *models.ConnectionType
		// validate the relation to
		for _, c := range modelType.Connections {
			if c.Model == relationTo && c.KnownAs == "" {
				connValCheck = c
				knownAs = ""
				break
			} else if c.KnownAs == relationTo {
				connValCheck = c
				connParam.KnownAs = c.KnownAs
				relationTo = c.Model // restore the original name
				knownAs = c.KnownAs
				break
			}
		}

		if connValCheck != nil {
			switch connValCheck.Relation {
			case "has_many":
				if !strings.HasSuffix(k, "_ids") {
					return nil, errors.New("Has Many Relation doesnt support _id, try _ids instead")
				}
			case "has_one":
				if !strings.HasSuffix(k, "_id") {
					return nil, errors.New("Has one Relation doesnt support _ids, try _id instead")
				}
			}
		} else {
			return nil, errors.New(fmt.Sprintf("Invalid Relation %s", k))
		}

		// validate the relation to
		for _, connection := range modelType.Connections {
			if connection.Model == relationTo && connection.Type == "backward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionID = uid
				connParam.ActionIDs = ids

				connParam.ConnectionType = connection.Type
				connParam.BackwardConnectionType = connection
				if connection.Relation == "has_one" {

					var identifier string
					if knownAs != "" {
						identifier = fmt.Sprintf(`system_%s_as_%s_id`, connection.Model, knownAs)
					} else {
						identifier = fmt.Sprintf(`system_%s_id`, connection.Model)
					}

					ij := &models.InjectableHasOneConnection{
						ModelName: modelType.Name,
						IDs:       []string{uid},
						Data: map[string]string{
							identifier: ids[0],
						},
					}
					if connParam.InjectableHasOneConnects == nil {
						connParam.InjectableHasOneConnects = []*models.InjectableHasOneConnection{ij}
					} else {
						connParam.InjectableHasOneConnects = append(connParam.InjectableHasOneConnects, ij)
					}
				}

				// find forward
				var forwardModelType *models.ModelType
				for _, ct := range project.Schema.Models {
					if ct.Name == relationTo {
						forwardModelType = ct
						break
					}
				}

				for _, _connection := range forwardModelType.Connections {
					if _connection.Model == modelType.Name && _connection.Type == "forward" {
						connParam.BackwardConnectionModelType = forwardModelType
						connParam.ForwardConnectionType = _connection
						if _connection.Relation == "has_one" {

							var identifier string
							if knownAs != "" {
								identifier = fmt.Sprintf(`system_%s_as_%s_id`, _connection.Model, knownAs)
							} else {
								identifier = fmt.Sprintf(`system_%s_id`, _connection.Model)
							}

							ij := &models.InjectableHasOneConnection{
								ModelName: relationTo,
								IDs:       ids,
								Data: map[string]string{
									identifier: uid,
								},
							}
							if connParam.InjectableHasOneConnects == nil {
								connParam.InjectableHasOneConnects = []*models.InjectableHasOneConnection{ij}
							} else {
								connParam.InjectableHasOneConnects = append(connParam.InjectableHasOneConnects, ij)
							}
						}
						for _, ct := range project.Schema.Models {
							if ct.Name == _connection.Model {
								connParam.ForwardConnectionModelType = ct
								break
							}
						}
					}
				}
			} else if connection.Model == relationTo && connection.Type == "forward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionID = uid
				connParam.ActionIDs = ids

				connParam.ConnectionType = connection.Type
				connParam.ForwardConnectionType = connection
				if connection.Relation == "has_one" {

					var identifier string
					if knownAs != "" {
						identifier = fmt.Sprintf(`system_%s_as_%s_id`, connection.Model, knownAs)
					} else {
						identifier = fmt.Sprintf(`system_%s_id`, connection.Model)
					}

					ij := &models.InjectableHasOneConnection{
						ModelName: modelType.Name,
						IDs:       []string{uid},
						Data: map[string]string{
							identifier: ids[0],
						},
					}
					if connParam.InjectableHasOneConnects == nil {
						connParam.InjectableHasOneConnects = []*models.InjectableHasOneConnection{ij}
					} else {
						connParam.InjectableHasOneConnects = append(connParam.InjectableHasOneConnects, ij)
					}
				}
				// find forward
				var backwardModelType *models.ModelType
				for _, ct := range project.Schema.Models {
					if ct.Name == relationTo {
						backwardModelType = ct
						break
					}
				}
				for _, _connection := range backwardModelType.Connections {
					if _connection.Model == modelType.Name && _connection.Type == "backward" {
						connParam.BackwardConnectionModelType = backwardModelType
						connParam.BackwardConnectionType = _connection
						if _connection.Relation == "has_one" {

							var identifier string
							if knownAs != "" {
								identifier = fmt.Sprintf(`system_%s_as_%s_id`, _connection.Model, knownAs)
							} else {
								identifier = fmt.Sprintf(`system_%s_id`, _connection.Model)
							}

							ij := &models.InjectableHasOneConnection{
								ModelName: relationTo,
								IDs:       ids,
								Data: map[string]string{
									identifier: uid,
								},
							}
							if connParam.InjectableHasOneConnects == nil {
								connParam.InjectableHasOneConnects = []*models.InjectableHasOneConnection{ij}
							} else {
								connParam.InjectableHasOneConnects = append(connParam.InjectableHasOneConnects, ij)
							}
						}
						for _, ct := range project.Schema.Models {
							if ct.Name == _connection.Model {
								connParam.ForwardConnectionModelType = ct
								break
							}
						}
					}
				}
			}
		}

		connParams = append(connParams, &connParam)
	}

	return connParams, nil
}
