package resolver

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
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

	var messageText string
	if val, ok := p.Args["message"].(string); ok {
		messageText = val
	}

	data := &models.SubscriptionEvent{
		ProjectID: param.ProjectID,
		UserID:    param.UserID,
		Message:   messageText,
		Type:      "info",
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Create Watermill message
	msg := message.NewMessage(param.UserID, payload)
	err = s.PubSubService.Publish("system_notify_channel", msg)
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

	userID := param.UserID

	subs, err := s.GraphQLSubscription.subscribe(p.Context, userID)
	if err != nil {
		return nil, err
	}

	return subs.Data, nil
}
