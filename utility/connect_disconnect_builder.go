package utility

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
)

func ConnectDisconnectParamBuilder(project *models.Project, uid string, connectionIds map[string]interface{}, modelType *models.ModelType) ([]*models.ConnectDisconnectParam, error) {

	projectID := project.ID

	collectionName := fmt.Sprintf("p_%s", projectID)

	var connParams []*models.ConnectDisconnectParam

	for k, v := range connectionIds {
		connParam := models.ConnectDisconnectParam{}
		connParam.DocCollectionName = collectionName
		connParam.DocRelationName = fmt.Sprintf(`p_%s_relation`, projectID)

		var ids []string
		var relationTo string
		if strings.HasSuffix(k, "_ids") {
			val, ok := v.([]interface{})
			if !ok {
				return nil, errors.New(fmt.Sprintf("invalid Relation '%s' Input Type, Expected a List but got %T", k, v))
			}
			for _, id := range val {
				if sid, ok := id.(string); ok && sid != "" {
					ids = append(ids, sid)
				}
			}
			relationTo = strings.TrimSuffix(k, "_ids")
		} else if strings.HasSuffix(k, "_id") {
			if sid, ok := v.(string); ok && sid != "" {
				ids = append(ids, sid)
				relationTo = strings.TrimSuffix(k, "_id")
			} else if v != nil {
				return nil, errors.New(fmt.Sprintf("invalid Relation '%s' Input Type, Expected a String but got empty string", k))
			}
		}

		if len(ids) == 0 {
			return nil, errors.New(fmt.Sprintf("Relation '%s' you are trying to connect with has no ids", k))
		}

		if relationTo == "" { // skip if relationTo is not found then no need to build the connection
			return nil, errors.New(fmt.Sprintf("Relation you are trying to connect with '%s' not found", k))
		}

		var knownAs string
		var connValCheck *models.ConnectionType
		// validate the relation to
		for _, c := range modelType.Connections {
			if c == nil {
				continue
			}
			modelMatches := c.Model == relationTo || ModelIDMatchesGraphQLField(c.Model, relationTo)
			if modelMatches && c.KnownAs == "" {
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
			if connection == nil {
				continue
			}
			modelMatches := connection.Model == relationTo ||
				ModelIDMatchesGraphQLField(connection.Model, relationTo)
			if modelMatches && connection.Type == "backward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionID = uid
				connParam.ActionIDs = ids

				connParam.ConnectionType = connection.Type
				connParam.BackwardConnectionType = connection
				if connection.Relation == "has_one" {

					identifier := SyntheticSystemRelationFieldIdentifier(connection.Model, knownAs)

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
					if ct.Name == relationTo || ModelIDMatchesGraphQLField(ct.Name, relationTo) {
						forwardModelType = ct
						break
					}
				}

				if forwardModelType != nil {
					for _, _connection := range forwardModelType.Connections {
						if _connection == nil {
							continue
						}
						peerMatches := _connection.Model == modelType.Name ||
							ModelIDMatchesGraphQLField(_connection.Model, modelType.Name)
						if peerMatches && _connection.Type == "forward" && _connection.KnownAs == knownAs {
							connParam.BackwardConnectionModelType = forwardModelType
							connParam.ForwardConnectionType = _connection
							if _connection.Relation == "has_one" {

								identifier := SyntheticSystemRelationFieldIdentifier(_connection.Model, knownAs)

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
								if ct.Name == _connection.Model || ModelIDMatchesGraphQLField(ct.Name, _connection.Model) {
									connParam.ForwardConnectionModelType = ct
									break
								}
							}
							break
						}
					}
				}
			} else if modelMatches && connection.Type == "forward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionID = uid
				connParam.ActionIDs = ids

				connParam.ConnectionType = connection.Type
				connParam.ForwardConnectionType = connection
				if connection.Relation == "has_one" {

					identifier := SyntheticSystemRelationFieldIdentifier(connection.Model, knownAs)

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
				// find reverse (must match KnownAs — dual has_one to same model)
				var backwardModelType *models.ModelType
				for _, ct := range project.Schema.Models {
					if ct.Name == relationTo || ModelIDMatchesGraphQLField(ct.Name, relationTo) {
						backwardModelType = ct
						break
					}
				}
				if backwardModelType != nil {
					for _, _connection := range backwardModelType.Connections {
						if _connection == nil {
							continue
						}
						peerMatches := _connection.Model == modelType.Name ||
							ModelIDMatchesGraphQLField(_connection.Model, modelType.Name)
						if peerMatches && _connection.Type == "backward" && _connection.KnownAs == knownAs {
							connParam.BackwardConnectionModelType = backwardModelType
							connParam.BackwardConnectionType = _connection
							if _connection.Relation == "has_one" {

								identifier := SyntheticSystemRelationFieldIdentifier(_connection.Model, knownAs)

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
								if ct.Name == _connection.Model || ModelIDMatchesGraphQLField(ct.Name, _connection.Model) {
									connParam.ForwardConnectionModelType = ct
									break
								}
							}
							break
						}
					}
				}
			}
		}

		connParams = append(connParams, &connParam)
	}

	return connParams, nil
}
