package resolver

import (
	"encoding/json"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) SendEvent(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	var message string
	if val, ok := p.Args["message"].(string); ok {
		message = val
	}

	data := &models.SubscriptionEvent{
		ProjectID: param.ProjectID,
		UserID:    param.UserID,
		Message:   message,
		Type:      "info",
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	err = s.PubSubService.Publish(p.Context, "system_notify_channel", payload)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": "msg published to queue",
	}, nil
}

func (s *GraphQLServer) EventSubscription(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	userId := param.UserID

	subs, err := s.GraphQLSubscription.subscribe(p.Context, userId)
	if err != nil {
		return nil, err
	}

	return subs.Data, nil
}
